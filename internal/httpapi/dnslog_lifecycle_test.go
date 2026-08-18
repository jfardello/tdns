package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	contractapi "github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/server"
)

func TestDNSLogLifecycleRoutesAuthorizeAndPersistState(t *testing.T) {
	dbPath, err := db.Bootstrap(context.Background(), filepath.Join(t.TempDir(), "tdns.sqlite"))
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	const keyEnvironment = "TDNS_TEST_HTTPAPI_PRIVACY_KEY"
	t.Setenv(keyEnvironment, base64.StdEncoding.EncodeToString(make([]byte, 32)))
	conf := &config.Config{
		Database: config.DatabaseConf{File: dbPath},
		DNSLog: config.DNSLogConf{
			Enabled: true,
			Pseudonymization: config.DNSLogPseudonymizationConf{
				KeyEnvironment: keyEnvironment,
			},
		},
		Auth: config.AuthConf{Browser: config.BrowserAuthConf{
			RememberDays: config.DefaultBrowserRememberDays,
		}},
	}
	config.SetRunningConfig(conf)
	dnsLog := &middleware.DNSLog{}
	if err := dnsLog.Config(*conf); err != nil {
		t.Fatalf("DNSLog Config: %v", err)
	}
	t.Cleanup(dnsLog.Stop)
	dnsServer := &server.Server{Middlewares: map[string]middleware.Middleware{"dns-log": dnsLog}}
	manager := testAuthManager(t)
	handler, err := NewHandler(dnsServer, manager, nil)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	readToken := issueTestToken(t, manager, auth.ScopeRead)
	writeToken := issueTestToken(t, manager, auth.ScopeWrite)

	request := func(method, path, token string, body ...string) *httptest.ResponseRecorder {
		t.Helper()
		var requestBody string
		if len(body) > 0 {
			requestBody = body[0]
		}
		req := httptest.NewRequest(method, path, strings.NewReader(requestBody))
		req.Header.Set("Authorization", "Bearer "+token)
		if requestBody != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	if response := request(http.MethodGet, "/api/dns-log", readToken); response.Code != http.StatusOK {
		t.Fatalf("read status code = %d, want 200", response.Code)
	} else {
		var body contractapi.Response
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode status: %v", err)
		}
		if body.DNSLog == nil || !body.DNSLog.Enabled {
			t.Fatalf("unexpected DNS-log status: %#v", body.DNSLog)
		}
	}
	if response := request(http.MethodPost, "/api/dns-log/stop", readToken); response.Code != http.StatusForbidden {
		t.Fatalf("read-only stop code = %d, want 403", response.Code)
	}
	if response := request(http.MethodDelete, "/api/dns-log", readToken); response.Code != http.StatusForbidden {
		t.Fatalf("read-only clear code = %d, want 403", response.Code)
	}
	if response := request(http.MethodPut, "/api/dns-log/privacy", readToken, `{"domains_pseudonymized":false,"clients_pseudonymized":false}`); response.Code != http.StatusForbidden {
		t.Fatalf("read-only privacy update code = %d, want 403", response.Code)
	}
	if response := request(http.MethodPut, "/api/dns-log/privacy", writeToken, `{"domains_pseudonymized":false,"clients_pseudonymized":false}`); response.Code != http.StatusConflict {
		t.Fatalf("running privacy update code = %d, want 409: %s", response.Code, response.Body.String())
	}
	dnsLog.Append(middleware.LogEvent{Client: "192.0.2.1", Domain: "example.com."})
	if response := request(http.MethodPost, "/api/dns-log/stop", writeToken); response.Code != http.StatusOK {
		t.Fatalf("write stop code = %d, want 200: %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPut, "/api/dns-log/privacy", writeToken, `{"domains_pseudonymized":true}`); response.Code != http.StatusBadRequest {
		t.Fatalf("partial privacy update code = %d, want 400: %s", response.Code, response.Body.String())
	}

	store, err := overrides.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open overrides: %v", err)
	}
	defer store.Close()
	row, err := store.GetValue(context.Background(), overrides.OverrideDNSLogEnabled, "enabled")
	if err != nil {
		t.Fatalf("read enabled override: %v", err)
	}
	if row == nil || row.Value != "false" {
		t.Fatalf("persisted DNS-log override = %#v", row)
	}
	if response := request(http.MethodPut, "/api/dns-log/privacy", writeToken, `{"domains_pseudonymized":true,"clients_pseudonymized":true}`); response.Code != http.StatusOK {
		t.Fatalf("privacy update code = %d, want 200: %s", response.Code, response.Body.String())
	} else {
		var body contractapi.Response
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode privacy update: %v", err)
		}
		if body.DNSLog == nil || !body.DNSLog.RequiresClear || !body.DNSLog.DomainsPseudonymized || !body.DNSLog.ClientsPseudonymized {
			t.Fatalf("unexpected privacy update status: %#v", body.DNSLog)
		}
	}
	for kind := range map[overrides.Kind]bool{
		overrides.OverrideDNSLogDomainsPseudonymized: true,
		overrides.OverrideDNSLogClientsPseudonymized: true,
	} {
		row, err := store.GetValue(context.Background(), kind, "enabled")
		if err != nil || row == nil || row.Value != "true" {
			t.Fatalf("privacy override %d = %#v, %v; want true", kind, row, err)
		}
	}
	if response := request(http.MethodPost, "/api/dns-log/start", writeToken); response.Code != http.StatusConflict {
		t.Fatalf("start before privacy clear code = %d, want 409: %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodDelete, "/api/dns-log", writeToken); response.Code != http.StatusOK {
		t.Fatalf("write clear code = %d, want 200: %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, "/api/dns-log/start", writeToken); response.Code != http.StatusOK {
		t.Fatalf("write start code = %d, want 200: %s", response.Code, response.Body.String())
	}
	row, err = store.GetValue(context.Background(), overrides.OverrideDNSLogEnabled, "enabled")
	if err != nil {
		t.Fatalf("read restarted override: %v", err)
	}
	if row == nil || row.Value != "true" {
		t.Fatalf("persisted restarted DNS-log override = %#v", row)
	}
}

func TestDNSLogRoutesReturnServiceUnavailableWithoutMiddleware(t *testing.T) {
	config.SetRunningConfig(&config.Config{Auth: config.AuthConf{Browser: config.BrowserAuthConf{
		RememberDays: config.DefaultBrowserRememberDays,
	}}})
	dnsServer := &server.Server{Middlewares: make(map[string]middleware.Middleware)}
	manager := testAuthManager(t)
	handler, err := NewHandler(dnsServer, manager, nil)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	readToken := issueTestToken(t, manager, auth.ScopeRead)
	writeToken := issueTestToken(t, manager, auth.ScopeWrite)

	routes := []struct {
		method string
		path   string
		token  string
	}{
		{http.MethodGet, "/api/dns-log", readToken},
		{http.MethodGet, "/api/dns-log/top/10", readToken},
		{http.MethodGet, "/api/dns-log/clients", readToken},
		{http.MethodGet, "/api/dns-log/dashboard", readToken},
		{http.MethodGet, "/api/dns-log/dashboard/history", readToken},
		{http.MethodGet, "/api/dns-log/dashboard/current", readToken},
		{http.MethodPost, "/api/dns-log/start", writeToken},
		{http.MethodPut, "/api/dns-log/privacy", writeToken},
		{http.MethodDelete, "/api/dns-log", writeToken},
		{http.MethodPost, "/api/dns-log/rotate", writeToken},
		{http.MethodPost, "/api/dns-log/alias", writeToken},
	}

	for _, route := range routes {
		name := route.method + " " + route.path
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, nil)
			request.Header.Set("Authorization", "Bearer "+route.token)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("status = %d, want 503: %s", response.Code, response.Body.String())
			}
			if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
				t.Fatalf("Content-Type = %q, want application/json", contentType)
			}
			var body contractapi.Response
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if body.Message != dnsLogUnavailableMessage {
				t.Fatalf("message = %q, want %q", body.Message, dnsLogUnavailableMessage)
			}
		})
	}
}
