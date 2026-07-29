package auth

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
)

func encodeTestKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

var (
	testActiveKey   = []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	testPreviousKey = []byte("abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789")
)

func managerConfig() config.AuthConf {
	return config.AuthConf{
		Issuer:         "test-issuer",
		BearerAudience: "test-api",
		ActiveKey: config.SigningKeyConf{
			ID:    "active",
			Value: base64.StdEncoding.EncodeToString(testActiveKey),
		},
	}
}

func TestIssueAndValidateStrictBearerClaims(t *testing.T) {
	manager, err := NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	token, err := manager.IssueBearer("operator", ScopeRead, time.Hour)
	if err != nil {
		t.Fatalf("IssueBearer: %v", err)
	}
	principal, err := manager.ValidateBearer(token, ScopeRead)
	if err != nil {
		t.Fatalf("ValidateBearer: %v", err)
	}
	if principal.Subject != "operator" || principal.Scope != ScopeRead ||
		principal.KeyID != "active" || principal.TokenID == "" ||
		principal.Purpose != BearerPurpose {
		t.Fatalf("principal = %#v", principal)
	}
	if _, err := manager.ValidateBearer(token, ScopeWrite); !errors.Is(err, ErrInsufficientScope) {
		t.Fatalf("write authorization error = %v, want insufficient scope", err)
	}
}

func TestValidateRejectsWrongIssuerAudiencePurposeAndMissingClaims(t *testing.T) {
	manager, err := NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	now := time.Now()
	base := Claims{
		Scope:   ScopeWrite,
		Purpose: BearerPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "operator",
			Audience:  jwt.ClaimStrings{"test-api"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			NotBefore: jwt.NewNumericDate(now.Add(-time.Second)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Second)),
			ID:        "identifier",
		},
	}

	tests := []struct {
		name   string
		mutate func(*Claims)
	}{
		{name: "issuer", mutate: func(c *Claims) { c.Issuer = "other" }},
		{name: "audience", mutate: func(c *Claims) { c.Audience = jwt.ClaimStrings{"other"} }},
		{name: "purpose", mutate: func(c *Claims) { c.Purpose = "browser-code" }},
		{name: "issued at", mutate: func(c *Claims) { c.IssuedAt = nil }},
		{name: "not before", mutate: func(c *Claims) { c.NotBefore = nil }},
		{name: "identifier", mutate: func(c *Claims) { c.ID = "" }},
		{name: "subject", mutate: func(c *Claims) { c.Subject = "" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			claims := base
			test.mutate(&claims)
			token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
			token.Header["kid"] = "active"
			signed, err := token.SignedString(testActiveKey)
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			if _, err := manager.ValidateBearer(signed, ScopeRead); err == nil {
				t.Fatal("ValidateBearer accepted invalid claims")
			}
		})
	}
}

func TestValidateRejectsPreStrictToken(t *testing.T) {
	manager, err := NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
		"scope": ScopeWrite,
		"sub":   "operator",
		"exp":   time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString(testActiveKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := manager.ValidateBearer(signed, ScopeRead); err == nil {
		t.Fatal("ValidateBearer accepted a pre-strict token without a key identifier")
	}
}

func TestPreviousKeyHasBoundedAcceptance(t *testing.T) {
	conf := managerConfig()
	conf.PreviousKey = config.SigningKeyConf{
		ID:    "previous",
		Value: base64.StdEncoding.EncodeToString(testPreviousKey),
	}
	conf.PreviousKeyAcceptUntil = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	manager, err := NewManager(conf, "", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	now := time.Now()
	manager.now = func() time.Time { return now }
	claims := Claims{
		Scope:   ScopeWrite,
		Purpose: BearerPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    conf.Issuer,
			Subject:   "operator",
			Audience:  jwt.ClaimStrings{conf.BearerAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        "previous-token",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	token.Header["kid"] = "previous"
	signed, err := token.SignedString(testPreviousKey)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := manager.ValidateBearer(signed, ScopeRead); err != nil {
		t.Fatalf("previous key before cutoff: %v", err)
	}
	manager.now = func() time.Time { return manager.previousAccept }
	if _, err := manager.ValidateBearer(signed, ScopeRead); err == nil {
		t.Fatal("previous key accepted at cutoff")
	}
}

func TestSigningKeySourcePrecedenceAndValidation(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "active.key")
	if err := os.WriteFile(keyFile, []byte(base64.StdEncoding.EncodeToString(testPreviousKey)), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TDNS_TEST_ACTIVE_KEY", base64.StdEncoding.EncodeToString(testActiveKey))
	conf := managerConfig()
	conf.ActiveKey.Environment = "TDNS_TEST_ACTIVE_KEY"
	conf.ActiveKey.File = keyFile
	conf.ActiveKey.Value = base64.StdEncoding.EncodeToString(testPreviousKey)
	manager, err := NewManager(conf, "", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if string(manager.active.value) != string(testActiveKey) {
		t.Fatal("environment key did not take precedence")
	}

	fileConf := managerConfig()
	fileConf.ActiveKey.Value = ""
	fileConf.ActiveKey.File = keyFile
	if err := os.Chmod(keyFile, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(fileConf, "", Options{}); err != nil {
		t.Fatalf("NewManager rejected a group-readable restricted key: %v", err)
	}
	if err := os.Chmod(keyFile, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(fileConf, "", Options{}); err == nil {
		t.Fatal("NewManager accepted a key readable by other users")
	}

	conf.ActiveKey.ID = ""
	if _, err := NewManager(conf, "", Options{}); err == nil {
		t.Fatal("NewManager accepted a missing active key identifier")
	}
	conf = managerConfig()
	conf.ActiveKey.Value = base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := NewManager(conf, "", Options{}); err == nil {
		t.Fatal("NewManager accepted a short key")
	}
}

func TestPersistentKeyRequiredForOfflineIssuance(t *testing.T) {
	if _, err := NewManager(config.AuthConf{}, "", Options{}); err == nil {
		t.Fatal("NewManager accepted missing persistent key")
	}
	manager, err := NewManager(config.AuthConf{}, "", Options{AllowEphemeral: true})
	if err != nil {
		t.Fatalf("ephemeral manager: %v", err)
	}
	if manager.ActiveKeyID() != "ephemeral" {
		t.Fatalf("active key ID = %q", manager.ActiveKeyID())
	}
	if _, err := NewManager(config.AuthConf{
		ActiveKey: config.SigningKeyConf{ID: "configured-without-key"},
	}, "", Options{AllowEphemeral: true}); err == nil {
		t.Fatal("NewManager replaced an incomplete active key with an ephemeral key")
	}
}

func TestIssueBearerEnforcesNormalMaximum(t *testing.T) {
	manager, err := NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	lifetime := (MaximumTokenDays + 1) * 24 * time.Hour
	if _, err := manager.IssueBearer("operator", ScopeWrite, lifetime); err == nil {
		t.Fatal("IssueBearer accepted a lifetime above the normal maximum")
	}
	if _, err := manager.IssueLongLivedBearer("operator", ScopeWrite, lifetime); err != nil {
		t.Fatalf("IssueLongLivedBearer: %v", err)
	}
}
