package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/overrides"
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

func (api *v1) overrideStore(ctx context.Context) (*overrides.Store, error) {
	return overrides.Open(ctx, config.GetRunningConfig().Database.File)
}

func replaceOverrideValues(ctx context.Context, store *overrides.Store, kind overrides.Kind, values []string) error {
	if err := store.DeleteByKind(ctx, kind); err != nil {
		return err
	}
	for _, each := range values {
		if err := store.Upsert(ctx, kind, overrides.OverrideUpsert, each, ""); err != nil {
			return err
		}
	}
	return nil
}

func replaceOverrideHosts(ctx context.Context, store *overrides.Store, kind overrides.Kind, hosts map[string]string) error {
	if err := store.DeleteByKind(ctx, kind); err != nil {
		return err
	}
	for domain, address := range hosts {
		if err := store.Upsert(ctx, kind, overrides.OverrideUpsert, domain, address); err != nil {
			return err
		}
	}
	return nil
}

func copyStrings(values []string) []string {
	return append([]string(nil), values...)
}

func middlewareCloneHosts(hosts map[string]string) map[string]string {
	cloned := make(map[string]string, len(hosts))
	for domain, address := range hosts {
		cloned[domain] = address
	}
	return cloned
}

func (api *v1) StubToggle(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["stub-resolver"].(*middleware.StubResolver)
	state, err := actionToBool(r.PathValue("action"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
			Kind:          StubResolverResponseKind,
		}, w)
		return
	}

	api.server.StubsToggle(state)
	status := p.Status()
	res := Response{
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Kind:          StubResolverResponseKind,
		StubResolver:  &status,
	}
	writeJSON(res, w)

}

func (api *v1) StubStatus(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["stub-resolver"].(*middleware.StubResolver)
	status := p.Status()
	writeJSON(Response{
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Kind:          StubResolverResponseKind,
		StubResolver:  &status,
	}, w)
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
	status := r.URL.Query().Get("status")
	client := r.URL.Query().Get("client")
	clientMode := r.URL.Query().Get("client_mode")

	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}
	items, err := p.GetTop(top, since, status, client, clientMode)
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

func (api *v1) DNSLogClients(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["dns-log"].(*middleware.DNSLog)
	w.Header().Set("Content-Type", "application/json")

	search := r.URL.Query().Get("search")
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(Response{
				Kind:          DNSLogResponseKind,
				Message:       err.Error(),
				CurrentStatus: "Enabled",
			}, w)
			return
		}
		limit = parsed
	}

	items, err := p.SearchClients(search, limit)
	if err != nil {
		writeJSON(Response{
			Kind:          DNSLogResponseKind,
			Message:       err.Error(),
			CurrentStatus: "Enabled",
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          DNSLogResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: "Enabled",
		Clients:       items,
	}, w)
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
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	action := r.PathValue("action")
	state, err := actionToBool(action)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	st := api.server.BlacklistToggle(state)
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st),
		}, w)
		return
	}
	status.Enabled = st

	resp := Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(st),
		Blacklist:     &status,
	}
	writeJSON(resp, w)

}

func (api *v1) BlacklistStatus(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     &status,
	}, w)
}

func (api *v1) BlacklistAddRuntimeWhitelist(w http.ResponseWriter, r *http.Request) {
	req := &BlacklistWhitelistRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(api.server.Middlewares["blacklist"].(*middleware.BlackList).IsEnabled()),
		}, w)
		return
	}

	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	if err := p.AddRuntimeWhitelist(req.Domains); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     &status,
	}, w)
}

func (api *v1) BlacklistReplacePersistedHosts(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	req := &BlacklistHostsRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	hosts := overrides.NormalizeDomains(req.Hosts)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideBlacklistHost, hosts); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.Blacklist.ExtraHosts = copyStrings(hosts)
	config.SetRunningConfig(conf)
	if err := p.ReplacePersistedHosts(hosts); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     &status,
	}, w)
}

func (api *v1) BlacklistReplacePersistedExcludes(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["blacklist"].(*middleware.BlackList)
	req := &BlacklistExcludesRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	excludes := overrides.NormalizeSelectors(req.Excludes)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideBlacklistExclude, excludes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.Blacklist.PersistedExcludes = copyStrings(excludes)
	config.SetRunningConfig(conf)
	if err := p.ReplacePersistedExcludes(excludes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          BlacklistResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          BlacklistResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Blacklist:     &status,
	}, w)
}

func (api *v1) StaticResponseToggle(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       "static response middleware is not configured",
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}
	action := r.PathValue("action")
	state, err := actionToBool(action)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	p.SetEnabled(state)
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	resp := Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        &status,
	}
	writeJSON(resp, w)

}

