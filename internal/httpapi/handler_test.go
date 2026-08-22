package httpapi

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jfardello/tdns/config"
)

func TestSwaggerRoutesFollowConfiguration(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		config.SetRunningConfig(&config.Config{})
		handler, err := NewHandler(nil, testAuthManager(t), nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/swagger/index.html", "/swagger/doc.json", "/swagger/openapi.yaml"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusNotFound {
				t.Errorf("%s status = %d, want %d", path, response.Code, http.StatusNotFound)
			}
		}
	})

	t.Run("enabled", func(t *testing.T) {
		config.SetRunningConfig(&config.Config{Server: config.Server{SwaggerEnabled: true}})
		handler, err := NewHandler(nil, testAuthManager(t), nil)
		if err != nil {
			t.Fatal(err)
		}

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("UI status = %d, want %d", response.Code, http.StatusOK)
		}
		if body, _ := io.ReadAll(response.Result().Body); !strings.Contains(string(body), "SwaggerUIBundle") {
			t.Fatal("Swagger UI response does not contain the UI application")
		}

		for _, path := range []string{"/swagger/doc.json", "/swagger/openapi.yaml"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Errorf("%s status = %d, want %d", path, response.Code, http.StatusOK)
			}
			if path == "/swagger/doc.json" && !strings.Contains(response.Body.String(), "BearerAuth") {
				t.Error("generated Swagger document does not define bearer authentication")
			}
		}
	})
}

func TestNewHandlerRejectsInvalidRememberedSessionLifetime(t *testing.T) {
	config.SetRunningConfig(&config.Config{Auth: config.AuthConf{
		Browser: config.BrowserAuthConf{RememberDays: config.MaxBrowserRememberDays + 1},
	}})
	if _, err := NewHandler(nil, testAuthManager(t), nil); err == nil {
		t.Fatal("NewHandler accepted an invalid remembered-session lifetime")
	}
}

func TestNewHandlerRegistersAPIRoutes(t *testing.T) {
	config.SetRunningConfig(&config.Config{})
	handler, err := NewHandler(nil, testAuthManager(t), nil)
	if err != nil {
		t.Fatal(err)
	}

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
		{http.MethodGet, "/api/wildcard"},
		{http.MethodPost, "/api/wildcard/start"},
		{http.MethodPut, "/api/wildcard/domains"},
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
		{http.MethodGet, "/api/dns-log/dashboard/history"},
		{http.MethodGet, "/api/dns-log/dashboard/current"},
		{http.MethodGet, "/api/dns-log/dashboard"},
		{http.MethodGet, "/api/dns-log/clients"},
		{http.MethodGet, "/api/dns-log/top/10"},
		{http.MethodPost, "/api/dns-log/rotate"},
		{http.MethodPost, "/api/dns-log/alias"},
		{http.MethodGet, "/api/dns-log"},
		{http.MethodPut, "/api/dns-log/privacy"},
		{http.MethodPost, "/api/dns-log/start"},
		{http.MethodDelete, "/api/dns-log"},
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
