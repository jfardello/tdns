package cmd

import (
	"testing"
	"time"

	"github.com/jfardello/tdns/internal/auth"
)

func TestTokenLifetimePolicy(t *testing.T) {
	for _, test := range []struct {
		name      string
		days      int
		override  bool
		want      time.Duration
		wantError bool
	}{
		{name: "default", days: auth.DefaultTokenDays, want: 30 * 24 * time.Hour},
		{name: "maximum", days: auth.MaximumTokenDays, want: 180 * 24 * time.Hour},
		{name: "above maximum", days: auth.MaximumTokenDays + 1, wantError: true},
		{name: "explicit override", days: auth.MaximumTokenDays + 1, override: true, want: 181 * 24 * time.Hour},
		{name: "zero", days: 0, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := tokenLifetime(test.days, test.override)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError = %t", err, test.wantError)
			}
			if got != test.want {
				t.Fatalf("lifetime = %s, want %s", got, test.want)
			}
		})
	}
}

func TestParseTokenScope(t *testing.T) {
	for input, want := range map[string]string{
		"read-only":  auth.ScopeRead,
		"ro":         auth.ScopeRead,
		"read-write": auth.ScopeWrite,
		"rw":         auth.ScopeWrite,
	} {
		got, err := parseTokenScope(input)
		if err != nil {
			t.Fatalf("parseTokenScope(%q): %v", input, err)
		}
		if got != want {
			t.Fatalf("parseTokenScope(%q) = %q, want %q", input, got, want)
		}
	}
	if _, err := parseTokenScope("admin"); err == nil {
		t.Fatal("parseTokenScope accepted an unsupported scope")
	}
}
