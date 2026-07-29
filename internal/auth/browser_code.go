package auth

import (
	"crypto/hkdf"
	"crypto/sha512"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
)

const (
	BrowserCodeAudience = "tdns-browser-exchange"
	BrowserCodePurpose  = "browser-login"
	BrowserCodeTTL      = 2 * time.Minute
	BrowserSessionTTL   = 12 * time.Hour

	browserCodeKeyContext = "tdns/browser-code-signing/v1"
)

var ErrPersistentKeyRequired = errors.New("browser login codes require a persistent active signing key")

// IssueBrowserCode creates a short-lived credential that can only be consumed
// by the browser session exchange.
func (m *Manager) IssueBrowserCode(subject, scope string, lifetime time.Duration) (string, error) {
	if !m.persistent {
		return "", ErrPersistentKeyRequired
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", errors.New("browser login code subject must not be empty")
	}
	if scope != ScopeRead && scope != ScopeWrite {
		return "", fmt.Errorf("unsupported browser login code scope %q", scope)
	}
	if lifetime <= 0 {
		return "", errors.New("browser login code lifetime must be greater than zero")
	}
	if lifetime > BrowserCodeTTL {
		return "", fmt.Errorf("browser login code lifetime exceeds the %s maximum", BrowserCodeTTL)
	}

	identifier, err := randomIdentifier()
	if err != nil {
		return "", err
	}
	now := m.now()
	claims := Claims{
		Scope:   scope,
		Purpose: BrowserCodePurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   subject,
			Audience:  jwt.ClaimStrings{BrowserCodeAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(lifetime)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        identifier,
		},
	}
	key, err := browserCodeKey(m.active.value)
	if err != nil {
		return "", err
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	token.Header["kid"] = m.active.id
	signed, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign browser login code: %w", err)
	}
	log.GetLogger("auth", "audit").WithFields(logrus.Fields{
		"event":   "browser_code_issued",
		"key_id":  m.active.id,
		"outcome": "success",
		"scope":   scope,
		"subject": subject,
	}).Info("Authentication audit event.")
	return signed, nil
}

func (m *Manager) ValidateBrowserCode(tokenString string) (Principal, error) {
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
		return browserCodeKey(key)
	},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS512.Alg()}),
		jwt.WithAudience(BrowserCodeAudience),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
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
	if claims.Purpose != BrowserCodePurpose {
		return Principal{}, ErrInvalidPurpose
	}
	if claims.Scope != ScopeRead && claims.Scope != ScopeWrite {
		return Principal{}, jwt.ErrTokenInvalidClaims
	}
	if claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) > BrowserCodeTTL {
		return Principal{}, jwt.ErrTokenInvalidClaims
	}
	keyID, _ := token.Header["kid"].(string)
	return Principal{
		Subject:   claims.Subject,
		Scope:     claims.Scope,
		TokenID:   claims.ID,
		KeyID:     keyID,
		Purpose:   claims.Purpose,
		IssuedAt:  claims.IssuedAt.Time,
		ExpiresAt: claims.ExpiresAt.Time,
	}, nil
}

func browserCodeKey(signingKey []byte) ([]byte, error) {
	key, err := hkdf.Key(sha512.New, signingKey, nil, browserCodeKeyContext, sha512.Size)
	if err != nil {
		return nil, fmt.Errorf("derive browser login code key: %w", err)
	}
	return key, nil
}
