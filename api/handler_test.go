package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jfardello/tdns/config"
)

func TestNewHandlerRegistersAPIRoutes(t *testing.T) {
	config.SetRunningConfig(&config.Config{})
	handler := NewHandler(nil)

	routes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/stub-resolver"},
		{http.MethodGet, "/api/stub-resolver"},
		{http.MethodPost, "/api/stub-resolver/start"},
		{http.MethodPost, "/api/zen-mode/persisted/domains"},
		{http.MethodPost, "/api/zen-mode/persisted/excludes"},
		{http.MethodPost, "/api/zen-mode"},
		{http.MethodGet, "/api/zen-mode"},
		{http.MethodPost, "/api/zen-mode/start"},
		{http.MethodGet, "/api/cache"},
		{http.MethodPost, "/api/cache/excludes"},
		{http.MethodPost, "/api/cache/start"},
		{http.MethodDelete, "/api/cache"},
		{http.MethodGet, "/api/blacklist"},
		{http.MethodPost, "/api/blacklist/persisted/hosts"},
		{http.MethodPost, "/api/blacklist/persisted/excludes"},
		{http.MethodPost, "/api/blacklist/start"},
		{http.MethodPost, "/api/blacklist/whitelist"},
		{http.MethodGet, "/api/static-response"},
		{http.MethodPost, "/api/static-response/persisted"},
		{http.MethodPost, "/api/static-response"},
		{http.MethodPost, "/api/static-response/start"},
		{http.MethodGet, "/api/dns-log/dashboard"},
		{http.MethodGet, "/api/dns-log/clients"},
		{http.MethodGet, "/api/dns-log/top/10"},
		{http.MethodGet, "/api/dns-log/rotate"},
		{http.MethodPost, "/api/dns-log/alias"},
		{http.MethodPost, "/api/tagger/tags"},
		{http.MethodGet, "/api/tagger/tags"},
		{http.MethodDelete, "/api/tagger/tags/example"},
		{http.MethodGet, "/api/tagger/hosts"},
		{http.MethodGet, "/api/tagger/tags/example"},
		{http.MethodPost, "/api/tagger/tags/example"},
		{http.MethodDelete, "/api/tagger/tags/example/192.0.2.1"},
		{http.MethodPost, "/api/tagger/address"},
		{http.MethodPut, "/api/tagger/address/192.0.2.1"},
		{http.MethodPut, "/api/tagger/addr/example"},
	}

	for _, route := range routes {
		name := route.method + " " + route.path
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
			}
		})
	}
}