func (api *v1) StaticResponseStatus(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       MESSAGE_OK,
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}
	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        &status,
	}, w)
}

func (api *v1) StaticResponseReplace(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       "static response middleware is not configured",
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}
	req := &StaticReplaceRequest{}
	err := json.NewDecoder(r.Body).Decode(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	hosts, err := middleware.ReadHostsLines(req.Hosts)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	if err := p.ReplaceRuntimeHosts(hosts); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        &status,
	}, w)
}

func (api *v1) StaticResponseReplacePersisted(w http.ResponseWriter, r *http.Request) {
	p, ok := api.server.Middlewares["static-response"].(*middleware.StaticResponse)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       "static response middleware is not configured",
			CurrentStatus: formatBool(false),
			Static:        staticResponseStatusFromConfig(),
		}, w)
		return
	}

	req := &StaticReplaceRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	hosts, err := middleware.ReadHostsLines(req.Hosts)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideHosts(r.Context(), store, overrides.OverrideStaticHost, hosts); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.StaticResponse.ExtraHosts = middlewareCloneHosts(hosts)
	config.SetRunningConfig(conf)
	p.ReplacePersistedHosts(hosts)

	status, err := p.Status()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          StaticResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.IsEnabled()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          StaticResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Static:        &status,
	}, w)
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
	status, err := st.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
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
		ZenMode:       &status,
	}, w)

}

func (api *v1) ZenPersistedDomainsReplace(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"].(*middleware.ZenMode)
	req := &ZenReplaceRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	domains := overrides.NormalizeDomains(req.ZenDomains)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideZenDomain, domains); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.ZenMode.PersistedDomains = copyStrings(domains)
	config.SetRunningConfig(conf)
	p.ReplacePersistedDomains(domains)

	status, err := p.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		ZenMode:       &status,
	}, w)
}

func (api *v1) ZenPersistedExcludesReplace(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"].(*middleware.ZenMode)
	req := &ZenExcludesRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	excludes := overrides.NormalizeSelectors(req.Excludes)
	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := replaceOverrideValues(r.Context(), store, overrides.OverrideZenExclude, excludes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	conf := config.GetRunningConfig()
	conf.ZenMode.PersistedExcludes = copyStrings(excludes)
	config.SetRunningConfig(conf)
	p.ReplacePersistedExcludes(excludes)

	status, err := p.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		ZenMode:       &status,
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
	st := p.(*middleware.StubResolver)
	c := config.GetRunningConfig()
	err = st.ReplaceRuntimeEntries(stubRequest.Stubs, c.Timeout, c.UpstreamTimeout)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          StubResolverResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(st.IsEnabled()),
		}, w)
		return
	}

	status := st.Status()
	logger.Infof("Loaded: %d stubs", len(status.RuntimeStubs))
	resp := Response{
		Message:       MESSAGE_OK,
		Kind:          StubResolverResponseKind,
		CurrentStatus: formatBool(status.Enabled),
		Items:         stubRequest.Stubs,
		StubResolver:  &status,
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
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       "Status Fail",
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         &status,
	}, w)
}

func (api *v1) CacheStatus(w http.ResponseWriter, r *http.Request) {
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         &status,
	}, w)
}

func (api *v1) CacheToggle(w http.ResponseWriter, r *http.Request) {
	state, err := actionToBool(r.PathValue("action"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}

	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := store.Upsert(r.Context(), overrides.OverrideCacheEnabled, overrides.OverrideSet, "enabled", strconv.FormatBool(state)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}

	api.server.CacheToggle(state)
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         &status,
	}, w)
}

func (api *v1) CacheReplaceExcludes(w http.ResponseWriter, r *http.Request) {
	req := &CacheExcludeRequest{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}

	normalized := overrides.NormalizeCacheSelectors(req.Excludes)

	store, err := api.overrideStore(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	defer func() { _ = store.Close() }()

	if err := store.DeleteByKind(r.Context(), overrides.OverrideCacheExclude); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          CacheResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
		}, w)
		return
	}
	for _, each := range normalized {
		if err := store.Upsert(r.Context(), overrides.OverrideCacheExclude, overrides.OverrideUpsert, each, ""); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			writeJSON(Response{
				Kind:          CacheResponseKind,
				Message:       err.Error(),
				CurrentStatus: formatBool(middleware.GetCache().IsEnabled()),
			}, w)
			return
		}
	}

	api.server.CacheReplaceExcludes(normalized)
	status := middleware.GetCache().StatusView()
	writeJSON(Response{
		Kind:          CacheResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Cache:         &status,
	}, w)
}

func (api *v1) ZenModeStart(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"]
	z := p.(*middleware.ZenMode)
	z.Start()
	status, err := z.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(z.Status()),
		}, w)
		return
	}
	res := Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		Items:         z.GetDomains(),
		ZenMode:       &status,
	}
	writeJSON(res, w)
}

