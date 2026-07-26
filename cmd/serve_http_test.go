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
		handler, err := newHTTPHandler(nil)
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
		handler, err := newHTTPHandler(nil)
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
	handler, err := newHTTPHandler(nil)
	if err != nil {
		t.Fatalf("newHTTPHandler: %v", err)
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/swagger/doc.json", nil))
	if strings.Contains(response.Body.String(), "<html") {
		t.Fatal("disabled Swagger route fell through to the SPA")
	}
}
