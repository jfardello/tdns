package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	contractapi "github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/browserauth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/internal/httpapi"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

const lifecycleSessionCookie = "__Host-tdns-session"

func TestAdministratorAuthenticationLifecycle(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "tdns.sqlite")
	if _, err := db.Bootstrap(ctx, dbPath); err != nil {
		t.Fatalf("bootstrap migrations: %v", err)
	}
	var logOutput bytes.Buffer
	previousLogOutput := logrus.StandardLogger().Out
	logrus.SetOutput(&logOutput)
	t.Cleanup(func() { logrus.SetOutput(previousLogOutput) })

	oldPassword := "initial administrator password"
	newPassword := "rotated administrator password"
	var commandOutput strings.Builder
	var apiErrorOutput strings.Builder
	runPasswordCommand(t, dbPath, &commandOutput, []string{"set", "--username", "Admin"}, oldPassword, oldPassword)

	store := openLifecycleStore(t, dbPath)
	manager := lifecycleAuthManager(t)
	config.SetRunningConfig(&config.Config{Auth: config.AuthConf{
		Browser: config.BrowserAuthConf{RememberDays: 2},
	}})
	handler := lifecycleHandler(t, manager, store)

	passwordResponse := lifecyclePasswordLogin(t, handler, "admin", oldPassword, false)
	passwordCookie, passwordSession := lifecycleLoginResult(t, passwordResponse)
	initialPasswordCookieValue := passwordCookie.Value
	if passwordCookie.MaxAge != 0 || !passwordCookie.Expires.IsZero() {
		t.Fatalf("non-persistent password cookie = %#v", passwordCookie)
	}

	code, err := manager.IssueBrowserCode("recovery-admin", auth.ScopeWrite, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	codeResponse := lifecycleCodeLogin(t, handler, code, true)
	codeCookie, codeSession := lifecycleLoginResult(t, codeResponse)
	if codeCookie.MaxAge != 2*24*60*60 || codeCookie.Expires.IsZero() {
		t.Fatalf("remembered browser-code cookie = %#v", codeCookie)
	}
	if replay := lifecycleCodeLogin(t, handler, code, true); replay.Code != http.StatusUnauthorized {
		t.Fatalf("browser-code replay status = %d, want 401", replay.Code)
	} else {
		apiErrorOutput.Write(replay.Body.Bytes())
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store = openLifecycleStore(t, dbPath)
	handler = lifecycleHandler(t, manager, store)
	if response := lifecycleSessionStatus(handler, codeCookie); response.Code != http.StatusOK {
		t.Fatalf("remembered session after restart = %d: %s", response.Code, response.Body.String())
	}

	missingCSRF := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/logout", nil)
	missingCSRF.AddCookie(passwordCookie)
	missingCSRF.Header.Set("Sec-Fetch-Site", "same-origin")
	if response := serveLifecycle(handler, missingCSRF); response.Code != http.StatusForbidden {
		t.Fatalf("logout without CSRF status = %d, want 403", response.Code)
	} else {
		apiErrorOutput.Write(response.Body.Bytes())
	}
	logout := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/logout", nil)
	logout.AddCookie(passwordCookie)
	logout.Header.Set("Sec-Fetch-Site", "same-origin")
	logout.Header.Set("X-CSRF-Token", passwordSession.CSRFToken)
	if response := serveLifecycle(handler, logout); response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d: %s", response.Code, response.Body.String())
	}

	passwordResponse = lifecyclePasswordLogin(t, handler, "admin", oldPassword, true)
	passwordCookie, rotatedCandidateSession := lifecycleLoginResult(t, passwordResponse)
	runPasswordCommand(t, dbPath, &commandOutput, []string{"set", "--username", "Admin"}, newPassword, newPassword)
	if response := lifecycleSessionStatus(handler, passwordCookie); response.Code != http.StatusUnauthorized {
		t.Fatalf("rotated password session status = %d, want 401", response.Code)
	}
	if response := lifecyclePasswordLogin(t, handler, "admin", oldPassword, false); response.Code != http.StatusUnauthorized {
		t.Fatalf("old password status = %d, want 401", response.Code)
	} else {
		apiErrorOutput.Write(response.Body.Bytes())
	}
	newPasswordResponse := lifecyclePasswordLogin(t, handler, "admin", newPassword, false)
	newPasswordCookie, newPasswordSession := lifecycleLoginResult(t, newPasswordResponse)
	if response := lifecycleSessionStatus(handler, codeCookie); response.Code != http.StatusOK {
		t.Fatalf("recovery session after password rotation = %d: %s", response.Code, response.Body.String())
	}
	credential, err := store.Administrator(ctx)
	if err != nil {
		t.Fatal(err)
	}
	credentialHash := string(credential.PasswordHash)

	runPasswordCommand(t, dbPath, &commandOutput, []string{"disable"})
	if response := lifecycleSessionStatus(handler, newPasswordCookie); response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled password session status = %d, want 401", response.Code)
	}
	if response := lifecyclePasswordLogin(t, handler, "admin", newPassword, false); response.Code != http.StatusUnauthorized {
		t.Fatalf("disabled password login status = %d, want 401", response.Code)
	} else {
		apiErrorOutput.Write(response.Body.Bytes())
	}
	if response := lifecycleSessionStatus(handler, codeCookie); response.Code != http.StatusOK {
		t.Fatalf("recovery session after password disable = %d: %s", response.Code, response.Body.String())
	}

	past := time.Now().Add(-13 * time.Hour)
	expired, err := store.RedeemCode(ctx, auth.Principal{
		Subject:   "expired",
		Scope:     auth.ScopeWrite,
		TokenID:   "expired-code-identifier",
		Purpose:   auth.BrowserCodePurpose,
		ExpiresAt: past.Add(time.Minute),
	}, past)
	if err != nil {
		t.Fatal(err)
	}
	sessions, codes, err := store.PurgeExpired(ctx, time.Now(), 1)
	if err != nil || sessions != 1 || codes != 1 {
		t.Fatalf("purge expired records = sessions %d, codes %d, error %v", sessions, codes, err)
	}
	if _, err := store.GetSession(ctx, expired.SessionID, time.Now()); !errors.Is(err, browserauth.ErrSessionNotFound) {
		t.Fatalf("purged session error = %v", err)
	}
	if _, err := store.Administrator(ctx); !errors.Is(err, browserauth.ErrAdministratorUnavailable) {
		t.Fatalf("disabled administrator error = %v", err)
	}

	secrets := []string{
		oldPassword,
		newPassword,
		code,
		codeCookie.Value,
		codeSession.CSRFToken,
		initialPasswordCookieValue,
		passwordCookie.Value,
		passwordSession.CSRFToken,
		rotatedCandidateSession.CSRFToken,
		newPasswordCookie.Value,
		newPasswordSession.CSRFToken,
		expired.SessionID,
		expired.CSRFToken,
		credentialHash,
	}
	if strings.Contains(commandOutput.String(), oldPassword) || strings.Contains(commandOutput.String(), newPassword) {
		t.Fatal("password command output contains a plaintext password")
	}
	for _, secret := range secrets {
		if secret != "" && strings.Contains(logOutput.String(), secret) {
			t.Fatalf("authentication logs contain secret %q", secret)
		}
		if secret != "" && strings.Contains(apiErrorOutput.String(), secret) {
			t.Fatalf("authentication API error contains secret %q", secret)
		}
	}
	assertLifecycleSecretsAbsentFromSQLite(t, dbPath, secrets[:len(secrets)-1])
}

