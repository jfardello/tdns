package httpapi

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contractapi "github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/browserauth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/internal/sqliteutil"
	"github.com/sirupsen/logrus"
)

const testAdministratorPassword = "correct horse battery staple"

type rejectingPasswordSessionStore struct {
	BrowserSessionStore
	attempts int
}

func (s *rejectingPasswordSessionStore) CreatePasswordSession(context.Context, string, []byte, time.Time, ...browserauth.SessionOptions) (browserauth.Credentials, error) {
	s.attempts++
	return browserauth.Credentials{}, browserauth.ErrInvalidCredentials
}

func passwordLoginRequestWithRemember(t *testing.T, username, password string, remember bool) *http.Request {
	t.Helper()
	body, err := json.Marshal(contractapi.BrowserPasswordLoginRequest{
		Username: username,
		Password: password,
		Remember: remember,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/login", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	sameOrigin(request)
	return request
}

func passwordLoginRequest(t *testing.T, username, password string) *http.Request {
	return passwordLoginRequestWithRemember(t, username, password, false)
}

func testPasswordBrowserStore(t *testing.T) (*browserauth.Store, *sql.DB) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tdns.sqlite")
	if _, err := db.Bootstrap(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	store, err := browserauth.Open(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	connection, err := sql.Open(sqliteutil.DriverName(), sqliteutil.ReadWriteDSN(path))
	if err != nil {
		t.Fatal(err)
	}
	if err := sqliteutil.ConfigureConnection(context.Background(), connection, true); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = connection.Close()
		_ = store.Close()
	})
	return store, connection
}

func TestBrowserPasswordLoginCreatesExistingOpaqueSession(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	if err := store.SetAdministratorPassword(context.Background(), "Admin", []byte(testAdministratorPassword), time.Now()); err != nil {
		t.Fatal(err)
	}
	handler := testAPIHandler(t, manager, store)
	request := passwordLoginRequest(t, " ADMIN ", testAdministratorPassword)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("password login response is cacheable")
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != sessionCookieName || cookie.Value == "" || cookie.Path != "/" ||
		!cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode ||
		cookie.Domain != "" || cookie.MaxAge != 0 || !cookie.Expires.IsZero() {
		t.Fatalf("cookie = %#v", cookie)
	}
	var session contractapi.BrowserSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.Subject != "admin" || session.Scope != auth.ScopeWrite || session.CSRFToken == "" {
		t.Fatalf("response = %#v", session)
	}
	stored, err := store.GetSession(context.Background(), cookie.Value, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if stored.AuthenticationMethod != browserauth.AuthenticationMethodPassword {
		t.Fatalf("authentication method = %q", stored.AuthenticationMethod)
	}
	if err := store.ValidateCSRF(context.Background(), cookie.Value, session.CSRFToken, time.Now()); err != nil {
		t.Fatalf("CSRF state = %v", err)
	}
}

func TestBrowserPasswordLoginCreatesRememberedSession(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	if err := store.SetAdministratorPassword(context.Background(), "admin", []byte(testAdministratorPassword), time.Now()); err != nil {
		t.Fatal(err)
	}
	const rememberDays = 5
	handler := testAPIHandlerWithRememberDays(t, manager, store, rememberDays)
	request := passwordLoginRequestWithRemember(t, "admin", testAdministratorPassword, true)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	cookie := cookies[0]
	if cookie.MaxAge != rememberDays*24*60*60 || cookie.Expires.IsZero() {
		t.Fatalf("remembered cookie = %#v", cookie)
	}
	stored, err := store.GetSession(context.Background(), cookie.Value, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if !stored.Persistent || stored.AuthenticationMethod != browserauth.AuthenticationMethodPassword ||
		stored.ExpiresAt.Sub(stored.CreatedAt) != rememberDays*24*time.Hour {
		t.Fatalf("stored remembered password session = %#v", stored)
	}
}

func TestBrowserPasswordLoginReturnsGenericInvalidCredentialResponse(t *testing.T) {
	manager := testAuthManager(t)
	store, connection := testPasswordBrowserStore(t)
	now := time.Now()
	if err := store.SetAdministratorPassword(context.Background(), "admin", []byte(testAdministratorPassword), now); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		username string
		password string
		prepare  func(*testing.T)
	}{
		{name: "wrong password", username: "admin", password: "incorrect password value"},
		{name: "unknown username", username: "unknown", password: testAdministratorPassword},
		{name: "invalid username", username: "../admin", password: testAdministratorPassword},
		{name: "short password", username: "admin", password: "short"},
		{name: "missing account", username: "admin", password: testAdministratorPassword, prepare: func(t *testing.T) {
			if _, err := connection.Exec(`DELETE FROM local_administrator`); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "disabled account", username: "admin", password: testAdministratorPassword, prepare: func(t *testing.T) {
			if err := store.DisableAdministrator(context.Background(), now); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "malformed hash", username: "admin", password: testAdministratorPassword, prepare: func(t *testing.T) {
			if _, err := connection.Exec(`UPDATE local_administrator SET password_hash = 'malformed'`); err != nil {
				t.Fatal(err)
			}
		}},
	}
	var expectedBody string
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := store.SetAdministratorPassword(context.Background(), "admin", []byte(testAdministratorPassword), now); err != nil {
				t.Fatal(err)
			}
			if test.prepare != nil {
				test.prepare(t)
			}
			handler := testAPIHandler(t, manager, store)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, passwordLoginRequest(t, test.username, test.password))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d: %s", response.Code, response.Body.String())
			}
			if expectedBody == "" {
				expectedBody = response.Body.String()
			} else if response.Body.String() != expectedBody {
				t.Fatalf("body = %q, want %q", response.Body.String(), expectedBody)
			}
		})
	}
}

