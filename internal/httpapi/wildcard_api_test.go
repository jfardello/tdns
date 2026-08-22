package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	contractapi "github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/server"
)

func TestWildcardRoutesAuthorizePersistAndRestoreState(t *testing.T) {
	dbPath, err := db.Bootstrap(context.Background(), filepath.Join(t.TempDir(), "tdns.sqlite"))
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	conf := &config.Config{
		Database: config.DatabaseConf{File: dbPath},
		Wildcard: config.WildcardConf{
			PrimaryDomain:         middleware.DefaultWildcardDomain,
			AvailableExtraDomains: []string{"nip.io", "xip.io"},
			EnabledExtraDomains:   []string{"xip.io"},
		},
	}
	config.SetRunningConfig(conf)
	wildcard := &middleware.Wildcard{}
	if err := wildcard.Config(*conf); err != nil {
		t.Fatalf("Wildcard Config: %v", err)
	}
	dnsServer := &server.Server{Middlewares: map[string]middleware.Middleware{"wildcard": wildcard}}
	manager := testAuthManager(t)
	handler, err := NewHandler(dnsServer, manager, nil)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	readToken := issueTestToken(t, manager, auth.ScopeRead)
	writeToken := issueTestToken(t, manager, auth.ScopeWrite)

	request := func(method, path, token, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if body != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	if response := request(http.MethodGet, "/api/wildcard", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status code = %d, want 401", response.Code)
	}
	if response := request(http.MethodGet, "/api/wildcard", readToken, ""); response.Code != http.StatusOK {
		t.Fatalf("read status code = %d, want 200: %s", response.Code, response.Body.String())
	}
	if response := request(http.MethodPost, "/api/wildcard/start", readToken, ""); response.Code != http.StatusForbidden {
		t.Fatalf("read-only start code = %d, want 403", response.Code)
	}
	if response := request(http.MethodPut, "/api/wildcard/domains", writeToken, `{"domains":["example.com"]}`); response.Code != http.StatusBadRequest {
		t.Fatalf("unauthorized domain code = %d, want 400: %s", response.Code, response.Body.String())
	}
	if got := wildcard.Status().EnabledExtraDomains; !slices.Equal(got, []string{"xip.io"}) {
		t.Fatalf("rejected replacement changed domains to %#v", got)
	}

	if response := request(http.MethodPut, "/api/wildcard/domains", writeToken, `{"domains":["NIP.IO.","nip.io"]}`); response.Code != http.StatusOK {
		t.Fatalf("domain replacement code = %d, want 200: %s", response.Code, response.Body.String())
	} else {
		var body contractapi.Response
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode replacement response: %v", err)
		}
		if body.Wildcard == nil || !slices.Equal(body.Wildcard.EnabledExtraDomains, []string{"nip.io"}) {
			t.Fatalf("unexpected wildcard response: %#v", body.Wildcard)
		}
	}
	if response := request(http.MethodPost, "/api/wildcard/start", writeToken, ""); response.Code != http.StatusOK {
		t.Fatalf("start code = %d, want 200: %s", response.Code, response.Body.String())
	}

	store, err := overrides.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open overrides: %v", err)
	}
	defer store.Close()
	enabledRow, err := store.GetValue(context.Background(), overrides.OverrideWildcardEnabled, "enabled")
	if err != nil || enabledRow == nil || enabledRow.Value != "true" {
		t.Fatalf("enabled override = %#v, %v", enabledRow, err)
	}
	domainsRow, err := store.GetValue(context.Background(), overrides.OverrideWildcardDomains, "enabled")
	if err != nil || domainsRow == nil || domainsRow.Value != `["nip.io"]` {
		t.Fatalf("domains override = %#v, %v", domainsRow, err)
	}

	restarted := &config.Config{Wildcard: config.WildcardConf{
		AvailableExtraDomains: []string{"nip.io", "xip.io"},
		EnabledExtraDomains:   []string{"xip.io"},
	}}
	rows, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List overrides: %v", err)
	}
	if err := overrides.Apply(restarted, rows); err != nil {
		t.Fatalf("Apply overrides: %v", err)
	}
	restartedWildcard := &middleware.Wildcard{}
	if err := restartedWildcard.Config(*restarted); err != nil {
		t.Fatalf("configure restarted wildcard: %v", err)
	}
	status := restartedWildcard.Status()
	if !status.Enabled || !slices.Equal(status.EnabledExtraDomains, []string{"nip.io"}) {
		t.Fatalf("restarted status = %#v", status)
	}

	if response := request(http.MethodPut, "/api/wildcard/domains", writeToken, `{"domains":[]}`); response.Code != http.StatusOK {
		t.Fatalf("empty replacement code = %d, want 200: %s", response.Code, response.Body.String())
	}
	emptyRow, err := store.GetValue(context.Background(), overrides.OverrideWildcardDomains, "enabled")
	if err != nil || emptyRow == nil || emptyRow.Value != `[]` {
		t.Fatalf("empty domains override = %#v, %v", emptyRow, err)
	}
	emptyRestart := &config.Config{Wildcard: config.WildcardConf{
		AvailableExtraDomains: []string{"nip.io", "xip.io"},
		EnabledExtraDomains:   []string{"xip.io"},
	}}
	rows, err = store.List(context.Background())
	if err != nil {
		t.Fatalf("List overrides after empty replacement: %v", err)
	}
	if err := overrides.Apply(emptyRestart, rows); err != nil {
		t.Fatalf("Apply empty replacement: %v", err)
	}
	if len(emptyRestart.Wildcard.EnabledExtraDomains) != 0 {
		t.Fatalf("empty replacement restored YAML domains: %#v", emptyRestart.Wildcard.EnabledExtraDomains)
	}
}

func TestWildcardDomainMutationRequiresBrowserCSRF(t *testing.T) {
	dbPath, err := db.Bootstrap(context.Background(), filepath.Join(t.TempDir(), "tdns.sqlite"))
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	conf := &config.Config{
		Database: config.DatabaseConf{File: dbPath},
		Wildcard: config.WildcardConf{AvailableExtraDomains: []string{"nip.io"}},
	}
	config.SetRunningConfig(conf)
	wildcard := &middleware.Wildcard{}
	if err := wildcard.Config(*conf); err != nil {
		t.Fatalf("Wildcard Config: %v", err)
	}
	dnsServer := &server.Server{Middlewares: map[string]middleware.Middleware{"wildcard": wildcard}}
	manager := testAuthManager(t)
	browserStore := testBrowserStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	credentials, err := browserStore.RedeemCode(context.Background(), auth.Principal{
		Subject: "browser", Scope: auth.ScopeWrite, TokenID: "wildcard-code",
		Purpose: auth.BrowserCodePurpose, ExpiresAt: now.Add(time.Minute),
	}, now)
	if err != nil {
		t.Fatalf("RedeemCode: %v", err)
	}
	handler, err := NewHandler(dnsServer, manager, browserStore)
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}

	request := func(csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "https://tdns.example/api/wildcard/domains", strings.NewReader(`{"domains":["nip.io"]}`))
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: credentials.SessionID})
		sameOrigin(req)
		if csrf != "" {
			req.Header.Set(csrfHeaderName, csrf)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}
	if response := request(""); response.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d, want 403", response.Code)
	}
	if response := request(credentials.CSRFToken); response.Code != http.StatusOK {
		t.Fatalf("valid CSRF status = %d, want 200: %s", response.Code, response.Body.String())
	}
}
