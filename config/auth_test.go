package config

import "testing"

func TestValidateBrowserRememberDays(t *testing.T) {
	for _, test := range []struct {
		name  string
		days  int
		valid bool
	}{
		{name: "minimum", days: MinBrowserRememberDays, valid: true},
		{name: "default", days: DefaultBrowserRememberDays, valid: true},
		{name: "maximum", days: MaxBrowserRememberDays, valid: true},
		{name: "zero", days: 0},
		{name: "negative", days: -1},
		{name: "above maximum", days: MaxBrowserRememberDays + 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			conf := &Config{Auth: AuthConf{Browser: BrowserAuthConf{RememberDays: test.days}}}
			err := Validate(conf)
			if test.valid && err != nil {
				t.Fatalf("Validate: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("Validate accepted an invalid remembered-session lifetime")
			}
		})
	}
}
