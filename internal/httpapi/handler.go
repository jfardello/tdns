package httpapi

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	contractapi "github.com/jfardello/tdns/api"
	_ "github.com/jfardello/tdns/api/docs"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/server"
	"github.com/rs/cors"
	httpSwagger "github.com/swaggo/http-swagger/v2"
)

type v1 struct {
	server           *server.Server
	authManager      *auth.Manager
	browserStore     BrowserSessionStore
	exchangeLimiter  *exchangeLimiter
	passwordLimiter  *passwordLimiter
	rememberLifetime time.Duration
	dnsLogMutationMu sync.Mutex
}

func NewHandler(
	dns *server.Server,
	authManager *auth.Manager,
	browserStore BrowserSessionStore,
) (http.Handler, error) {
	conf := config.GetRunningConfig()
	rememberDays := conf.Auth.Browser.RememberDays
	if rememberDays == 0 {
		rememberDays = config.DefaultBrowserRememberDays
	}
	if rememberDays < config.MinBrowserRememberDays || rememberDays > config.MaxBrowserRememberDays {
		return nil, errors.New("auth.browser.remember_days must be between 1 and 30")
	}
	readOnly := Requirement{IsRequired: true, Scope: auth.ScopeRead}
	readWrite := Requirement{IsRequired: true, Scope: auth.ScopeWrite}
	api := v1{
		server:           dns,
		authManager:      authManager,
		browserStore:     browserStore,
		exchangeLimiter:  newExchangeLimiter(),
		passwordLimiter:  newPasswordLimiter(),
		rememberLifetime: time.Duration(rememberDays) * 24 * time.Hour,
	}
	mux := http.NewServeMux()

	registerRoute(mux, "POST /api/stub-resolver", api.StubReplace, readWrite, "stub_replace", authManager, browserStore)
	registerRoute(mux, "GET /api/stub-resolver", api.StubStatus, readOnly, "", authManager, browserStore)
	registerRoute(mux, "POST /api/zen-mode/persisted/domains", api.ZenPersistedDomainsReplace, readWrite, "zen_persisted_domains_replace", authManager, browserStore)
	registerRoute(mux, "POST /api/zen-mode/persisted/excludes", api.ZenPersistedExcludesReplace, readWrite, "zen_persisted_excludes_replace", authManager, browserStore)
	registerRoute(mux, "POST /api/zen-mode", api.ZenDomainsReplace, readWrite, "zen_domains_replace", authManager, browserStore)
	registerRoute(mux, "GET /api/zen-mode", api.ZenModeStatus, readOnly, "", authManager, browserStore)
	registerRoute(mux, "POST /api/stub-resolver/{action}", api.StubToggle, readWrite, "stub_toggle", authManager, browserStore)
	registerRoute(mux, "GET /api/cache", api.CacheStatus, readOnly, "", authManager, browserStore)
	registerRoute(mux, "POST /api/cache/excludes", api.CacheReplaceExcludes, readWrite, "cache_excludes_replace", authManager, browserStore)
	registerRoute(mux, "POST /api/cache/{action}", api.CacheToggle, readWrite, "cache_toggle", authManager, browserStore)
	registerRoute(mux, "GET /api/blacklist", api.BlacklistStatus, readOnly, "", authManager, browserStore)
	registerRoute(mux, "POST /api/blacklist/persisted/hosts", api.BlacklistReplacePersistedHosts, readWrite, "blacklist_persisted_hosts_replace", authManager, browserStore)
	registerRoute(mux, "POST /api/blacklist/persisted/excludes", api.BlacklistReplacePersistedExcludes, readWrite, "blacklist_persisted_excludes_replace", authManager, browserStore)
	registerRoute(mux, "POST /api/blacklist/{action}", api.BlacklistToggle, readWrite, "blacklist_toggle", authManager, browserStore)
	registerRoute(mux, "POST /api/blacklist/whitelist", api.BlacklistAddRuntimeWhitelist, readWrite, "blacklist_whitelist_add", authManager, browserStore)
	registerRoute(mux, "GET /api/static-response", api.StaticResponseStatus, readOnly, "", authManager, browserStore)
	registerRoute(mux, "POST /api/static-response/persisted", api.StaticResponseReplacePersisted, readWrite, "static_response_persisted_replace", authManager, browserStore)
	registerRoute(mux, "POST /api/static-response", api.StaticResponseReplace, readWrite, "static_response_replace", authManager, browserStore)
	registerRoute(mux, "POST /api/static-response/{action}", api.StaticResponseToggle, readWrite, "static_response_toggle", authManager, browserStore)
	registerRoute(mux, "GET /api/dns-log/dashboard/history", api.DNSLogDashboardHistory, readOnly, "", authManager, browserStore)
	registerRoute(mux, "GET /api/dns-log/dashboard/current", api.DNSLogDashboardCurrent, readOnly, "", authManager, browserStore)
	registerRoute(mux, "GET /api/dns-log/dashboard", api.DNSLogDashboard, readOnly, "", authManager, browserStore)
	registerRoute(mux, "GET /api/dns-log/clients", api.DNSLogClients, readOnly, "", authManager, browserStore)
	registerRoute(mux, "GET /api/dns-log/top/{top}", api.DNSLogTop, readOnly, "", authManager, browserStore)
	registerRoute(mux, "GET /api/dns-log", api.DNSLogStatus, readOnly, "", authManager, browserStore)
	registerRoute(mux, "POST /api/dns-log/{action}", api.DNSLogToggle, readWrite, "dns_log_toggle", authManager, browserStore)
	registerRoute(mux, "DELETE /api/dns-log", api.DNSLogClear, readWrite, "dns_log_clear", authManager, browserStore)
	registerRoute(mux, "POST /api/dns-log/rotate", api.DNSLogRotate, readWrite, "dns_log_rotate", authManager, browserStore)
	registerRoute(mux, "POST /api/dns-log/alias", api.DNSLogAlias, readWrite, "dns_log_alias", authManager, browserStore)
	registerRoute(mux, "POST /api/zen-mode/start", api.ZenModeStart, readWrite, "zen_mode_start", authManager, browserStore)
	registerRoute(mux, "DELETE /api/cache", api.DeleteCache, readWrite, "cache_delete", authManager, browserStore)

	registerRoute(mux, "POST /api/tagger/tags", api.TaggerAddTag, readWrite, "tag_add", authManager, browserStore)
	registerRoute(mux, "GET /api/tagger/tags", api.TaggerGetTags, readOnly, "", authManager, browserStore)
	registerRoute(mux, "DELETE /api/tagger/tags/{tagName}", api.TaggerDeleteTag, readWrite, "tag_delete", authManager, browserStore)

	registerRoute(mux, "GET /api/tagger/hosts", api.TaggerKnownHosts, readOnly, "", authManager, browserStore)
	registerRoute(mux, "GET /api/tagger/tags/{tagName}", api.TaggerTagGetMembers, readOnly, "", authManager, browserStore)
	registerRoute(mux, "POST /api/tagger/tags/{tagName}", api.TaggerAddMember, readWrite, "tag_member_add", authManager, browserStore)
	registerRoute(mux, "DELETE /api/tagger/tags/{tagName}/{address}", api.TaggerDeleteTagMember, readWrite, "tag_member_delete", authManager, browserStore)
	registerRoute(mux, "POST /api/tagger/address", api.TaggerAddressCreate, readWrite, "tagger_address_create", authManager, browserStore)
	registerRoute(mux, "PUT /api/tagger/address/{address}", api.TaggerAddressReplace, readWrite, "tagger_address_replace", authManager, browserStore)
	registerRoute(mux, "PUT /api/tagger/addr/{tagName}", api.TaggerLegacyAddressReplace, readWrite, "tagger_legacy_address_replace", authManager, browserStore)

	mux.HandleFunc("POST /api/auth/exchange", api.BrowserCodeExchange)
	mux.HandleFunc("POST /api/auth/login", api.BrowserPasswordLogin)
	mux.HandleFunc("GET /api/auth/session", api.BrowserSession)
	mux.HandleFunc("POST /api/auth/logout", api.BrowserLogout)
	if conf.Server.SwaggerEnabled {
		mux.Handle("GET /swagger/", httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"),
			httpSwagger.DeepLinking(true),
		))
		mux.HandleFunc("GET /swagger/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/yaml")
			_, _ = w.Write(contractapi.OpenAPISpec())
		})
	}
	return withCORS(mux, conf.CORS)
}

