package auth

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
)

func TestIssueAndValidateBrowserCode(t *testing.T) {
	manager, err := NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 7, 29, 12, 0, 0, 0, time.UTC)
	manager.now = func() time.Time { return now }

	code, err := manager.IssueBrowserCode("operator", ScopeWrite, BrowserCodeTTL)
	if err != nil {
		t.Fatalf("IssueBrowserCode: %v", err)
	}
	principal, err := manager.ValidateBrowserCode(code)
	if err != nil {
		t.Fatalf("ValidateBrowserCode: %v", err)
	}
	if principal.Subject != "operator" ||
		principal.Scope != ScopeWrite ||
		principal.Purpose != BrowserCodePurpose ||
		principal.TokenID == "" ||
		principal.KeyID != "active" ||
		!principal.IssuedAt.Equal(now) ||
		!principal.ExpiresAt.Equal(now.Add(BrowserCodeTTL)) {
		t.Fatalf("principal = %#v", principal)
	}

	manager.now = func() time.Time { return now.Add(BrowserCodeTTL) }
	if _, err := manager.ValidateBrowserCode(code); err == nil {
		t.Fatal("browser login code accepted at its expiration boundary")
	}
}

func TestBrowserCodeContractIsSeparatedFromBearerTokens(t *testing.T) {
	manager, err := NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	browserCode, err := manager.IssueBrowserCode("operator", ScopeRead, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ValidateBearer(browserCode, ScopeRead); err == nil {
		t.Fatal("browser login code accepted as API bearer token")
	}

	bearer, err := manager.IssueBearer("operator", ScopeRead, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ValidateBrowserCode(bearer); err == nil {
		t.Fatal("API bearer token accepted as browser login code")
	}
}

func TestBrowserCodeRequiresPersistentKeyAndBoundedLifetime(t *testing.T) {
	manager, err := NewManager(config.AuthConf{}, "", Options{AllowEphemeral: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.IssueBrowserCode("operator", ScopeWrite, time.Minute); !errors.Is(err, ErrPersistentKeyRequired) {
		t.Fatalf("ephemeral issuance error = %v", err)
	}

	manager, err = NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	for _, lifetime := range []time.Duration{0, -time.Second, BrowserCodeTTL + time.Second} {
		if _, err := manager.IssueBrowserCode("operator", ScopeWrite, lifetime); err == nil {
			t.Fatalf("IssueBrowserCode accepted lifetime %s", lifetime)
		}
	}
}

func TestBrowserCodeRejectsWrongSigningKey(t *testing.T) {
	issuer, err := NewManager(managerConfig(), "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	conf := managerConfig()
	conf.ActiveKey.Value = encodeTestKey(testPreviousKey)
	validator, err := NewManager(conf, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	code, err := issuer.IssueBrowserCode("operator", ScopeRead, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := validator.ValidateBrowserCode(code); err == nil {
		t.Fatal("browser login code signed by the wrong key was accepted")
	}
}

func TestBrowserCodePreviousKeyHasBoundedAcceptance(t *testing.T) {
	conf := managerConfig()
	conf.PreviousKey = config.SigningKeyConf{
		ID:    "previous",
		Value: encodeTestKey(testPreviousKey),
	}
	conf.PreviousKeyAcceptUntil = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
	manager, err := NewManager(conf, "", Options{})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	manager.now = func() time.Time { return now }
	claims := Claims{
		Scope:   ScopeRead,
		Purpose: BrowserCodePurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    conf.Issuer,
			Subject:   "operator",
			Audience:  jwt.ClaimStrings{BrowserCodeAudience},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Minute)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        "previous-browser-code",
		},
	}
	derivedKey, err := browserCodeKey(testPreviousKey)
	if err != nil {
		t.Fatal(err)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	token.Header["kid"] = "previous"
	signed, err := token.SignedString(derivedKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.ValidateBrowserCode(signed); err != nil {
		t.Fatalf("previous key before cutoff: %v", err)
	}
	manager.now = func() time.Time { return manager.previousAccept }
	if _, err := manager.ValidateBrowserCode(signed); err == nil {
		t.Fatal("previous browser-code key accepted at its cutoff")
	}
}
