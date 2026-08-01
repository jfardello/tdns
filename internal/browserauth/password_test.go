package browserauth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jfardello/tdns/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

func TestAdministratorPasswordValidationBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		password []byte
		wantErr  bool
	}{
		{name: "minimum", password: []byte(strings.Repeat("a", AdministratorPasswordMin))},
		{name: "maximum", password: []byte(strings.Repeat("a", AdministratorPasswordMax))},
		{name: "below minimum", password: []byte(strings.Repeat("a", AdministratorPasswordMin-1)), wantErr: true},
		{name: "above maximum", password: []byte(strings.Repeat("a", AdministratorPasswordMax+1)), wantErr: true},
		{name: "invalid UTF-8", password: []byte{0xff, 0xfe, 0xfd}, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateAdministratorPassword(test.password)
			if (err != nil) != test.wantErr {
				t.Fatalf("error = %v, wantErr = %t", err, test.wantErr)
			}
		})
	}
}

func TestSetAdministratorStoresCost12HashAndNormalizesUsername(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	password := []byte("correct horse battery staple")
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)

	if err := store.SetAdministratorPassword(ctx, " Admin.User ", password, now); err != nil {
		t.Fatal(err)
	}
	credential, err := store.Administrator(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Username != "admin.user" || credential.Subject != "admin.user" || credential.Scope != auth.ScopeWrite {
		t.Fatalf("credential = %#v", credential)
	}
	if bytes.Contains(credential.PasswordHash, password) {
		t.Fatal("stored credential contains the plaintext password")
	}
	if cost, err := bcrypt.Cost(credential.PasswordHash); err != nil || cost != AdministratorBcryptCost {
		t.Fatalf("bcrypt cost = %d, error = %v", cost, err)
	}
	if err := bcrypt.CompareHashAndPassword(credential.PasswordHash, password); err != nil {
		t.Fatalf("stored hash does not verify: %v", err)
	}
}

