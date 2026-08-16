package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	conf := &config.Config{
		Database: config.DatabaseConf{File: dbPath},
		DNSLog:   config.DNSLogConf{Enabled: true},
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

	request := func(method, path, token string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
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
	if response := request(http.MethodPost, "/api/dns-log/stop", writeToken); response.Code != http.StatusOK {
		t.Fatalf("write stop code = %d, want 200: %s", response.Code, response.Body.String())
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
