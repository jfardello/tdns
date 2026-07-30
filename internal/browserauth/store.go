package browserauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/internal/sqliteutil"
)

const (
	DefaultPurgeLimit = 500
	MaxCSRFTokens     = 8
	opaqueSecretBytes = 32
)

var (
	ErrCodeConsumed    = errors.New("browser login code has already been consumed")
	ErrCodeExpired     = errors.New("browser login code expired")
	ErrSessionNotFound = errors.New("browser session not found")
	ErrSessionExpired  = errors.New("browser session expired")
	ErrInvalidCSRF     = errors.New("invalid CSRF token")
)

type Session struct {
	Subject   string
	Scope     string
	CreatedAt time.Time
	LastUsed  time.Time
	ExpiresAt time.Time
}

type Credentials struct {
	SessionID string
	CSRFToken string
	Session   Session
}

type Store struct {
	conn *sql.DB
}

func Open(ctx context.Context, dbPath string) (*Store, error) {
	resolvedPath, err := db.ResolvePath(dbPath)
	if err != nil {
		return nil, err
	}
	conn, err := sql.Open(sqliteutil.DriverName(), sqliteutil.ReadWriteDSN(resolvedPath))
	if err != nil {
		return nil, err
	}
	// Browser authentication writes are small and infrequent. A single
	// connection makes concurrent code redemption deterministic.
	conn.SetMaxOpenConns(1)
	if err := sqliteutil.ConfigureConnection(ctx, conn, true); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &Store{conn: conn}, nil
}

func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	return s.conn.Close()
}

