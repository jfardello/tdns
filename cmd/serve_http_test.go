package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jfardello/tdns/config"
)

func TestHTTPHandlerRoutesSwaggerThroughAPIHandler(t *testing.T) {
	t.Run("disabled", func(t *testing.T) {
		config.SetRunningConfig(&config.Config{})
		handler, err := newHTTPHandler(nil, nil, nil)
		if err != nil {
			t.Fatalf("newHTTPHandler: %v", err)
		}

		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil))
		if response.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
		}
	})

	t.Run("enabled", func(t *testing.T) {
		config.SetRunningConfig(&config.Config{Server: config.Server{SwaggerEnabled: true}})
		handler, err := newHTTPHandler(nil, nil, nil)
		if err != nil {
			t.Fatalf("newHTTPHandler: %v", err)
		}

		for _, path := range []string{"/swagger/index.html", "/swagger/doc.json", "/swagger/openapi.yaml"} {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			if response.Code != http.StatusOK {
				t.Errorf("%s status = %d, want %d", path, response.Code, http.StatusOK)
			}
			if response.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("%s is missing security headers", path)
			}
		}
	})
}

func TestHTTPHandlerDoesNotServeSwaggerFromSPA(t *testing.T) {
	config.SetRunningConfig(&config.Config{})
	handler, err := newHTTPHandler(nil, nil, nil)
	if err != nil {
		t.Fatalf("newHTTPHandler: %v", err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if strings.Contains(response.Body.String(), "<html") {
		t.Fatal("disabled Swagger route fell through to the SPA")
	}
}

func TestHTTPHandlerDoesNotServeDiagnosticsFromManagement(t *testing.T) {
	config.SetRunningConfig(&config.Config{})
	handler, err := newHTTPHandler(nil, nil, nil)
	if err != nil {
		t.Fatalf("newHTTPHandler: %v", err)
	}
	for _, path := range []string{"/metrics", "/debug/pprof/"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusNotFound {
			t.Errorf("%s status = %d, want %d", path, response.Code, http.StatusNotFound)
		}
		if strings.Contains(response.Body.String(), "<html") {
			t.Errorf("%s fell through to the SPA", path)
		}
	}
}

func TestHTTPHandlerAppliesRestrictiveCSPToWebUI(t *testing.T) {
	config.SetRunningConfig(&config.Config{})
	handler, err := newHTTPHandler(nil, nil, nil)
	if err != nil {
		t.Fatalf("newHTTPHandler: %v", err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	policy := response.Header().Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("web UI response is missing Content-Security-Policy")
	}
	for _, prohibited := range []string{"'unsafe-eval'", "script-src 'self' 'unsafe-inline'"} {
		if strings.Contains(policy, prohibited) {
			t.Fatalf("web UI policy contains prohibited source %q: %s", prohibited, policy)
		}
	}
	if !strings.Contains(policy, "default-src 'none'") || !strings.Contains(policy, "'sha256-") {
		t.Fatalf("web UI policy is missing restrictive defaults or generated script hashes: %s", policy)
	}
}
