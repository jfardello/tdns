package api

import (
	"net/http"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/server"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
)

type v1 struct {
	server *server.Server
}

// Metrics.
//
//	@Summary		Metrics
//	@Description	Return Prometheus metrics.
//	@Tags			monitoring
//	@ID				metricsGet
//	@Success		200	{string}	string	"Prometheus metrics"
//	@Router			/metrics [get]
func (api *v1) Metrics(w http.ResponseWriter, r *http.Request) {
	promhttp.Handler().ServeHTTP(w, r)
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
	mux.HandleFunc("PUT /api/tagger/addr/{tagName}", Require(api.TaggerLegacyAddressReplace, protected))

	mux.HandleFunc("GET /metrics", api.Metrics)
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
