package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contractapi "github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/browserauth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/sirupsen/logrus"
)

func testBrowserStore(t *testing.T) *browserauth.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tdns.sqlite")
	if _, err := db.Bootstrap(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	store, err := browserauth.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store
}

func testAPIHandler(
	t *testing.T,
	manager *auth.Manager,
	store BrowserSessionStore,
) http.Handler {
	t.Helper()
	config.SetRunningConfig(&config.Config{})
	handler, err := NewHandler(nil, manager, store)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func sameOrigin(request *http.Request) {
	request.Header.Set("Sec-Fetch-Site", "same-origin")
}

func TestBrowserAuthenticationLifecycle(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	handler := testAPIHandler(t, manager, store)
	code, err := manager.IssueBrowserCode("browser-admin", auth.ScopeWrite, time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	body, _ := json.Marshal(contractapi.BrowserCodeExchangeRequest{Code: code})
	exchange := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/exchange", bytes.NewReader(body))
	exchange.Header.Set("Content-Type", "application/json")
	sameOrigin(exchange)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, exchange)
	if response.Code != http.StatusOK {
		t.Fatalf("exchange status = %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("exchange response is cacheable")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("exchange cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName ||
		cookie.Value == "" ||
		cookie.Path != "/" ||
		!cookie.Secure ||
		!cookie.HttpOnly ||
		cookie.SameSite != http.SameSiteStrictMode ||
		cookie.Domain != "" ||
		cookie.MaxAge != 0 ||
		!cookie.Expires.IsZero() {
		t.Fatalf("session cookie = %#v", cookie)
	}
	setCookie := response.Header().Get("Set-Cookie")
	for _, prohibited := range []string{"Domain=", "Expires=", "Max-Age="} {
		if strings.Contains(setCookie, prohibited) {
			t.Fatalf("session cookie contains %q: %s", prohibited, setCookie)
		}
	}
	var initial contractapi.BrowserSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &initial); err != nil {
		t.Fatal(err)
	}
	if initial.Subject != "browser-admin" || initial.Scope != auth.ScopeWrite || initial.CSRFToken == "" {
		t.Fatalf("exchange response = %#v", initial)
	}

	replay := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/exchange", bytes.NewReader(body))
	replay.Header.Set("Content-Type", "application/json")
	sameOrigin(replay)
	replayResponse := httptest.NewRecorder()
	handler.ServeHTTP(replayResponse, replay)
	if replayResponse.Code != http.StatusUnauthorized {
		t.Fatalf("replay status = %d, want 401", replayResponse.Code)
	}

	status := httptest.NewRequest(http.MethodGet, "https://tdns.example/api/auth/session", nil)
	status.AddCookie(cookie)
	statusResponse := httptest.NewRecorder()
	handler.ServeHTTP(statusResponse, status)
	if statusResponse.Code != http.StatusOK {
		t.Fatalf("session status = %d: %s", statusResponse.Code, statusResponse.Body.String())
	}
	var restored contractapi.BrowserSessionResponse
	if err := json.Unmarshal(statusResponse.Body.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.CSRFToken == "" || restored.CSRFToken == initial.CSRFToken {
		t.Fatal("session status did not issue a fresh CSRF token")
	}
	if err := store.ValidateCSRF(context.Background(), cookie.Value, initial.CSRFToken, time.Now()); err != nil {
		t.Fatalf("initial CSRF token invalidated by reload: %v", err)
	}

	logout := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/logout", nil)
	logout.AddCookie(cookie)
	logout.Header.Set(csrfHeaderName, restored.CSRFToken)
	sameOrigin(logout)
	logoutResponse := httptest.NewRecorder()
	handler.ServeHTTP(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}
	deleted := logoutResponse.Result().Cookies()
	if len(deleted) != 1 || deleted[0].Name != sessionCookieName || deleted[0].MaxAge >= 0 {
		t.Fatalf("logout cookie = %#v", deleted)
	}
	if _, err := store.GetSession(context.Background(), cookie.Value, time.Now()); !errors.Is(err, browserauth.ErrSessionNotFound) {
		t.Fatalf("logged-out session error = %v", err)
	}
}

func TestBrowserExchangeRequiresStrictJSONAndSameOrigin(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)

	tests := []struct {
		name        string
		contentType string
		body        string
		sameOrigin  bool
		want        int
	}{
		{name: "non json", contentType: "text/plain", body: `{}`, sameOrigin: true, want: http.StatusUnsupportedMediaType},
		{name: "unknown field", contentType: "application/json", body: `{"code":"x","extra":true}`, sameOrigin: true, want: http.StatusBadRequest},
		{name: "trailing json", contentType: "application/json", body: `{"code":"x"} {}`, sameOrigin: true, want: http.StatusBadRequest},
		{name: "empty code", contentType: "application/json", body: `{"code":""}`, sameOrigin: true, want: http.StatusBadRequest},
		{name: "cross site", contentType: "application/json", body: `{"code":"x"}`, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := testAPIHandler(t, manager, store)
			request := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/exchange", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if test.sameOrigin {
				sameOrigin(request)
			} else {
				request.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestCookieAndBearerAuthenticationRemainSeparated(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	credentials, err := store.RedeemCode(context.Background(), auth.Principal{
		Subject:   "browser",
		Scope:     auth.ScopeWrite,
		TokenID:   "middleware-code",
		Purpose:   auth.BrowserCodePurpose,
		ExpiresAt: now.Add(time.Minute),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	bearer, err := manager.IssueBearer("api", auth.ScopeWrite, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	handler := Require(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := IdentityFromContext(r.Context())
		if !ok {
			t.Fatal("request identity missing")
		}
		w.Header().Set("X-Auth-Transport", string(identity.Transport))
		w.WriteHeader(http.StatusNoContent)
	}, Requirement{IsRequired: true, Scope: auth.ScopeWrite}, manager, store)

	tests := []struct {
		name       string
		bearer     string
		cookie     bool
		csrf       string
		origin     bool
		method     string
		wantStatus int
		transport  AuthTransport
	}{
		{name: "bearer unsafe without csrf", bearer: bearer, method: http.MethodPost, wantStatus: http.StatusNoContent, transport: AuthTransportBearer},
		{name: "cookie safe", cookie: true, method: http.MethodGet, wantStatus: http.StatusNoContent, transport: AuthTransportSession},
		{name: "cookie unsafe", cookie: true, csrf: credentials.CSRFToken, origin: true, method: http.MethodPost, wantStatus: http.StatusNoContent, transport: AuthTransportSession},
		{name: "cookie missing csrf", cookie: true, origin: true, method: http.MethodPost, wantStatus: http.StatusForbidden},
		{name: "cookie missing origin", cookie: true, csrf: credentials.CSRFToken, method: http.MethodPost, wantStatus: http.StatusForbidden},
		{name: "ambiguous", bearer: bearer, cookie: true, method: http.MethodGet, wantStatus: http.StatusUnauthorized},
		{name: "invalid bearer does not fall back", bearer: "invalid", cookie: true, method: http.MethodGet, wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "https://tdns.example/api/test", nil)
			if test.bearer != "" {
				request.Header.Set("Authorization", "Bearer "+test.bearer)
			}
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: credentials.SessionID})
			}
			if test.csrf != "" {
				request.Header.Set(csrfHeaderName, test.csrf)
			}
			if test.origin {
				sameOrigin(request)
			}
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
			if test.transport != "" && response.Header().Get("X-Auth-Transport") != string(test.transport) {
				t.Fatalf("transport = %q, want %q", response.Header().Get("X-Auth-Transport"), test.transport)
			}
		})
	}
}

func TestRejectedLogoutDoesNotClearCookie(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	credentials, err := store.RedeemCode(context.Background(), auth.Principal{
		Subject:   "browser",
		Scope:     auth.ScopeWrite,
		TokenID:   "logout-code",
		Purpose:   auth.BrowserCodePurpose,
		ExpiresAt: now.Add(time.Minute),
	}, now)
	if err != nil {
		t.Fatal(err)
	}
	handler := testAPIHandler(t, manager, store)
	request := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/logout", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: credentials.SessionID})
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", response.Code)
	}
	if response.Header().Get("Set-Cookie") != "" {
		t.Fatal("rejected logout cleared the session cookie")
	}
	if _, err := store.GetSession(context.Background(), credentials.SessionID, time.Now()); err != nil {
		t.Fatalf("rejected logout revoked the session: %v", err)
	}
}

func TestBrowserExchangeRateLimitAndAuditHideSubmittedCode(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	handler := testAPIHandler(t, manager, store)
	const submittedCode = "submitted-browser-code-must-remain-secret"
	body := `{"code":"` + submittedCode + `"}`

	var output bytes.Buffer
	previousOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&output)
	t.Cleanup(func() {
		logrus.SetOutput(previousOutput)
	})

	for attempt := 1; attempt <= exchangeClientBurst+1; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/exchange", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		request.RemoteAddr = "192.0.2.55:1234"
		sameOrigin(request)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		want := http.StatusUnauthorized
		if attempt > exchangeClientBurst {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, want)
		}
		if want == http.StatusTooManyRequests && response.Header().Get("Retry-After") == "" {
			t.Fatal("rate-limited response has no Retry-After header")
		}
	}
	if strings.Contains(output.String(), submittedCode) {
		t.Fatal("authentication audit output contains the submitted browser code")
	}
}
