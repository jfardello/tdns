package cmd

import (
	"testing"
	"time"

	"github.com/jfardello/tdns/internal/auth"
)

func TestBrowserCodeLifetime(t *testing.T) {
	for _, test := range []struct {
		name      string
		lifetime  time.Duration
		wantError bool
	}{
		{name: "maximum", lifetime: auth.BrowserCodeTTL},
		{name: "shorter", lifetime: time.Minute},
		{name: "above maximum", lifetime: auth.BrowserCodeTTL + time.Second, wantError: true},
		{name: "zero", lifetime: 0, wantError: true},
		{name: "negative", lifetime: -time.Second, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := browserCodeLifetime(test.lifetime)
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError = %t", err, test.wantError)
			}
			if err == nil && got != test.lifetime {
				t.Fatalf("lifetime = %s, want %s", got, test.lifetime)
			}
		})
	}
}
