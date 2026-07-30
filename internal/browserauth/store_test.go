package browserauth

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/db"
)

func newTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tdns.sqlite")
	if _, err := db.Bootstrap(context.Background(), dbPath); err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	store, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() {
		_ = store.Close()
	})
	return store, dbPath
}

func codePrincipal(id string, now time.Time) auth.Principal {
	return auth.Principal{
		Subject:   "operator",
		Scope:     auth.ScopeWrite,
		TokenID:   id,
		Purpose:   auth.BrowserCodePurpose,
		IssuedAt:  now,
		ExpiresAt: now.Add(auth.BrowserCodeTTL),
	}
}

func TestRedeemCodeStoresOnlyHashedSecrets(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	principal := codePrincipal("browser-code-jti", now)

	credentials, err := store.RedeemCode(ctx, principal, now)
	if err != nil {
		t.Fatalf("RedeemCode: %v", err)
	}
	if credentials.SessionID == "" || credentials.CSRFToken == "" {
		t.Fatal("RedeemCode returned an empty secret")
	}
	if credentials.Session.ExpiresAt.Sub(credentials.Session.CreatedAt) != auth.BrowserSessionTTL {
		t.Fatalf("session lifetime = %s", credentials.Session.ExpiresAt.Sub(credentials.Session.CreatedAt))
	}

	var sessionHash, csrfHash []byte
	if err := store.conn.QueryRow(`
SELECT session_hash, csrf_hash
FROM browser_sessions`).Scan(&sessionHash, &csrfHash); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(sessionHash, hashSecret(credentials.SessionID)) ||
		!bytes.Equal(csrfHash, hashSecret(credentials.CSRFToken)) {
		t.Fatal("stored browser secrets are not their expected hashes")
	}
	if bytes.Equal(sessionHash, []byte(credentials.SessionID)) ||
		bytes.Equal(csrfHash, []byte(credentials.CSRFToken)) {
		t.Fatal("raw browser secret was persisted")
	}

	var codeHash []byte
	if err := store.conn.QueryRow(`SELECT code_hash FROM consumed_browser_codes`).Scan(&codeHash); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(codeHash, hashSecret(principal.TokenID)) ||
		bytes.Equal(codeHash, []byte(principal.TokenID)) {
		t.Fatal("browser code identifier was not stored exclusively as a hash")
	}
}

func TestConcurrentCodeRedemptionSucceedsOnce(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	principal := codePrincipal("concurrent-code", now)

	const attempts = 24
	var successes atomic.Int32
	var consumed atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for range attempts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := store.RedeemCode(ctx, principal, now)
			switch {
			case err == nil:
				successes.Add(1)
			case errors.Is(err, ErrCodeConsumed):
				consumed.Add(1)
			default:
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("unexpected redemption error: %v", err)
	}
	if successes.Load() != 1 || consumed.Load() != attempts-1 {
		t.Fatalf("successes = %d, consumed = %d", successes.Load(), consumed.Load())
	}
}

func TestSessionPersistenceExpirationCSRFAndRevocation(t *testing.T) {
	store, dbPath := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	credentials, err := store.RedeemCode(ctx, codePrincipal("persistent-code", now), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateCSRF(ctx, credentials.SessionID, "wrong", now); !errors.Is(err, ErrInvalidCSRF) {
		t.Fatalf("wrong CSRF error = %v", err)
	}
	if err := store.ValidateCSRF(ctx, credentials.SessionID, credentials.CSRFToken, now); err != nil {
		t.Fatalf("ValidateCSRF: %v", err)
	}
	issued := make([]string, 0, MaxCSRFTokens+2)
	for i := range MaxCSRFTokens + 2 {
		token, err := store.IssueCSRF(ctx, credentials.SessionID, now.Add(time.Duration(i+1)*time.Nanosecond))
		if err != nil {
			t.Fatalf("IssueCSRF: %v", err)
		}
		issued = append(issued, token)
	}
	if err := store.ValidateCSRF(ctx, credentials.SessionID, credentials.CSRFToken, now); !errors.Is(err, ErrInvalidCSRF) {
		t.Fatalf("initial CSRF token remained valid after bounded eviction: %v", err)
	}
	for i, token := range issued {
		err := store.ValidateCSRF(ctx, credentials.SessionID, token, now)
		if i < len(issued)-MaxCSRFTokens {
			if !errors.Is(err, ErrInvalidCSRF) {
				t.Fatalf("evicted token %d error = %v", i, err)
			}
		} else if err != nil {
			t.Fatalf("retained token %d error = %v", i, err)
		}
	}
	var csrfCount int
	if err := store.conn.QueryRow(
		`SELECT COUNT(*) FROM browser_session_csrf_tokens`,
	).Scan(&csrfCount); err != nil {
		t.Fatal(err)
	}
	if csrfCount != MaxCSRFTokens {
		t.Fatalf("stored CSRF tokens = %d, want %d", csrfCount, MaxCSRFTokens)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(ctx, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()

	session, err := reopened.GetSession(ctx, credentials.SessionID, credentials.Session.ExpiresAt.Add(-time.Second))
	if err != nil {
		t.Fatalf("GetSession before expiry: %v", err)
	}
	if session.Subject != "operator" || session.Scope != auth.ScopeWrite {
		t.Fatalf("session = %#v", session)
	}
	if _, err := reopened.GetSession(ctx, credentials.SessionID, credentials.Session.ExpiresAt); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("expiration-boundary error = %v", err)
	}
	if err := reopened.RevokeSession(ctx, credentials.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.GetSession(ctx, credentials.SessionID, now); !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("revoked-session error = %v", err)
	}
}

func TestPurgeExpiredIsBounded(t *testing.T) {
	store, _ := newTestStore(t)
	ctx := context.Background()
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	for i, id := range []string{"one", "two", "three"} {
		principal := codePrincipal(id, now.Add(time.Duration(i)*time.Second))
		if _, err := store.RedeemCode(ctx, principal, principal.IssuedAt); err != nil {
			t.Fatal(err)
		}
	}
	purgeAt := now.Add(auth.BrowserSessionTTL + time.Minute)
	sessions, codes, err := store.PurgeExpired(ctx, purgeAt, 2)
	if err != nil {
		t.Fatal(err)
	}
	if sessions != 2 || codes != 2 {
		t.Fatalf("first purge removed sessions=%d codes=%d", sessions, codes)
	}
	sessions, codes, err = store.PurgeExpired(ctx, purgeAt, 2)
	if err != nil {
		t.Fatal(err)
	}
	if sessions != 1 || codes != 1 {
		t.Fatalf("second purge removed sessions=%d codes=%d", sessions, codes)
	}
	if _, _, err := store.PurgeExpired(ctx, purgeAt, 0); err == nil {
		t.Fatal("PurgeExpired accepted an unbounded zero limit")
	}
}