// RedeemCode atomically consumes a validated browser code and creates a
// session. The code, session identifier, and CSRF token are never persisted.
func (s *Store) RedeemCode(ctx context.Context, principal auth.Principal, now time.Time) (Credentials, error) {
	if principal.Purpose != auth.BrowserCodePurpose ||
		strings.TrimSpace(principal.TokenID) == "" ||
		strings.TrimSpace(principal.Subject) == "" ||
		(principal.Scope != auth.ScopeRead && principal.Scope != auth.ScopeWrite) {
		return Credentials{}, errors.New("invalid browser login code principal")
	}
	now = now.UTC()
	if !now.Before(principal.ExpiresAt) {
		return Credentials{}, ErrCodeExpired
	}
	sessionID, err := randomSecret()
	if err != nil {
		return Credentials{}, err
	}
	csrfToken, err := randomSecret()
	if err != nil {
		return Credentials{}, err
	}
	session := Session{
		Subject:   principal.Subject,
		Scope:     principal.Scope,
		CreatedAt: time.Unix(now.Unix(), 0).UTC(),
		LastUsed:  time.Unix(now.Unix(), 0).UTC(),
		ExpiresAt: time.Unix(now.Add(auth.BrowserSessionTTL).Unix(), 0).UTC(),
	}

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return Credentials{}, err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	result, err := tx.ExecContext(ctx, `
INSERT INTO consumed_browser_codes (code_hash, consumed_at, expires_at)
VALUES (?, ?, ?)
ON CONFLICT(code_hash) DO NOTHING`,
		hashSecret(principal.TokenID), now.Unix(), principal.ExpiresAt.Unix(),
	)
	if err != nil {
		return Credentials{}, fmt.Errorf("consume browser login code: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return Credentials{}, err
	}
	if affected != 1 {
		return Credentials{}, ErrCodeConsumed
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO browser_sessions
	(session_hash, subject, scope, csrf_hash, created_at, last_used_at, expires_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		hashSecret(sessionID),
		session.Subject,
		session.Scope,
		hashSecret(csrfToken),
		session.CreatedAt.Unix(),
		session.LastUsed.Unix(),
		session.ExpiresAt.Unix(),
	); err != nil {
		return Credentials{}, fmt.Errorf("create browser session: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO browser_session_csrf_tokens (session_hash, csrf_hash, created_at)
VALUES (?, ?, ?)`,
		hashSecret(sessionID),
		hashSecret(csrfToken),
		now.UnixNano(),
	); err != nil {
		return Credentials{}, fmt.Errorf("create browser session CSRF token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Credentials{}, err
	}
	return Credentials{
		SessionID: sessionID,
		CSRFToken: csrfToken,
		Session:   session,
	}, nil
}

func (s *Store) GetSession(ctx context.Context, sessionID string, now time.Time) (Session, error) {
	if sessionID == "" {
		return Session{}, ErrSessionNotFound
	}
	var session Session
	var createdAt, lastUsed, expiresAt int64
	err := s.conn.QueryRowContext(ctx, `
SELECT subject, scope, created_at, last_used_at, expires_at
FROM browser_sessions
WHERE session_hash = ?`,
		hashSecret(sessionID),
	).Scan(&session.Subject, &session.Scope, &createdAt, &lastUsed, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrSessionNotFound
	}
	if err != nil {
		return Session{}, err
	}
	session.CreatedAt = time.Unix(createdAt, 0).UTC()
	session.LastUsed = time.Unix(lastUsed, 0).UTC()
	session.ExpiresAt = time.Unix(expiresAt, 0).UTC()
	if !now.Before(session.ExpiresAt) {
		return Session{}, ErrSessionExpired
	}
	return session, nil
}

func (s *Store) TouchSession(ctx context.Context, sessionID string, now time.Time) error {
	if sessionID == "" {
		return ErrSessionNotFound
	}
	result, err := s.conn.ExecContext(ctx, `
UPDATE browser_sessions
SET last_used_at = ?
WHERE session_hash = ? AND expires_at > ?`,
		now.UTC().Unix(), hashSecret(sessionID), now.UTC().Unix(),
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrSessionNotFound
	}
	return nil
}

func (s *Store) RevokeSession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return ErrSessionNotFound
	}
	_, err := s.conn.ExecContext(ctx, `DELETE FROM browser_sessions WHERE session_hash = ?`, hashSecret(sessionID))
	return err
}

func (s *Store) ValidateCSRF(ctx context.Context, sessionID, csrfToken string, now time.Time) error {
	if sessionID == "" {
		return ErrSessionNotFound
	}
	if csrfToken == "" {
		return ErrInvalidCSRF
	}
	var expiresAt int64
	err := s.conn.QueryRowContext(ctx, `
SELECT expires_at
FROM browser_sessions
WHERE session_hash = ?`,
		hashSecret(sessionID),
	).Scan(&expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrSessionNotFound
	}
	if err != nil {
		return err
	}
	if !now.Before(time.Unix(expiresAt, 0)) {
		return ErrSessionExpired
	}
	rows, err := s.conn.QueryContext(ctx, `
SELECT csrf_hash
FROM browser_session_csrf_tokens
WHERE session_hash = ?`,
		hashSecret(sessionID),
	)
	if err != nil {
		return err
	}
	defer rows.Close()
	candidate := hashSecret(csrfToken)
	valid := 0
	for rows.Next() {
		var storedHash []byte
		if err := rows.Scan(&storedHash); err != nil {
			return err
		}
		valid |= subtle.ConstantTimeCompare(storedHash, candidate)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if valid != 1 {
		return ErrInvalidCSRF
	}
	return nil
}

// IssueCSRF creates a reload-safe CSRF token while retaining a bounded set for
// other tabs using the same browser session.
func (s *Store) IssueCSRF(ctx context.Context, sessionID string, now time.Time) (string, error) {
	if sessionID == "" {
		return "", ErrSessionNotFound
	}
	token, err := randomSecret()
	if err != nil {
		return "", err
	}
	sessionHash := hashSecret(sessionID)
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	var expiresAt int64
	if err := tx.QueryRowContext(ctx, `
SELECT expires_at
FROM browser_sessions
WHERE session_hash = ?`,
		sessionHash,
	).Scan(&expiresAt); errors.Is(err, sql.ErrNoRows) {
		return "", ErrSessionNotFound
	} else if err != nil {
		return "", err
	}
	if !now.Before(time.Unix(expiresAt, 0)) {
		return "", ErrSessionExpired
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO browser_session_csrf_tokens (session_hash, csrf_hash, created_at)
VALUES (?, ?, ?)`,
		sessionHash,
		hashSecret(token),
		now.UTC().UnixNano(),
	); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM browser_session_csrf_tokens
WHERE session_hash = ?
  AND csrf_hash IN (
	SELECT csrf_hash
	FROM browser_session_csrf_tokens
	WHERE session_hash = ?
	ORDER BY created_at DESC, csrf_hash DESC
	LIMIT -1 OFFSET ?
  )`,
		sessionHash,
		sessionHash,
		MaxCSRFTokens,
	); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return token, nil
}

// PurgeExpired removes at most limit sessions and limit consumed-code records.
func (s *Store) PurgeExpired(ctx context.Context, now time.Time, limit int) (sessions, codes int64, err error) {
	if limit <= 0 {
		return 0, 0, errors.New("purge limit must be greater than zero")
	}
	sessions, err = deleteExpired(ctx, s.conn, "browser_sessions", now.UTC().Unix(), limit)
	if err != nil {
		return 0, 0, err
	}
	codes, err = deleteExpired(ctx, s.conn, "consumed_browser_codes", now.UTC().Unix(), limit)
	if err != nil {
		return sessions, 0, err
	}
	return sessions, codes, nil
}

func deleteExpired(ctx context.Context, conn *sql.DB, table string, now int64, limit int) (int64, error) {
	query := fmt.Sprintf(`
DELETE FROM %s
WHERE rowid IN (
	SELECT rowid FROM %s
	WHERE expires_at <= ?
	ORDER BY expires_at
	LIMIT ?
)`, table, table)
	result, err := conn.ExecContext(ctx, query, now, limit)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func randomSecret() (string, error) {
	value := make([]byte, opaqueSecretBytes)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate browser authentication secret: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func hashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
