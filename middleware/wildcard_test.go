package middleware

import (
	"context"
	"net/netip"
	"slices"
	"testing"

	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

func TestParseWildcardName(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		wantAddress string
		wantManaged bool
		wantError   bool
	}{
		{name: "IPv4 dots", query: "app.192.168.1.20.tdns.home.arpa.", wantAddress: "192.168.1.20", wantManaged: true},
		{name: "IPv4 dots with labels", query: "customer.app.10.0.0.5.tdns.home.arpa", wantAddress: "10.0.0.5", wantManaged: true},
		{name: "IPv4 dashes", query: "app-192-168-1-20.tdns.home.arpa", wantAddress: "192.168.1.20", wantManaged: true},
		{name: "IPv4 hexadecimal", query: "app-c0a80114.tdns.home.arpa", wantAddress: "192.168.1.20", wantManaged: true},
		{name: "IPv6 compressed", query: "--1.tdns.home.arpa", wantAddress: "::1", wantManaged: true},
		{name: "IPv6 ULA", query: "service.fd00--20.tdns.home.arpa", wantAddress: "fd00::20", wantManaged: true},
		{name: "enabled extra domain", query: "10-0-0-8.nip.io", wantAddress: "10.0.0.8", wantManaged: true},
		{name: "suffix boundary", query: "10-0-0-8.evilnip.io", wantManaged: false},
		{name: "unmanaged", query: "example.com", wantManaged: false},
		{name: "zone apex", query: "tdns.home.arpa", wantManaged: true, wantError: true},
		{name: "bad octet", query: "app.192.168.1.999.tdns.home.arpa", wantManaged: true, wantError: true},
		{name: "bad hexadecimal", query: "app-c0a80z14.tdns.home.arpa", wantManaged: true, wantError: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			address, managed, err := parseWildcardName(test.query, []string{"tdns.home.arpa", "nip.io"})
			if managed != test.wantManaged {
				t.Fatalf("managed = %v, want %v", managed, test.wantManaged)
			}
			if (err != nil) != test.wantError {
				t.Fatalf("error = %v, wantError %v", err, test.wantError)
			}
			if test.wantAddress != "" && address != netip.MustParseAddr(test.wantAddress) {
				t.Fatalf("address = %s, want %s", address, test.wantAddress)
			}
		})
	}
}

func TestWildcardConfigUsesSecureDefaultsAndValidatesExtraDomains(t *testing.T) {
	wildcard := &Wildcard{}
	if err := wildcard.Config(config.Config{}); err != nil {
		t.Fatalf("Config defaults: %v", err)
	}
	status := wildcard.Status()
	if status.Enabled || status.PrimaryDomain != DefaultWildcardDomain || status.TTL != DefaultWildcardTTL || status.AllowPublicAddresses {
		t.Fatalf("default status = %#v", status)
	}

	err := wildcard.Config(config.Config{Wildcard: config.WildcardConf{
		AvailableExtraDomains: []string{"nip.io"},
		EnabledExtraDomains:   []string{"xip.io"},
	}})
	if err == nil {
		t.Fatal("Config accepted an enabled domain outside available_extra_domains")
	}
}

func TestWildcardReplaceEnabledExtraDomainsIsAtomic(t *testing.T) {
	wildcard := &Wildcard{}
	if err := wildcard.Config(config.Config{Wildcard: config.WildcardConf{
		AvailableExtraDomains: []string{"nip.io", "xip.io"},
		EnabledExtraDomains:   []string{"nip.io"},
	}}); err != nil {
		t.Fatalf("Config: %v", err)
	}

	if err := wildcard.ReplaceEnabledExtraDomains([]string{"XIP.IO.", "xip.io"}); err != nil {
		t.Fatalf("ReplaceEnabledExtraDomains: %v", err)
	}
	if got := wildcard.Status().EnabledExtraDomains; !slices.Equal(got, []string{"xip.io"}) {
		t.Fatalf("enabled domains = %#v", got)
	}
	if err := wildcard.ReplaceEnabledExtraDomains([]string{"example.com"}); err == nil {
		t.Fatal("ReplaceEnabledExtraDomains accepted a domain outside the allowlist")
	}
	if got := wildcard.Status().EnabledExtraDomains; !slices.Equal(got, []string{"xip.io"}) {
		t.Fatalf("failed replacement changed enabled domains to %#v", got)
	}
}

