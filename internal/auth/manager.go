package auth

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
)

const (
	ScopeRead     = "tdns.kubewire.net:ro"
	ScopeWrite    = "tdns.kubewire.net:rw"
	BearerPurpose = "api-bearer"

	DefaultIssuer         = "tdns"
	DefaultBearerAudience = "tdns-management-api"
	DefaultTokenDays      = 30
	MaximumTokenDays      = 180

	minimumKeyBytes = 64
	clockSkew       = 30 * time.Second
)

var (
	ErrInsufficientScope = errors.New("insufficient scope")
	ErrInvalidPurpose    = errors.New("invalid token purpose")
	keyIDPattern         = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

type Options struct {
	AllowEphemeral bool
}

type Principal struct {
	Subject   string
	Scope     string
	TokenID   string
	KeyID     string
	Purpose   string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type Claims struct {
	Scope   string `json:"scope"`
	Purpose string `json:"purpose"`
	jwt.RegisteredClaims
}

type signingKey struct {
	id    string
	value []byte
}

type Manager struct {
	issuer           string
	bearerAudience   string
	active           signingKey
	verificationKeys map[string][]byte
	previousAccept   time.Time
	previousID       string
	persistent       bool
	now              func() time.Time
}

func NewManager(conf config.AuthConf, legacyInline string, options Options) (*Manager, error) {
	now := time.Now()
	issuer := strings.TrimSpace(conf.Issuer)
	if issuer == "" {
		issuer = DefaultIssuer
	}
	audience := strings.TrimSpace(conf.BearerAudience)
	if audience == "" {
		audience = DefaultBearerAudience
	}

	active, source, err := loadSigningKey(conf.ActiveKey, legacyInline)
	if err != nil {
		return nil, fmt.Errorf("load active signing key: %w", err)
	}
	if len(active.value) == 0 {
		if strings.TrimSpace(conf.ActiveKey.ID) != "" {
			return nil, errors.New("active signing key identifier is configured without key material")
		}
		if !options.AllowEphemeral {
			return nil, errors.New("a persistent active signing key is required")
		}
		active.value = make([]byte, minimumKeyBytes)
		if _, err := rand.Read(active.value); err != nil {
			return nil, fmt.Errorf("generate temporary active signing key: %w", err)
		}
		active.id = "ephemeral"
		source = "generated"
	}
	if err := validateSigningKey(active); err != nil {
		return nil, fmt.Errorf("active signing key: %w", err)
	}

	manager := &Manager{
		issuer:           issuer,
		bearerAudience:   audience,
		active:           active,
		verificationKeys: map[string][]byte{active.id: active.value},
		persistent:       source != "generated",
		now:              time.Now,
	}
	keyLogger := log.GetLogger("auth", "key-load").WithFields(logrus.Fields{
		"key_id": active.id,
		"source": source,
		"slot":   "active",
	})
	if source == "generated" {
		keyLogger.Warn("Generated a temporary signing key; offline tokens will not survive restart.")
	} else {
		keyLogger.Info("Loaded signing key.")
	}

	previousConfigured := signingKeyMaterialConfigured(conf.PreviousKey) ||
		strings.TrimSpace(conf.PreviousKey.ID) != "" ||
		strings.TrimSpace(conf.PreviousKeyAcceptUntil) != ""
	if !previousConfigured {
		return manager, nil
	}
	previous, source, err := loadSigningKey(conf.PreviousKey, "")
	if err != nil {
		return nil, fmt.Errorf("load previous signing key: %w", err)
	}
	if err := validateSigningKey(previous); err != nil {
		return nil, fmt.Errorf("previous signing key: %w", err)
	}
	if previous.id == active.id {
		return nil, fmt.Errorf("active and previous signing key identifiers must differ")
	}
	acceptUntil, err := time.Parse(time.RFC3339, conf.PreviousKeyAcceptUntil)
	if err != nil {
		return nil, fmt.Errorf("auth.previous_key_accept_until must be an RFC3339 timestamp: %w", err)
	}
	if !acceptUntil.After(now) {
		return nil, errors.New("auth.previous_key_accept_until must be in the future")
	}
	manager.previousID = previous.id
	manager.previousAccept = acceptUntil
	manager.verificationKeys[previous.id] = previous.value
	log.GetLogger("auth", "key-load").WithFields(logrus.Fields{
		"accept_until": acceptUntil.Format(time.RFC3339),
		"key_id":       previous.id,
		"source":       source,
		"slot":         "previous",
	}).Info("Loaded signing key.")
	return manager, nil
}

func (m *Manager) IssueBearer(subject, scope string, lifetime time.Duration) (string, error) {
	return m.issueBearer(subject, scope, lifetime, false)
}

func (m *Manager) IssueLongLivedBearer(subject, scope string, lifetime time.Duration) (string, error) {
	return m.issueBearer(subject, scope, lifetime, true)
}

func (m *Manager) issueBearer(subject, scope string, lifetime time.Duration, allowLongLived bool) (string, error) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", errors.New("token subject must not be empty")
	}
	if scope != ScopeRead && scope != ScopeWrite {
		return "", fmt.Errorf("unsupported token scope %q", scope)
	}
	if lifetime <= 0 {
		return "", errors.New("token lifetime must be greater than zero")
	}
	maximumLifetime := MaximumTokenDays * 24 * time.Hour
	if lifetime > maximumLifetime && !allowLongLived {
		return "", fmt.Errorf("token lifetime exceeds the %d-day maximum", MaximumTokenDays)
	}
	identifier, err := randomIdentifier()
	if err != nil {
		return "", err
	}
	now := m.now()
	claims := Claims{
		Scope:   scope,
		Purpose: BearerPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{m.bearerAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        identifier,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	token.Header["kid"] = m.active.id
	signed, err := token.SignedString(m.active.value)
	if err != nil {
		return "", fmt.Errorf("sign bearer token: %w", err)
	}
	log.GetLogger("auth", "audit").WithFields(logrus.Fields{
		"event":   "token_issued",
		"key_id":  m.active.id,
		"outcome": "success",
		"scope":   scope,
	}).Info("Authentication audit event.")
	return signed, nil
}

func (m *Manager) ValidateBearer(tokenString, requiredScope string) (Principal, error) {
	claims := new(Claims)
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		keyID, ok := token.Header["kid"].(string)
		if !ok || !keyIDPattern.MatchString(keyID) {
			return nil, jwt.ErrTokenUnverifiable
		}
		key, exists := m.verificationKeys[keyID]
		if !exists {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		if keyID == m.previousID && !m.now().Before(m.previousAccept) {
			return nil, jwt.ErrTokenExpired
		}
		return key, nil
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Alg()}),
		jwt.WithAudience(m.bearerAudience),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(clockSkew),
		jwt.WithTimeFunc(m.now),
	)
	if err != nil {
		return Principal{}, err
	}
	if !token.Valid ||
		claims.IssuedAt == nil ||
		claims.NotBefore == nil ||
		claims.ExpiresAt == nil ||
		strings.TrimSpace(claims.ID) == "" ||
		strings.TrimSpace(claims.Subject) == "" {
		return Principal{}, jwt.ErrTokenInvalidClaims
	}
	if claims.Purpose != BearerPurpose {
		return Principal{}, ErrInvalidPurpose
	}
	keyID, _ := token.Header["kid"].(string)
	principal := Principal{
		Subject:   claims.Subject,
		Scope:     claims.Scope,
		TokenID:   claims.ID,
		KeyID:     keyID,
		Purpose:   claims.Purpose,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}
	if !ScopeAllows(principal.Scope, requiredScope) {
		return principal, ErrInsufficientScope
	}
	return principal, nil
}

