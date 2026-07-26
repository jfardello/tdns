package httpapi

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jfardello/tdns/config"
)

func testSigningKey(t *testing.T) []byte {
	t.Helper()
	key := []byte("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	config.SetRunningConfig(&config.Config{Server: config.Server{
		SigningKey: base64.StdEncoding.EncodeToString(key),
	}})
	return key
}

func signTestToken(t *testing.T, method jwt.SigningMethod, claims jwt.MapClaims, key []byte) string {
	t.Helper()
	token, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return token
}

func TestValidateRequiresHS512AndExpiration(t *testing.T) {
	key := testSigningKey(t)
	validClaims := jwt.MapClaims{
		"scope": RWSCOPE,
		"sub":   "test",
		"exp":   time.Now().Add(time.Hour).Unix(),
	}

	valid := signTestToken(t, jwt.SigningMethodHS512, validClaims, key)
	if _, err := Validate(valid, RWSCOPE); err != nil {
		t.Fatalf("validate issued token: %v", err)
	}

	wrongAlgorithm := signTestToken(t, jwt.SigningMethodHS256, validClaims, key)
	if _, err := Validate(wrongAlgorithm, RWSCOPE); err == nil {
		t.Fatal("Validate accepted HS256 token")
	}

	withoutExpiry := signTestToken(t, jwt.SigningMethodHS512, jwt.MapClaims{
		"scope": RWSCOPE,
		"sub":   "test",
	}, key)
	if _, err := Validate(withoutExpiry, RWSCOPE); !errors.Is(err, jwt.ErrTokenRequiredClaimMissing) {
		t.Fatalf("missing expiration error = %v, want required claim error", err)
	}
}

func TestValidateRejectsNonStringScopeWithoutPanicking(t *testing.T) {
	key := testSigningKey(t)
	token := signTestToken(t, jwt.SigningMethodHS512, jwt.MapClaims{
		"scope": 42,
		"exp":   time.Now().Add(time.Hour).Unix(),
	}, key)

	if _, err := Validate(token, RWSCOPE); !errors.Is(err, jwt.ErrTokenInvalidClaims) {
		t.Fatalf("scope error = %v, want invalid claims", err)
	}
}

func TestRequireAcceptsOnlyBearerScheme(t *testing.T) {
	testSigningKey(t)
	token, err := IssueToken(1, "test")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	handler := Require(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}, Auth{IsRequired: true, Scope: RWSCOPE})

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
