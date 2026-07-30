package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jfardello/tdns/config"
)

func TestBrowserOriginValidation(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		wantErr bool
	}{
		{name: "same origin fetch metadata", headers: map[string]string{"Sec-Fetch-Site": "same-origin"}},
		{name: "same site rejected", headers: map[string]string{"Sec-Fetch-Site": "same-site"}, wantErr: true},
		{name: "cross site rejected", headers: map[string]string{"Sec-Fetch-Site": "cross-site"}, wantErr: true},
		{name: "exact origin fallback", headers: map[string]string{"Origin": "https://tdns.example"}},
		{name: "http origin rejected", headers: map[string]string{"Origin": "http://tdns.example"}, wantErr: true},
		{name: "null origin rejected", headers: map[string]string{"Origin": "null"}, wantErr: true},
		{name: "exact referer fallback", headers: map[string]string{"Referer": "https://tdns.example/dashboard"}},
		{name: "other referer rejected", headers: map[string]string{"Referer": "https://other.example/"}, wantErr: true},
		{name: "missing evidence rejected", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/test", nil)
			for key, value := range test.headers {
				request.Header.Set(key, value)
			}
			err := validateBrowserOrigin(request)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr = %t", err, test.wantErr)
			}
		})
	}
}

func TestCORSConfigurationFailsClosed(t *testing.T) {
	base := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	for _, test := range []struct {
		name string
		conf config.CORSConf
	}{
		{name: "empty", conf: config.CORSConf{Enabled: true}},
		{name: "wildcard", conf: config.CORSConf{Enabled: true, AllowedOrigins: []string{"*"}}},
		{name: "wildcard host", conf: config.CORSConf{Enabled: true, AllowedOrigins: []string{"https://*.example.com"}}},
		{name: "path", conf: config.CORSConf{Enabled: true, AllowedOrigins: []string{"https://example.com/path"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := withCORS(base, test.conf); err == nil {
				t.Fatal("withCORS accepted unsafe configuration")
			}
		})
	}

	handler, err := withCORS(base, config.CORSConf{
		Enabled:        true,
		AllowedOrigins: []string{"https://admin.example"},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "https://tdns.example/api/cache", nil)
	request.Header.Set("Origin", "https://admin.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Access-Control-Allow-Origin") != "https://admin.example" {
		t.Fatalf("allow origin = %q", response.Header().Get("Access-Control-Allow-Origin"))
	}
}
