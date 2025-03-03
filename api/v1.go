package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/plugin"
	"github.com/jfardello/tdns/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

const (
	MESSAGE_OK = "Status OK"
)

type Auth struct {
	IsRequired bool
	Scope      string
}

func Require(handler func(http.ResponseWriter, *http.Request), auth Auth) http.HandlerFunc {
	logger := log.GetLogger("api", "JWTAuth")
	return func(w http.ResponseWriter, r *http.Request) {
		// Unauthenticated access allowed
		if !auth.IsRequired || len(auth.Scope) == 0 {
			handler(w, r)
			return
		}
		//Get bearer
		bearer := r.Header.Get("authorization")
		splitted := strings.Split(bearer, " ")
		if len(splitted) != 2 {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		claims, err := Validate(splitted[1], auth.Scope)
		if err != nil {
			switch {
			case errors.Is(err, jwt.ErrTokenExpired):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="The access token expired"`)
			case errors.Is(err, jwt.ErrTokenSignatureInvalid):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="Invalid token"`)
			case errors.Is(err, jwt.ErrTokenInvalidClaims):
				w.Header().Add("WWW-Authenticate", `"Bearer realm="tdns", error_description="Invalid claims"`)
			}

			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		logger.WithFields(logrus.Fields{"sub": claims["sub"], "scope": claims["scope"]}).Debug("Granting access.")
		handler(w, r)

	}
}

type v1 struct {
	server *server.Server
}

func (api *v1) StubToggle(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	curr := "disabled"
	var state bool
	switch action {
	case "start":
		state = true
	case "stop":
		state = false
	}
	if api.server.StubsToggle(state) {
		curr = "enabled"
	}
	res := Response{Message: MESSAGE_OK, CurrentStatus: curr, Kind: STUB_RESPONSE_KIND}
	writeJSON(res, w)

}

func (api *v1) BholeToggle(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	var state bool

	switch action {
	case "start":
		state = true
	case "stop":
		state = false

	}

	st := api.server.BholeToggle(state)

	resp := Response{
		Kind:          BHOLE_RESPONSE_KIND,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(st),
	}
	writeJSON(resp, w)

}

func (api *v1) StaticResposeToogle(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger("api", "StaticResposeToogle")
	p := api.server.Plugins["staticresponse"].(*plugin.StaticResponsePlugin)
	action := r.PathValue("action")
	state, err := actionToBool(action)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BHOLE_RESPONSE_KIND,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Enabled),
		}, w)
		return
	}
	c := config.GetRunningConfig()
	c.Static.Enabled = state
	config.SetRunningConfig(c)
	err = p.Config(*c)
	if err != nil {
		logger.Fatal(err)
	}

	resp := Response{
		Kind:          BHOLE_RESPONSE_KIND,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(p.Enabled),
	}
	writeJSON(resp, w)

}

func actionToBool(action string) (bool, error) {
	switch action {
	case "start":
		return true, nil
	case "stop":
		return false, nil

	}
	return false, errors.New("Invalid parameter.")
}

func (api *v1) ZenDomainsReplace(w http.ResponseWriter, r *http.Request) {
	l := log.GetLogger("serve", "api-server")
	logger := l.WithFields(logrus.Fields{"Method": "ZenDomainsReplace"})
	zreq := &ZenReplaceRequest{}
	err := json.NewDecoder(r.Body).Decode(&zreq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	p := api.server.Plugins["zenmode"]
	st := p.(*plugin.ZenmodePlugin)
	logger.Debug("About to replace domain for zen mode.")
	h := map[string]string{}
	for _, each := range zreq.ZenDomains {
		h[each] = plugin.ZENMODE_IP
		logger.WithFields(logrus.Fields{"domain": each}).Debug("Adding domain to zen mode.")
	}
	err = st.ReplaceDomains(h)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          STUB_RESPONSE_KIND,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.Status()),
		}, w)
		return
	}
	writeJSON(Response{
		Kind:          ZEN_RESPONSE_KIND,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(st.Status()),
		Items:         st.GetDomains(),
	}, w)

}