func ScopeAllows(granted, required string) bool {
	switch required {
	case ScopeRead:
		return granted == ScopeRead || granted == ScopeWrite
	case ScopeWrite:
		return granted == ScopeWrite
	default:
		return false
	}
}

func (m *Manager) ActiveKeyID() string {
	return m.active.id
}

func signingKeyMaterialConfigured(conf config.SigningKeyConf) bool {
	if environment := strings.TrimSpace(conf.Environment); environment != "" {
		if _, exists := os.LookupEnv(environment); exists {
			return true
		}
	}
	return strings.TrimSpace(conf.File) != "" || strings.TrimSpace(conf.Value) != ""
}

func loadSigningKey(conf config.SigningKeyConf, fallbackInline string) (signingKey, string, error) {
	key := signingKey{id: strings.TrimSpace(conf.ID)}
	encoded := ""
	source := ""

	if environment := strings.TrimSpace(conf.Environment); environment != "" {
		if value, exists := os.LookupEnv(environment); exists {
			encoded = strings.TrimSpace(value)
			source = "environment"
			if encoded == "" {
				return signingKey{}, "", fmt.Errorf("environment variable %s is empty", environment)
			}
		}
	}
	if encoded == "" && strings.TrimSpace(conf.File) != "" {
		info, err := os.Stat(conf.File)
		if err != nil {
			return signingKey{}, "", err
		}
		if !info.Mode().IsRegular() {
			return signingKey{}, "", fmt.Errorf("%s is not a regular file", conf.File)
		}
		if info.Mode().Perm()&0o037 != 0 {
			return signingKey{}, "", fmt.Errorf(
				"%s permissions %04o allow group write/execute or other access",
				conf.File,
				info.Mode().Perm(),
			)
		}
		value, err := os.ReadFile(conf.File)
		if err != nil {
			return signingKey{}, "", err
		}
		encoded = strings.TrimSpace(string(value))
		source = "file"
	}
	if encoded == "" {
		encoded = strings.TrimSpace(conf.Value)
		source = "inline"
	}
	if encoded == "" {
		encoded = strings.TrimSpace(fallbackInline)
		source = "legacy_inline"
	}
	if encoded == "" {
		return key, "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return signingKey{}, "", fmt.Errorf("decode base64 key: %w", err)
	}
	key.value = decoded
	return key, source, nil
}

func validateSigningKey(key signingKey) error {
	if !keyIDPattern.MatchString(key.id) {
		return errors.New("key identifier must contain 1-64 letters, digits, dots, underscores, or hyphens")
	}
	if len(key.value) < minimumKeyBytes {
		return fmt.Errorf("key contains %d decoded bytes, minimum is %d", len(key.value), minimumKeyBytes)
	}
	return nil
}

func randomIdentifier() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate token identifier: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}
