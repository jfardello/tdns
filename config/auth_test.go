package config

import (
	"strings"
	"testing"
)

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

func TestParseDNSLogRetention(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		valid bool
	}{
		{name: "one hour", value: "1h", valid: true},
		{name: "default", value: DefaultDNSLogRetention, valid: true},
		{name: "maximum", value: "180d", valid: true},
		{name: "empty"},
		{name: "zero", value: "0s"},
		{name: "negative", value: "-1h"},
		{name: "above maximum", value: "181d"},
		{name: "invalid", value: "forever"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParseDNSLogRetention(test.value)
			if test.valid && err != nil {
				t.Fatalf("ParseDNSLogRetention: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("ParseDNSLogRetention accepted an invalid duration")
			}
		})
	}
}

func TestValidateDNSLogPseudonymizationKeySource(t *testing.T) {
	base := Config{Auth: AuthConf{Browser: BrowserAuthConf{RememberDays: DefaultBrowserRememberDays}}}
	base.DNSLog.Pseudonymization.Domains = true
	if err := Validate(&base); err == nil {
		t.Fatal("Validate accepted pseudonymization without a key source")
	}

	base.DNSLog.Pseudonymization.KeyEnvironment = "TDNS_DNS_LOG_PSEUDONYMIZATION_KEY"
	if err := Validate(&base); err != nil {
		t.Fatalf("Validate rejected environment key source: %v", err)
	}

	base.DNSLog.Pseudonymization.KeyEnvironment = "KEY=value"
	if err := Validate(&base); err == nil {
		t.Fatal("Validate accepted an environment assignment instead of a variable name")
	}
}

func TestValidateDiagnosticsAddress(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "127.0.0.1:6060", valid: true},
		{value: "[::1]:6060", valid: true},
		{value: "192.168.1.2:9090", valid: true},
		{value: ":6060"},
		{value: "0.0.0.0:6060"},
		{value: "[::]:6060"},
		{value: "224.0.0.1:6060"},
		{value: "localhost:6060"},
		{value: "127.0.0.1:0"},
		{value: "127.0.0.1"},
	} {
		t.Run(strings.ReplaceAll(test.value, "/", "_"), func(t *testing.T) {
			err := ValidateDiagnosticsAddress(test.value)
			if test.valid && err != nil {
				t.Fatalf("ValidateDiagnosticsAddress: %v", err)
			}
			if !test.valid && err == nil {
				t.Fatal("ValidateDiagnosticsAddress accepted an unsafe address")
			}
		})
	}
}