func TestAdministratorFailsClosedForUnavailableHashes(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	if _, err := store.Administrator(ctx); !errors.Is(err, ErrAdministratorUnavailable) {
		t.Fatalf("missing credential error = %v", err)
	}
	if err := store.SetAdministratorPassword(ctx, "admin", []byte("correct horse battery staple"), now); err != nil {
		t.Fatal(err)
	}

	for _, hash := range [][]byte{
		[]byte("not-a-bcrypt-hash"),
		mustHashAtCost(t, []byte("correct horse battery staple"), 10),
	} {
		if _, err := store.conn.Exec(`UPDATE local_administrator SET password_hash = ?`, hash); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Administrator(ctx); !errors.Is(err, ErrAdministratorUnavailable) {
			t.Fatalf("invalid hash error = %v", err)
		}
	}
	if _, err := store.conn.Exec(`UPDATE local_administrator SET enabled = 0`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Administrator(ctx); !errors.Is(err, ErrAdministratorUnavailable) {
		t.Fatalf("disabled credential error = %v", err)
	}
}

func TestAdministratorReplacementAndDisableRevokeOnlyPasswordSessions(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	if err := store.SetAdministratorPassword(ctx, "first", []byte("first password value"), now); err != nil {
		t.Fatal(err)
	}
	insertTestSession(t, store, "browser-session", AuthenticationMethodBrowserCode, now)
	insertTestSession(t, store, "password-session", AuthenticationMethodPassword, now)

	if err := store.SetAdministratorPassword(ctx, "second", []byte("second password value"), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	credential, err := store.Administrator(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if credential.Username != "second" || !credential.CreatedAt.Equal(now) || !credential.UpdatedAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("replacement credential = %#v", credential)
	}
	assertSessionCount(t, store, AuthenticationMethodBrowserCode, 1)
	assertSessionCount(t, store, AuthenticationMethodPassword, 0)

	insertTestSession(t, store, "new-password-session", AuthenticationMethodPassword, now)
	if err := store.DisableAdministrator(ctx, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Administrator(ctx); !errors.Is(err, ErrAdministratorUnavailable) {
		t.Fatalf("disabled credential error = %v", err)
	}
	assertSessionCount(t, store, AuthenticationMethodBrowserCode, 1)
	assertSessionCount(t, store, AuthenticationMethodPassword, 0)
}

func TestConcurrentAdministratorReplacementRemainsConsistent(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	passwords := map[string][]byte{
		"first":  []byte("first password value"),
		"second": []byte("second password value"),
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(passwords))
	for username, password := range passwords {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- store.SetAdministratorPassword(ctx, username, password, now)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	credential, err := store.Administrator(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := bcrypt.CompareHashAndPassword(credential.PasswordHash, passwords[credential.Username]); err != nil {
		t.Fatalf("username and password hash were not updated atomically: %v", err)
	}
}

func TestCreatePasswordSessionUsesOpaqueSessionAndCSRFState(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 2, 10, 0, 0, 123, time.UTC)
	password := []byte("correct horse battery staple")
	if err := store.SetAdministratorPassword(ctx, "Admin", password, now.Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}

	credentials, err := store.CreatePasswordSession(ctx, " ADMIN ", password, now)
	if err != nil {
		t.Fatal(err)
	}
	if credentials.SessionID == "" || credentials.CSRFToken == "" {
		t.Fatal("password login returned an empty opaque secret")
	}
	if credentials.Session.Subject != "admin" ||
		credentials.Session.Scope != auth.ScopeWrite ||
		credentials.Session.AuthenticationMethod != AuthenticationMethodPassword ||
		credentials.Session.ExpiresAt.Sub(credentials.Session.CreatedAt) != auth.BrowserSessionTTL {
		t.Fatalf("session = %#v", credentials.Session)
	}
	if err := store.ValidateCSRF(ctx, credentials.SessionID, credentials.CSRFToken, now); err != nil {
		t.Fatalf("ValidateCSRF: %v", err)
	}
	stored, err := store.GetSession(ctx, credentials.SessionID, now)
	if err != nil {
		t.Fatal(err)
	}
	if stored.AuthenticationMethod != AuthenticationMethodPassword {
		t.Fatalf("stored authentication method = %q", stored.AuthenticationMethod)
	}
}

func TestCreatePasswordSessionRejectsInvalidCredentialsGenerically(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	password := []byte("correct horse battery staple")
	if err := store.SetAdministratorPassword(ctx, "admin", password, now); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		username string
		password []byte
		prepare  func(*testing.T)
	}{
		{name: "wrong password", username: "admin", password: []byte("incorrect password value")},
		{name: "unknown username", username: "unknown", password: password},
		{name: "invalid username", username: "../admin", password: password},
		{name: "short password", username: "admin", password: []byte("short")},
		{name: "long password", username: "admin", password: []byte(strings.Repeat("x", AdministratorPasswordMax+1))},
		{name: "disabled account", username: "admin", password: password, prepare: func(t *testing.T) {
			if _, err := store.conn.Exec(`UPDATE local_administrator SET enabled = 0`); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "malformed hash", username: "admin", password: password, prepare: func(t *testing.T) {
			if _, err := store.conn.Exec(`UPDATE local_administrator SET enabled = 1, password_hash = 'malformed'`); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := store.SetAdministratorPassword(ctx, "admin", password, now); err != nil {
				t.Fatal(err)
			}
			if test.prepare != nil {
				test.prepare(t)
			}
			if _, err := store.CreatePasswordSession(ctx, test.username, test.password, now); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("error = %v", err)
			}
			assertSessionCount(t, store, AuthenticationMethodPassword, 0)
		})
	}
}

func TestUnavailableAdministratorUsesCost12DummyComparison(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	if cost, err := bcrypt.Cost([]byte(dummyAdministratorHash)); err != nil || cost != AdministratorBcryptCost {
		t.Fatalf("dummy bcrypt cost = %d, error = %v", cost, err)
	}

	type comparison struct {
		hash     []byte
		password []byte
	}
	var comparisons []comparison
	store.comparePassword = func(hash, password []byte) error {
		comparisons = append(comparisons, comparison{
			hash:     append([]byte(nil), hash...),
			password: append([]byte(nil), password...),
		})
		return bcrypt.ErrMismatchedHashAndPassword
	}

	for _, test := range []struct {
		name     string
		username string
		password []byte
		prepare  func(*testing.T)
	}{
		{name: "missing", username: "admin", password: []byte("valid password value")},
		{name: "unknown", username: "unknown", password: []byte("valid password value"), prepare: func(t *testing.T) {
			storeValidAdministrator(t, store, now)
		}},
		{name: "disabled", username: "admin", password: []byte("valid password value"), prepare: func(t *testing.T) {
			storeValidAdministrator(t, store, now)
			if _, err := store.conn.Exec(`UPDATE local_administrator SET enabled = 0`); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "malformed", username: "admin", password: []byte("valid password value"), prepare: func(t *testing.T) {
			storeValidAdministrator(t, store, now)
			if _, err := store.conn.Exec(`UPDATE local_administrator SET password_hash = 'malformed'`); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.conn.Exec(`DELETE FROM local_administrator`); err != nil {
				t.Fatal(err)
			}
			if test.prepare != nil {
				test.prepare(t)
			}
			comparisons = nil
			if _, err := store.CreatePasswordSession(ctx, test.username, test.password, now); !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("error = %v", err)
			}
			if len(comparisons) != 1 || !bytes.Equal(comparisons[0].hash, []byte(dummyAdministratorHash)) {
				t.Fatalf("comparisons = %#v", comparisons)
			}
		})
	}
}

func TestPasswordRotationCannotLeaveAnOldPasswordSession(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	oldPassword := []byte("old administrator password")
	if err := store.SetAdministratorPassword(ctx, "admin", oldPassword, now); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() {
		<-start
		_, err := store.CreatePasswordSession(ctx, "admin", oldPassword, now.Add(time.Minute))
		errs <- err
	}()
	go func() {
		<-start
		errs <- store.SetAdministratorPassword(ctx, "admin", []byte("new administrator password"), now.Add(time.Minute))
	}()
	close(start)
	for range 2 {
		err := <-errs
		if err != nil && !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("concurrent credential operation: %v", err)
		}
	}
	assertSessionCount(t, store, AuthenticationMethodPassword, 0)
}

func storeValidAdministrator(t *testing.T, store *Store, now time.Time) {
	t.Helper()
	hash := mustHashAtCost(t, []byte("valid password value"), AdministratorBcryptCost)
	if _, err := store.conn.Exec(`
INSERT INTO local_administrator
	(singleton, username, password_hash, subject, scope, enabled, created_at, updated_at)
VALUES (1, 'admin', ?, 'admin', ?, 1, ?, ?)`, hash, auth.ScopeWrite, now.Unix(), now.Unix()); err != nil {
		t.Fatal(err)
	}
}

func mustHashAtCost(t *testing.T, password []byte, cost int) []byte {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword(password, cost)
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func insertTestSession(t *testing.T, store *Store, id, method string, now time.Time) {
	t.Helper()
	secretHash := hashSecret(id)
	if _, err := store.conn.Exec(`
INSERT INTO browser_sessions
	(session_hash, subject, scope, csrf_hash, created_at, last_used_at, expires_at, authentication_method)
VALUES (?, 'admin', ?, ?, ?, ?, ?, ?)`,
		secretHash, auth.ScopeWrite, hashSecret(id+"-csrf"), now.Unix(), now.Unix(), now.Add(time.Hour).Unix(), method,
	); err != nil {
		t.Fatal(err)
	}
}

func assertSessionCount(t *testing.T, store *Store, method string, want int) {
	t.Helper()
	var got int
	if err := store.conn.QueryRow(
		`SELECT COUNT(*) FROM browser_sessions WHERE authentication_method = ?`, method,
	).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s session count = %d, want %d", method, got, want)
	}
}