func (api *v1) StubReplace(w http.ResponseWriter, r *http.Request) {
	l := log.GetLogger("serve", "api-server")
	logger := l.WithFields(logrus.Fields{"Method": "StubReplace"})
	stubRequest := &StubReplaceRequest{}
	p := api.server.Plugins["stubresolver"]
	err := json.NewDecoder(r.Body).Decode(&stubRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ups, err := plugin.ParseStubList(stubRequest.Stubs)
	if err != nil {
		st := p.(*plugin.StubresolverPlugin)
		logger.Error("Error parsing stubs: ", err)
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          STUB_RESPONSE_KIND,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.EnableStubs),
		}, w)
		return
	}
	logger.Info("Replacing stubs")
	st := p.(*plugin.StubresolverPlugin)
	c := config.GetRunningConfig()
	c.BlackHole.Excludes = stubRequest.Stubs
	config.SetRunningConfig(c)
	err = st.Config(*c)
	if err != nil {
		logger.Fatal(err)
	}
	err = st.Init()
	if err != nil {
		logger.Error("Error initiating stubs: ", err)
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          STUB_RESPONSE_KIND,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.EnableStubs),
		}, w)
		return
	}

	logger.Infof("Loaded: %d stubs", len(ups))
	resp := Response{
		Message:       MESSAGE_OK,
		Kind:          STUB_RESPONSE_KIND,
		CurrentStatus: strconv.FormatBool(st.EnableStubs),
		Items:         stubRequest.Stubs,
	}
	writeJSON(resp, w)

}

func (api *v1) DeleteCache(w http.ResponseWriter, r *http.Request) {
	l := log.GetLogger("serve", "api-server")
	logger := l.WithFields(logrus.Fields{"Method": "ClearCache"})
	w.Header().Set("Content-Type", "application/json")
	logger.Info("Clearing cache")
	err := api.server.ClearCache()
	if err != nil {
		logger.Error("Error clearing cache: ", err)
		http.Error(w, `{"message":"Status Fail"}`, http.StatusInternalServerError)
	}
	_, err = w.Write([]byte(`{"message":"Status OK"}`))
	if err != nil {
		logger.Error(err)
	}
}

func (api *v1) ZenModeStart(w http.ResponseWriter, r *http.Request) {
	curr := "disabled"
	p := api.server.Plugins["zenmode"]
	z := p.(*plugin.ZenmodePlugin)
	z.Start()
	st := z.Status()
	if st {
		curr = "enabled"
	}
	res := Response{
		Kind:          ZEN_RESPONSE_KIND,
		Message:       MESSAGE_OK,
		CurrentStatus: curr,
		Items:         z.GetDomains(),
	}
	writeJSON(res, w)
}

func writeJSON(res Response, w http.ResponseWriter) {
	logger := log.GetLogger("api", "writeJSON")
	encoded, err := json.Marshal(res)
	if err != nil {
		logger.Fatal(err)
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(encoded)
	if err != nil {
		logger.Fatal(err)
	}

}

func formatBool(status bool) string {
	if status {
		return "enabled"
	} else {
		return "disabled"
	}
}

func Serve(dns *server.Server) {
	logger := log.GetLogger("serve", "api-server")
	conf := config.GetRunningConfig()
	protected := Auth{IsRequired: true, Scope: RWSCOPE}
	api := v1{server: dns}

	http.HandleFunc("POST /api/stubs", Require(api.StubReplace, protected))
	http.HandleFunc("POST /api/zen", Require(api.ZenDomainsReplace, protected))
	http.HandleFunc("POST /api/stubs/{action}", Require(api.StubToggle, protected))
	http.HandleFunc("POST /api/bhole/{action}", Require(api.BholeToggle, protected))
	http.HandleFunc("POST /api/static/{action}", Require(api.StaticResposeToogle, protected))
	http.HandleFunc("POST /api/zen/start", Require(api.ZenModeStart, protected))
	http.HandleFunc("DELETE /api/cache", Require(api.DeleteCache, protected))
	http.Handle("GET /metrics", promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	logger.Infof("Starting api server at %s, (crt:%s, keyfile:%s)", conf.Server.APIAddr, conf.Server.APICertFile, conf.Server.APIKeyFile)
	logger.Fatal(http.ListenAndServeTLS(conf.Server.APIAddr, conf.Server.APICertFile, conf.Server.APIKeyFile, nil))

}