func (api *v1) ZenModeStatus(w http.ResponseWriter, r *http.Request) {
	p := api.server.Middlewares["zen-mode"].(*middleware.ZenMode)
	status, err := p.StatusView()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(Response{
			Kind:          ZenModeResponseKind,
			Message:       err.Error(),
			CurrentStatus: formatBool(p.Status()),
		}, w)
		return
	}

	writeJSON(Response{
		Kind:          ZenModeResponseKind,
		Message:       MESSAGE_OK,
		CurrentStatus: formatBool(status.Enabled),
		ZenMode:       &status,
	}, w)
}

func staticResponseStatusFromConfig() *middleware.StaticResponseStatus {
	conf := config.GetRunningConfig()
	status := &middleware.StaticResponseStatus{
		Enabled:         conf.StaticResponse.Enabled,
		File:            conf.StaticResponse.File,
		Labels:          append([]string(nil), conf.StaticResponse.Labels...),
		ConfiguredHosts: []middleware.HostEntry{},
		PersistedHosts:  middleware.HostsToEntries(conf.StaticResponse.ExtraHosts),
		RuntimeHosts:    []middleware.HostEntry{},
	}
	if conf.StaticResponse.File == "" {
		return status
	}

	hosts, err := middleware.ReadHosts(conf.StaticResponse.File)
	if err == nil {
		status.ConfiguredHosts = middleware.HostsToEntries(hosts)
	}
	return status
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
	mux.HandleFunc("GET /api/stub-resolver", Require(api.StubStatus, protected))
	mux.HandleFunc("POST /api/zen-mode/persisted/domains", Require(api.ZenPersistedDomainsReplace, protected))
	mux.HandleFunc("POST /api/zen-mode/persisted/excludes", Require(api.ZenPersistedExcludesReplace, protected))
	mux.HandleFunc("POST /api/zen-mode", Require(api.ZenDomainsReplace, protected))
	mux.HandleFunc("GET /api/zen-mode", Require(api.ZenModeStatus, protected))
	mux.HandleFunc("POST /api/stub-resolver/{action}", Require(api.StubToggle, protected))
	mux.HandleFunc("GET /api/cache", Require(api.CacheStatus, protected))
	mux.HandleFunc("POST /api/cache/excludes", Require(api.CacheReplaceExcludes, protected))
	mux.HandleFunc("POST /api/cache/{action}", Require(api.CacheToggle, protected))
	mux.HandleFunc("GET /api/blacklist", Require(api.BlacklistStatus, protected))
	mux.HandleFunc("POST /api/blacklist/persisted/hosts", Require(api.BlacklistReplacePersistedHosts, protected))
	mux.HandleFunc("POST /api/blacklist/persisted/excludes", Require(api.BlacklistReplacePersistedExcludes, protected))
	mux.HandleFunc("POST /api/blacklist/{action}", Require(api.BlacklistToggle, protected))
	mux.HandleFunc("POST /api/blacklist/whitelist", Require(api.BlacklistAddRuntimeWhitelist, protected))
	mux.HandleFunc("GET /api/static-response", Require(api.StaticResponseStatus, protected))
	mux.HandleFunc("POST /api/static-response/persisted", Require(api.StaticResponseReplacePersisted, protected))
	mux.HandleFunc("POST /api/static-response", Require(api.StaticResponseReplace, protected))
	mux.HandleFunc("POST /api/static-response/{action}", Require(api.StaticResponseToggle, protected))
	mux.HandleFunc("GET /api/dns-log/dashboard", Require(api.DNSLogDashboard, protected))
	mux.HandleFunc("GET /api/dns-log/clients", Require(api.DNSLogClients, protected))
	mux.HandleFunc("GET /api/dns-log/top/{top}", Require(api.DNSLogTop, protected))
	mux.HandleFunc("GET /api/dns-log/rotate", Require(api.DNSLogRotate, protected))
	mux.HandleFunc("POST /api/dns-log/alias", Require(api.DNSLogAlias, protected))
	mux.HandleFunc("POST /api/zen-mode/start", Require(api.ZenModeStart, protected))
	mux.HandleFunc("DELETE /api/cache", Require(api.DeleteCache, protected))

	mux.HandleFunc("POST /api/tagger/tags", Require(api.TaggerAddTag, protected))
	mux.HandleFunc("GET /api/tagger/tags", Require(api.TaggerGetTags, protected))
	mux.HandleFunc("DELETE /api/tagger/tags/{tagName}", Require(api.TaggerDeleteTag, protected))

	mux.HandleFunc("GET /api/tagger/hosts", Require(api.TaggerKnownHosts, protected))
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