func TestWildcardRun(t *testing.T) {
	tests := []struct {
		name         string
		enabled      bool
		allowPublic  bool
		query        string
		queryType    uint16
		wantResolved bool
		wantRcode    int
		wantAnswer   string
		wantContext  bool
	}{
		{name: "disabled", query: "192.168.1.20.tdns.home.arpa.", queryType: dns.TypeA},
		{name: "private IPv4", enabled: true, query: "app-192-168-1-20.tdns.home.arpa.", queryType: dns.TypeA, wantResolved: true, wantRcode: dns.RcodeSuccess, wantAnswer: "192.168.1.20", wantContext: true},
		{name: "IPv6 loopback", enabled: true, query: "--1.tdns.home.arpa.", queryType: dns.TypeAAAA, wantResolved: true, wantRcode: dns.RcodeSuccess, wantAnswer: "::1", wantContext: true},
		{name: "IPv6 link-local", enabled: true, query: "fe80--1.tdns.home.arpa.", queryType: dns.TypeAAAA, wantResolved: true, wantRcode: dns.RcodeSuccess, wantAnswer: "fe80::1", wantContext: true},
		{name: "family mismatch", enabled: true, query: "10-0-0-8.tdns.home.arpa.", queryType: dns.TypeAAAA, wantResolved: true, wantRcode: dns.RcodeSuccess, wantContext: true},
		{name: "public rejected", enabled: true, query: "8-8-8-8.tdns.home.arpa.", queryType: dns.TypeA, wantResolved: true, wantRcode: dns.RcodeNameError},
		{name: "multicast rejected", enabled: true, query: "224-0-0-1.tdns.home.arpa.", queryType: dns.TypeA, wantResolved: true, wantRcode: dns.RcodeNameError},
		{name: "unspecified rejected", enabled: true, query: "0-0-0-0.tdns.home.arpa.", queryType: dns.TypeA, wantResolved: true, wantRcode: dns.RcodeNameError},
		{name: "public IPv6 rejected", enabled: true, query: "2001-db8--1.tdns.home.arpa.", queryType: dns.TypeAAAA, wantResolved: true, wantRcode: dns.RcodeNameError},
		{name: "public explicitly allowed", enabled: true, allowPublic: true, query: "8-8-8-8.tdns.home.arpa.", queryType: dns.TypeA, wantResolved: true, wantRcode: dns.RcodeSuccess, wantAnswer: "8.8.8.8", wantContext: true},
		{name: "malformed managed name", enabled: true, query: "invalid.tdns.home.arpa.", queryType: dns.TypeA, wantResolved: true, wantRcode: dns.RcodeNameError},
		{name: "unmanaged name", enabled: true, query: "example.com.", queryType: dns.TypeA},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wildcard := &Wildcard{}
			err := wildcard.Config(config.Config{Wildcard: config.WildcardConf{
				Enabled:              test.enabled,
				AllowPublicAddresses: test.allowPublic,
			}})
			if err != nil {
				t.Fatalf("Config: %v", err)
			}
			request := new(dns.Msg)
			request.SetQuestion(test.query, test.queryType)
			message := &Message{}
			message.SetMsg(request)
			message.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{}))

			result, err := wildcard.Run(message)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if result.IsResolved() != test.wantResolved {
				t.Fatalf("resolved = %v, want %v", result.IsResolved(), test.wantResolved)
			}
			if !test.wantResolved {
				return
			}
			response := result.Answer()
			if response.Rcode != test.wantRcode {
				t.Fatalf("rcode = %s, want %s", dns.RcodeToString[response.Rcode], dns.RcodeToString[test.wantRcode])
			}
			if len(response.Answer) == 0 {
				if test.wantAnswer != "" {
					t.Fatalf("answer is empty, want %s", test.wantAnswer)
				}
			} else if !containsRRAddress(response.Answer[0], test.wantAnswer) {
				t.Fatalf("answer = %s, want address %s", response.Answer[0], test.wantAnswer)
			}
			_, hasContext := result.GetValue("tdns/wildcard")
			if hasContext != test.wantContext {
				t.Fatalf("wildcard context = %v, want %v", hasContext, test.wantContext)
			}
		})
	}
}

func TestWildcardRunUsesEnabledExtraDomain(t *testing.T) {
	wildcard := &Wildcard{}
	if err := wildcard.Config(config.Config{Wildcard: config.WildcardConf{
		Enabled:               true,
		AvailableExtraDomains: []string{"nip.io"},
		EnabledExtraDomains:   []string{"NIP.IO."},
	}}); err != nil {
		t.Fatalf("Config: %v", err)
	}
	request := new(dns.Msg)
	request.SetQuestion("app-10-0-0-8.nip.io.", dns.TypeA)
	message := &Message{}
	message.SetMsg(request)
	message.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{}))

	result, err := wildcard.Run(message)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !result.IsResolved() || len(result.Answer().Answer) != 1 || !containsRRAddress(result.Answer().Answer[0], "10.0.0.8") {
		t.Fatalf("unexpected result: %#v", result.Answer())
	}
}

func TestWildcardRunRejectsMessageWithoutDNSPayload(t *testing.T) {
	wildcard := &Wildcard{}
	if err := wildcard.Config(config.Config{Wildcard: config.WildcardConf{Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	_, err := wildcard.Run(&Message{})
	if err == nil || err.Error() != "no msg" {
		t.Fatalf("Run error = %v, want no msg", err)
	}
}

func containsRRAddress(record dns.RR, address string) bool {
	switch value := record.(type) {
	case *dns.A:
		return value.A.String() == address
	case *dns.AAAA:
		return value.AAAA.String() == address
	default:
		return false
	}
}
