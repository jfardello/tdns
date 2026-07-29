package httpapi

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/sirupsen/logrus"
)

func testAuthManager(t *testing.T) *auth.Manager {
	t.Helper()
	key := []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	manager, err := auth.NewManager(config.AuthConf{
		Issuer:         auth.DefaultIssuer,
		BearerAudience: auth.DefaultBearerAudience,
		ActiveKey: config.SigningKeyConf{
			ID:    "test-active",
			Value: base64.StdEncoding.EncodeToString(key),
		},
	}, "", auth.Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return manager
}

func TestAuthenticationAuditDoesNotLogCredential(t *testing.T) {
	var output bytes.Buffer
	logrus.SetOutput(&output)
	t.Cleanup(func() { logrus.SetOutput(os.Stderr) })

	const credential = "secret-bearer-credential"
	handler := Require(func(http.ResponseWriter, *http.Request) {
		t.Fatal("invalid credential reached handler")
	}, Requirement{IsRequired: true, Scope: auth.ScopeRead}, testAuthManager(t))
	request := httptest.NewRequest(http.MethodGet, "/api/cache", nil)
	request.Header.Set("Authorization", "Bearer "+credential)
	handler(httptest.NewRecorder(), request)

	if strings.Contains(output.String(), credential) {
		t.Fatal("authentication audit log contains the bearer credential")
	}
	if !strings.Contains(output.String(), "authentication_failed") {
		t.Fatal("authentication failure did not emit an audit event")
	}
}

func issueTestToken(t *testing.T, manager *auth.Manager, scope string) string {
	t.Helper()
	token, err := manager.IssueBearer("test", scope, time.Hour)
	if err != nil {
		t.Fatalf("IssueBearer: %v", err)
	}
	return token
}

func TestRequireAcceptsOnlyBearerScheme(t *testing.T) {
	manager := testAuthManager(t)
	token := issueTestToken(t, manager, auth.ScopeWrite)
	handler := Require(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, Requirement{IsRequired: true, Scope: auth.ScopeWrite}, manager)

	for _, test := range []struct {
		name       string
		header     string
		wantStatus int
	}{
		{name: "bearer", header: "Bearer " + token, wantStatus: http.StatusNoContent},
		{name: "lowercase bearer", header: "bearer " + token, wantStatus: http.StatusNoContent},
		{name: "wrong scheme", header: "Basic " + token, wantStatus: http.StatusUnauthorized},
		{name: "missing scheme", header: token, wantStatus: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("Authorization", test.header)
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}

func TestRequireDistinguishesAuthenticationAndAuthorization(t *testing.T) {
	manager := testAuthManager(t)
	readToken := issueTestToken(t, manager, auth.ScopeRead)
	writeToken := issueTestToken(t, manager, auth.ScopeWrite)
	handler := Require(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := PrincipalFromContext(r.Context()); !ok {
			t.Error("authenticated request has no principal")
		}
		w.WriteHeader(http.StatusNoContent)
	}, Requirement{IsRequired: true, Scope: auth.ScopeWrite}, manager)

	for _, test := range []struct {
		name       string
		token      string
		wantStatus int
	}{
		{name: "missing", wantStatus: http.StatusUnauthorized},
		{name: "read only", token: readToken, wantStatus: http.StatusForbidden},
		{name: "read write", token: writeToken, wantStatus: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", nil)
			if test.token != "" {
				request.Header.Set("Authorization", "Bearer "+test.token)
			}
			response := httptest.NewRecorder()
			handler(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
			}
		})
	}
}