func runPasswordCommand(t *testing.T, dbPath string, output *strings.Builder, args []string, passwords ...string) {
	t.Helper()
	index := 0
	command := newAdministratorPasswordCommand(administratorPasswordCommandDependencies{
		openStore: func(ctx context.Context) (administratorPasswordStore, error) {
			return browserauth.Open(ctx, dbPath)
		},
		prompt: func(_ *cobra.Command, _ string) ([]byte, error) {
			if index >= len(passwords) {
				return nil, errors.New("unexpected password prompt")
			}
			password := []byte(passwords[index])
			index++
			return password, nil
		},
		now: time.Now,
	})
	command.SetOut(output)
	command.SetErr(output)
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		t.Fatalf("password command %v: %v", args, err)
	}
}

func openLifecycleStore(t *testing.T, dbPath string) *browserauth.Store {
	t.Helper()
	store, err := browserauth.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func lifecycleAuthManager(t *testing.T) *auth.Manager {
	t.Helper()
	key := []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	manager, err := auth.NewManager(config.AuthConf{
		ActiveKey: config.SigningKeyConf{
			ID:    "lifecycle-test",
			Value: base64.StdEncoding.EncodeToString(key),
		},
	}, "", auth.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func lifecycleHandler(t *testing.T, manager *auth.Manager, store *browserauth.Store) http.Handler {
	t.Helper()
	handler, err := httpapi.NewHandler(nil, manager, store)
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func lifecyclePasswordLogin(t *testing.T, handler http.Handler, username, password string, remember bool) *httptest.ResponseRecorder {
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
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	return serveLifecycle(handler, request)
}

func lifecycleCodeLogin(t *testing.T, handler http.Handler, code string, remember bool) *httptest.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(contractapi.BrowserCodeExchangeRequest{Code: code, Remember: remember})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "https://tdns.example/api/auth/exchange", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	return serveLifecycle(handler, request)
}

func lifecycleLoginResult(t *testing.T, response *httptest.ResponseRecorder) (*http.Cookie, contractapi.BrowserSessionResponse) {
	t.Helper()
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d: %s", response.Code, response.Body.String())
	}
	var session contractapi.BrowserSessionResponse
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == lifecycleSessionCookie {
			if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" || cookie.Domain != "" {
				t.Fatalf("session cookie = %#v", cookie)
			}
			return cookie, session
		}
	}
	t.Fatal("login response has no session cookie")
	return nil, contractapi.BrowserSessionResponse{}
}

func lifecycleSessionStatus(handler http.Handler, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, "https://tdns.example/api/auth/session", nil)
	request.AddCookie(cookie)
	return serveLifecycle(handler, request)
}

func serveLifecycle(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func assertLifecycleSecretsAbsentFromSQLite(t *testing.T, dbPath string, secrets []string) {
	t.Helper()
	var persisted []byte
	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		content, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			t.Fatal(err)
		}
		persisted = append(persisted, content...)
	}
	for _, secret := range secrets {
		if secret != "" && bytes.Contains(persisted, []byte(secret)) {
			t.Fatalf("SQLite artifacts contain secret %q", secret)
		}
	}
}
