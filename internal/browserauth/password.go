package browserauth

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jfardello/tdns/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

const (
	AdministratorBcryptCost    = 12
	AdministratorPasswordMin   = 12
	AdministratorPasswordMax   = 72
	administratorSingleton     = 1
	administratorUsernameLimit = 64
	dummyAdministratorHash     = "$2a$12$tSZ/zF0Vvlr6eDDWloupTOgqFCLLeMnCF/a2iu8Whzvyx7PtQD3hG"
	dummyAdministratorPassword = "tdns-dummy-password-comparison"
)

var (
	ErrAdministratorUnavailable = errors.New("local administrator credential is unavailable")
	ErrInvalidAdministratorName = errors.New("invalid local administrator username")
	ErrInvalidPassword          = errors.New("password must contain between 12 and 72 UTF-8 bytes")
	ErrInvalidCredentials       = errors.New("invalid administrator credentials")
)

type AdministratorCredential struct {
	Username     string
	PasswordHash []byte
	Subject      string
	Scope        string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func NormalizeAdministratorUsername(username string) (string, error) {
	username = strings.ToLower(strings.TrimSpace(username))
	if len(username) == 0 || len(username) > administratorUsernameLimit {
		return "", ErrInvalidAdministratorName
	}
	for i, char := range username {
		valid := char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '.' || char == '_' || char == '-'
		if !valid || (i == 0 && (char == '.' || char == '_' || char == '-')) {
			return "", ErrInvalidAdministratorName
		}
	}
	last := username[len(username)-1]
	if last == '.' || last == '_' || last == '-' {
		return "", ErrInvalidAdministratorName
	}
	return username, nil
}

func ValidateAdministratorPassword(password []byte) error {
	if !utf8.Valid(password) || len(password) < AdministratorPasswordMin || len(password) > AdministratorPasswordMax {
		return ErrInvalidPassword
	}
	return nil
}

func compareAdministratorPassword(hash, password []byte) error {
	return bcrypt.CompareHashAndPassword(hash, password)
}

func (s *Store) SetAdministratorPassword(ctx context.Context, username string, password []byte, now time.Time) error {
	normalized, err := NormalizeAdministratorUsername(username)
	if err != nil {
		return err
	}
	if err := ValidateAdministratorPassword(password); err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword(password, AdministratorBcryptCost)
	if err != nil {
		return fmt.Errorf("hash local administrator password: %w", err)
	}
	now = now.UTC().Truncate(time.Second)

	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `
INSERT INTO local_administrator
	(singleton, username, password_hash, subject, scope, enabled, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, ?, ?)
ON CONFLICT(singleton) DO UPDATE SET
	username = excluded.username,
	password_hash = excluded.password_hash,
	subject = excluded.subject,
	scope = excluded.scope,
	enabled = 1,
	updated_at = excluded.updated_at`,
		administratorSingleton, normalized, hash, normalized, auth.ScopeWrite, now.Unix(), now.Unix(),
	); err != nil {
		return fmt.Errorf("store local administrator credential: %w", err)
	}
	if err := revokePasswordSessions(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) DisableAdministrator(ctx context.Context, now time.Time) error {
	now = now.UTC().Truncate(time.Second)
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx, `
UPDATE local_administrator
SET enabled = 0, updated_at = ?
WHERE singleton = ?`, now.Unix(), administratorSingleton)
	if err != nil {
		return fmt.Errorf("disable local administrator credential: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrAdministratorUnavailable
	}
	if err := revokePasswordSessions(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Administrator(ctx context.Context) (AdministratorCredential, error) {
	return administratorCredential(ctx, s.conn)
}

type administratorQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func administratorCredential(ctx context.Context, querier administratorQuerier) (AdministratorCredential, error) {
	var credential AdministratorCredential
	var enabled int
	var createdAt, updatedAt int64
	err := querier.QueryRowContext(ctx, `
SELECT username, password_hash, subject, scope, enabled, created_at, updated_at
FROM local_administrator
WHERE singleton = ?`, administratorSingleton).Scan(
		&credential.Username,
		&credential.PasswordHash,
		&credential.Subject,
		&credential.Scope,
		&enabled,
		&createdAt,
		&updatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return AdministratorCredential{}, ErrAdministratorUnavailable
	}
	if err != nil {
		return AdministratorCredential{}, err
	}
	normalized, err := NormalizeAdministratorUsername(credential.Username)
	if err != nil || normalized != credential.Username || enabled != 1 || credential.Subject != credential.Username || credential.Scope != auth.ScopeWrite {
		return AdministratorCredential{}, ErrAdministratorUnavailable
	}
	cost, err := bcrypt.Cost(credential.PasswordHash)
	if err != nil || cost != AdministratorBcryptCost {
		return AdministratorCredential{}, ErrAdministratorUnavailable
	}
	credential.CreatedAt = time.Unix(createdAt, 0).UTC()
	credential.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return credential, nil
}

// CreatePasswordSession verifies the singleton administrator and creates its
// opaque browser session. Credential state is revalidated in the transaction
// so a concurrent password rotation cannot leave a post-rotation session.
func (s *Store) CreatePasswordSession(ctx context.Context, username string, password []byte, now time.Time) (Credentials, error) {
	normalized, normalizeErr := NormalizeAdministratorUsername(username)
	credential, credentialErr := s.Administrator(ctx)
	passwordErr := ValidateAdministratorPassword(password)

	comparisonHash := []byte(dummyAdministratorHash)
	comparisonPassword := password
	validCandidate := normalizeErr == nil &&
		credentialErr == nil &&
		normalized == credential.Username &&
		passwordErr == nil
	if validCandidate {
		comparisonHash = credential.PasswordHash
	} else if passwordErr != nil {
		comparisonPassword = []byte(dummyAdministratorPassword)
	}
	comparisonErr := s.comparePassword(comparisonHash, comparisonPassword)
	if credentialErr != nil && !errors.Is(credentialErr, ErrAdministratorUnavailable) {
		return Credentials{}, credentialErr
	}
	if !validCandidate || comparisonErr != nil {
		return Credentials{}, ErrInvalidCredentials
	}

	credentials, err := newSessionCredentials(
		credential.Subject,
		credential.Scope,
		AuthenticationMethodPassword,
		now,
	)
	if err != nil {
		return Credentials{}, err
	}
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return Credentials{}, err
	}
	defer func() { _ = tx.Rollback() }()

	current, err := administratorCredential(ctx, tx)
	if err != nil || current.Username != credential.Username ||
		subtle.ConstantTimeCompare(current.PasswordHash, credential.PasswordHash) != 1 {
		if err != nil && !errors.Is(err, ErrAdministratorUnavailable) {
			return Credentials{}, err
		}
		return Credentials{}, ErrInvalidCredentials
	}
	if err := insertSession(ctx, tx, credentials, now); err != nil {
		return Credentials{}, err
	}
	if err := tx.Commit(); err != nil {
		return Credentials{}, err
	}
	return credentials, nil
}

type passwordSessionRevoker interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func revokePasswordSessions(ctx context.Context, executor passwordSessionRevoker) error {
	if _, err := executor.ExecContext(ctx, `
DELETE FROM browser_sessions
WHERE authentication_method = ?`, AuthenticationMethodPassword); err != nil {
		return fmt.Errorf("revoke password-authenticated sessions: %w", err)
	}
	return nil
}