func TestBrowserPasswordLoginRejectsMalformedAndCrossSiteRequests(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	handler := testAPIHandler(t, manager, store)
	oversized := `{"username":"admin","password":"` + strings.Repeat("x", maximumExchangeBodyBytes) + `"}`
	tests := []struct {
		name        string
		contentType string
		body        string
		origin      bool
		want        int
	}{
		{name: "non json", contentType: "text/plain", body: `{}`, origin: true, want: http.StatusUnsupportedMediaType},
		{name: "unknown field", contentType: "application/json", body: `{"username":"admin","password":"value","extra":true}`, origin: true, want: http.StatusBadRequest},
		{name: "trailing JSON", contentType: "application/json", body: `{"username":"admin","password":"value"} {}`, origin: true, want: http.StatusBadRequest},
		{name: "missing username", contentType: "application/json", body: `{"password":"value"}`, origin: true, want: http.StatusBadRequest},
		{name: "missing password", contentType: "application/json", body: `{"username":"admin"}`, origin: true, want: http.StatusBadRequest},
		{name: "oversized", contentType: "application/json", body: oversized, origin: true, want: http.StatusRequestEntityTooLarge},
		{name: "cross site", contentType: "application/json", body: `{"username":"admin","password":"value"}`, want: http.StatusForbidden},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/login", strings.NewReader(test.body))
			request.Header.Set("Content-Type", test.contentType)
			if test.origin {
				sameOrigin(request)
			} else {
				request.Header.Set("Sec-Fetch-Site", "cross-site")
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d: %s", response.Code, test.want, response.Body.String())
			}
		})
	}
}

func TestBrowserPasswordLoginRejectsExistingCredentials(t *testing.T) {
	manager := testAuthManager(t)
	store := testBrowserStore(t)
	handler := testAPIHandler(t, manager, store)
	for _, test := range []struct {
		name   string
		bearer bool
		cookie bool
	}{
		{name: "bearer", bearer: true},
		{name: "session", cookie: true},
		{name: "both", bearer: true, cookie: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := passwordLoginRequest(t, "admin", testAdministratorPassword)
			if test.bearer {
				request.Header.Set("Authorization", "Bearer token")
			}
			if test.cookie {
				request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d", response.Code)
			}
		})
	}
}

func TestBrowserPasswordLoginRateLimitAndAuditAreSecretSafe(t *testing.T) {
	manager := testAuthManager(t)
	store := &rejectingPasswordSessionStore{BrowserSessionStore: testBrowserStore(t)}
	handler := testAPIHandler(t, manager, store)
	const submittedPassword = "submitted password secret"

	var output bytes.Buffer
	previousOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&output)
	t.Cleanup(func() { logrus.SetOutput(previousOutput) })

	for attempt := 1; attempt <= passwordSourceBurst+1; attempt++ {
		username := "sensitive-admin"
		if attempt%2 == 0 {
			username = " SENSITIVE-ADMIN "
		}
		request := passwordLoginRequest(t, username, submittedPassword)
		request.RemoteAddr = fmt.Sprintf("192.0.2.%d:1234", attempt)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		want := http.StatusUnauthorized
		if attempt > passwordSourceBurst {
			want = http.StatusTooManyRequests
		}
		if response.Code != want {
			t.Fatalf("attempt %d status = %d, want %d", attempt, response.Code, want)
		}
		if want == http.StatusTooManyRequests && response.Header().Get("Retry-After") == "" {
			t.Fatal("rate-limited response has no Retry-After header")
		}
	}
	if store.attempts != passwordUsernameBurst {
		t.Fatalf("password verification attempts = %d, want %d", store.attempts, passwordUsernameBurst)
	}
	logOutput := output.String()
	for _, secret := range []string{"sensitive-admin", submittedPassword, testAdministratorPassword} {
		if strings.Contains(logOutput, secret) {
			t.Fatalf("authentication audit output contains %q", secret)
		}
	}

	metricsRequest := httptest.NewRequest(http.MethodGet, "https://tdns.example/metrics", nil)
	metricsResponse := httptest.NewRecorder()
	handler.ServeHTTP(metricsResponse, metricsRequest)
	metrics := metricsResponse.Body.String()
	if !strings.Contains(metrics, "tdns_browser_authentication_attempts_total") ||
		!strings.Contains(metrics, `method="password"`) ||
		!strings.Contains(metrics, "tdns_password_authentication_duration_seconds") {
		t.Fatalf("password authentication metrics missing:\n%s", metrics)
	}
	if strings.Contains(metrics, "sensitive-admin") {
		t.Fatal("metrics expose the submitted username")
	}
}
