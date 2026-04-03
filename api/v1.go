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
	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
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
	res := Response{Message: MESSAGE_OK, CurrentStatus: curr, Kind: StubResolverResponseKind}
	writeJSON(res, w)

}
func (api *v1) DNSLogAlias(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")
	jr := new(DNSLogAliasRequest)
	err := json.NewDecoder(r.Body).Decode(jr)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	err = p.AddAlias(jr.Name, jr.Addr)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}

	res := Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
	}
	writeJSON(res, w)
}
func (api *v1) DNSLogRotate(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")

	since := r.URL.Query().Get("since")

	err := p.Rotate(since)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	res := Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Rotate OK",
	}
	writeJSON(res, w)
}

func (api *v1) DNSLogTop(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")
	top, err := strconv.Atoi(r.PathValue("top"))
	since := r.URL.Query().Get("since")

	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	items, err := p.GetTop(top, since)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	res := Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
		LogItems:      items}
	writeJSON(res, w)
}

func (api *v1) DNSLogDashboard(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")

	hours := 24
	if raw := r.URL.Query().Get("hours"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(Response{
				Kind:          DNSLogResponseKind,
				Message:       err.Error(),
				CurrentStatus: "Enabled",
			}, w)
			return
		}
		hours = parsed
	}

	stats, err := p.GetDashboardStats(hours)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}

	cacheStats := middleware.GetCache().Status()
	stats.Summary.CacheHits = cacheStats.Hits
	stats.Summary.CacheMisses = cacheStats.Misses

	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
		WindowHours:   stats.WindowHours,
		Summary:       &stats.Summary,
		Hourly:        stats.Hourly,
	}, w)
}

func (api *v1) BlacklistToggle(w http.ResponseWriter, r *http.Request) {
	action := r.PathValue("action")
	var state bool

	switch action {
	case "start":
		state = true
	case "stop":
		state = false

	}

	st := api.server.BlacklistToggle(state)

	resp := Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(st),
	}
	writeJSON(resp, w)

}

func (api *v1) StaticResponseToggle(w http.ResponseWriter, r *http.Request) {
	logger := log.GetLogger("api", "StaticResponseToggle")
	p := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	action := r.PathValue("action")
	state, err := actionToBool(action)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Enabled),
		}, w)
		return
	}
	c := config.GetRunningConfig()
	c.StaticResponse.Enabled = state
	config.SetRunningConfig(c)
	err = p.Config(*c)
	if err != nil {
		logger.Fatal(err)
	}

	resp := Response{
		Kind:          StaticResponseKind,
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
	p := api.server.Middlewares["zen-mode"]
	st := p.(*middleware.ZenMode)
	logger.Debug("About to replace domain for zen mode.")
	h := map[string]string{}
	for _, each := range zreq.ZenDomains {
		h[each] = middleware.ZenModeIP
		logger.WithFields(logrus.Fields{"domain": each}).Debug("Adding domain to zen mode.")
	}
	err = st.ReplaceDomains(h)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.Status()),
		}, w)
		return
	}
	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(st.Status()),
		Items:         st.GetDomains(),
	}, w)

}

func (api *v1) StubReplace(w http.ResponseWriter, r *http.Request) {
	l := log.GetLogger("serve", "api-server")
	logger := l.WithFields(logrus.Fields{"Method": "StubReplace"})
	stubRequest := &StubReplaceRequest{}
	p := api.server.Middlewares["stub-resolver"]
	err := json.NewDecoder(r.Body).Decode(&stubRequest)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c := config.GetRunningConfig()
	ups, err := middleware.ParseStubList(stubRequest.Stubs, c.Timeout, c.UpstreamTimeout)
	if err != nil {
		st := p.(*middleware.StubResolver)
		logger.Error("Error parsing stubs: ", err)
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StubResolverResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.EnableStubs),
		}, w)
		return
	}
	logger.Info("Replacing stubs")
	st := p.(*middleware.StubResolver)
	c.StubResolver.Stubs = stubRequest.Stubs
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
			Kind:          StubResolverResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.EnableStubs),
		}, w)
		return
	}

	logger.Infof("Loaded: %d stubs", len(ups))
	resp := Response{
		Message:       MESSAGE_OK,
		Kind:          StubResolverResponseKind,
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
	p := api.server.Middlewares["zen-mode"]
	z := p.(*middleware.ZenMode)
	z.Start()
	st := z.Status()
	if st {
		curr = "enabled"
	}
	res := Response{
		Kind:          ZenModeResponseKind,
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

func NewHandler(dns *server.Server) http.Handler {
	conf := config.GetRunningConfig()
	protected := Auth{IsRequired: true, Scope: RWSCOPE}
	api := v1{server: dns}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/stub-resolver", Require(api.StubReplace, protected))
	mux.HandleFunc("POST /api/zen-mode", Require(api.ZenDomainsReplace, protected))
	mux.HandleFunc("POST /api/stub-resolver/{action}", Require(api.StubToggle, protected))
	mux.HandleFunc("POST /api/blacklist/{action}", Require(api.BlacklistToggle, protected))
	mux.HandleFunc("POST /api/static-response/{action}", Require(api.StaticResponseToggle, protected))
	mux.HandleFunc("GET /api/dns-log/dashboard", Require(api.DNSLogDashboard, protected))
	mux.HandleFunc("GET /api/dns-log/top/{top}", Require(api.DNSLogTop, protected))
	mux.HandleFunc("GET /api/dns-log/rotate", Require(api.DNSLogRotate, protected))
	mux.HandleFunc("POST /api/dns-log/alias", Require(api.DNSLogAlias, protected))
	mux.HandleFunc("POST /api/zen-mode/start", Require(api.ZenModeStart, protected))
	mux.HandleFunc("DELETE /api/cache", Require(api.DeleteCache, protected))

	mux.HandleFunc("POST /api/tagger/tags", Require(api.TaggerAddTag, protected))
	mux.HandleFunc("GET /api/tagger/tags", Require(api.TaggerGetTags, protected))
	mux.HandleFunc("DELETE /api/tagger/tags/{tagName}", Require(api.TaggerDeleteTag, protected))

	mux.HandleFunc("GET /api/tagger/tags/{tagName}", Require(api.TaggerTagGetMembers, protected))
	mux.HandleFunc("POST /api/tagger/tags/{tagName}", Require(api.TaggerAddMember, protected))
	mux.HandleFunc("DELETE /api/tagger/tags/{tagName}/{address}", Require(api.TaggerDeleteTagMember, protected))
	mux.HandleFunc("POST /api/tagger/address", Require(api.TaggerAddressCreate, protected))
	mux.HandleFunc("PUT /api/tagger/address/{address}", Require(api.TaggerAddressReplace, protected))
	mux.HandleFunc("PUT /api/tagger/addr/{tagName}", Require(api.TaggerAddressReplace, protected))

	mux.Handle("GET /metrics", promhttp.Handler())
	return withCORS(mux, conf.CORS)
}

func withCORS(handler http.Handler, conf config.CORSConf) http.Handler {
	if !conf.Enabled {
		return handler
	}

	options := cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"authorization", "content-type"},
		Debug:          log.IsDebugEnabled(),
	}
	if len(conf.AllowedOrigins) > 0 {
		options.AllowedOrigins = conf.AllowedOrigins
	} else {
		options.AllowOriginFunc = func(string) bool { return true }
	}

	return cors.New(options).Handler(handler)
}