func registerRoute(
	mux *http.ServeMux,
	pattern string,
	handler http.HandlerFunc,
	requirement Requirement,
	auditAction string,
	manager *auth.Manager,
	browserStore BrowserSessionStore,
) {
	if auditAction != "" {
		handler = AuditMutation(auditAction, handler)
	}
	mux.HandleFunc(pattern, Require(handler, requirement, manager, browserStore))
}

func withCORS(handler http.Handler, conf config.CORSConf) (http.Handler, error) {
	if !conf.Enabled {
		return handler, nil
	}
	if len(conf.AllowedOrigins) == 0 {
		return nil, errors.New("cors.allowed_origins must not be empty when CORS is enabled")
	}
	for _, origin := range conf.AllowedOrigins {
		if err := validateCORSOrigin(origin); err != nil {
			return nil, err
		}
	}

	options := cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"authorization", "content-type", csrfHeaderName},
		AllowedOrigins: conf.AllowedOrigins,
		Debug:          log.IsDebugEnabled(),
	}

	return cors.New(options).Handler(handler), nil
}

func validateCORSOrigin(origin string) error {
	if strings.TrimSpace(origin) != origin || origin == "*" {
		return errors.New("cors.allowed_origins must contain exact origins without wildcards")
	}
	parsed, err := url.Parse(origin)
	if err != nil ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") ||
		parsed.Host == "" ||
		parsed.User != nil ||
		parsed.RawQuery != "" ||
		parsed.Fragment != "" ||
		parsed.Path != "" {
		return errors.New("cors.allowed_origins contains an invalid origin")
	}
	if strings.Contains(parsed.Hostname(), "*") {
		return errors.New("cors.allowed_origins must contain exact origins without wildcards")
	}
	return nil
}
