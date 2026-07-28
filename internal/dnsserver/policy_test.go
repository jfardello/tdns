package dnsserver

import (
	"fmt"
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/jfardello/tdns/config"
)

func testDNSAccessConf() config.DNSAccessConf {
	return config.DNSAccessConf{
		ClientQueriesPerSecond:   100,
		ClientBurst:              2,
		GlobalResponsesPerSecond: 1000,
		GlobalResponseBurst:      2,
		MaxConcurrentUpstreams:   2,
		MaxTrackedClients:        2,
		ClientIdleTimeout:        "10m",
	}
}

func TestPolicyLoopbackAndExplicitCIDRs(t *testing.T) {
	conf := testDNSAccessConf()
	conf.AllowedClientCIDRs = []string{"192.0.2.0/24", "2001:db8::/32"}
	policy, err := NewPolicy(conf)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}

	tests := []struct {
		address string
		allowed bool
	}{
		{address: "127.0.0.1", allowed: true},
		{address: "::1", allowed: true},
		{address: "::ffff:127.0.0.1", allowed: true},
		{address: "192.0.2.0", allowed: true},
		{address: "192.0.2.255", allowed: true},
		{address: "192.0.3.0", allowed: false},
		{address: "2001:db8:ffff::1", allowed: true},
		{address: "2001:db9::1", allowed: false},
		{address: "10.0.0.1", allowed: false},
		{address: "169.254.1.1", allowed: false},
		{address: "fe80::1", allowed: false},
	}
	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			if got := policy.isAllowed(netip.MustParseAddr(test.address)); got != test.allowed {
				t.Fatalf("isAllowed(%s) = %t, want %t", test.address, got, test.allowed)
			}
		})
	}
}

func TestPolicyRejectsMalformedConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*config.DNSAccessConf)
	}{
		{name: "malformed CIDR", mutate: func(c *config.DNSAccessConf) {
			c.AllowedClientCIDRs = []string{"192.0.2.1"}
		}},
		{name: "duplicate canonical CIDR", mutate: func(c *config.DNSAccessConf) {
			c.AllowedClientCIDRs = []string{"192.0.2.1/24", "192.0.2.0/24"}
		}},
		{name: "disabled client rate", mutate: func(c *config.DNSAccessConf) {
			c.ClientQueriesPerSecond = 0
		}},
		{name: "disabled response rate", mutate: func(c *config.DNSAccessConf) {
			c.GlobalResponsesPerSecond = 0
		}},
		{name: "disabled concurrency", mutate: func(c *config.DNSAccessConf) {
			c.MaxConcurrentUpstreams = 0
		}},
		{name: "invalid idle timeout", mutate: func(c *config.DNSAccessConf) {
			c.ClientIdleTimeout = "forever"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			conf := testDNSAccessConf()
			test.mutate(&conf)
			if _, err := NewPolicy(conf); err == nil {
				t.Fatal("NewPolicy succeeded, want validation error")
			}
		})
	}
}

func TestClientLimiterStateIsBoundedAndExpires(t *testing.T) {
	conf := testDNSAccessConf()
	conf.AllowedClientCIDRs = []string{"192.0.2.0/24"}
	conf.ClientIdleTimeout = "1s"
	policy, err := NewPolicy(conf)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	now := time.Unix(100, 0)

	for i := 1; i <= 20; i++ {
		addr := netip.MustParseAddr(fmt.Sprintf("192.0.2.%d", i))
		if !policy.clients.allow(addr, now) {
			t.Fatalf("new client %s was unexpectedly limited", addr)
		}
		if got := len(policy.clients.entries); got > conf.MaxTrackedClients {
			t.Fatalf("tracked clients = %d, maximum = %d", got, conf.MaxTrackedClients)
		}
	}

	now = now.Add(2 * time.Second)
	policy.clients.allow(netip.MustParseAddr("192.0.2.100"), now)
	if got := len(policy.clients.entries); got != 1 {
		t.Fatalf("tracked clients after idle expiry = %d, want 1", got)
	}
}

func TestClientLimiterStateRemainsBoundedUnderConcurrentSpoofedSources(t *testing.T) {
	conf := testDNSAccessConf()
	conf.MaxTrackedClients = 64
	policy, err := NewPolicy(conf)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	now := time.Unix(100, 0)

	var group sync.WaitGroup
	for i := 0; i < 1000; i++ {
		group.Add(1)
		go func(value int) {
			defer group.Done()
			addr := netip.AddrFrom4([4]byte{198, 51, byte(value >> 8), byte(value)})
			policy.clients.allow(addr, now)
		}(i)
	}
	group.Wait()

	policy.clients.mu.Lock()
	defer policy.clients.mu.Unlock()
	if got := len(policy.clients.entries); got != conf.MaxTrackedClients {
		t.Fatalf("tracked clients = %d, want bounded capacity %d", got, conf.MaxTrackedClients)
	}
}

func TestRemoteIPSupportsUDPAndTCPWithoutTrustingStrings(t *testing.T) {
	for _, remote := range []net.Addr{
		&net.UDPAddr{IP: net.ParseIP("192.0.2.1"), Port: 53},
		&net.TCPAddr{IP: net.ParseIP("2001:db8::1"), Port: 53},
	} {
		if _, ok := remoteIP(remote); !ok {
			t.Fatalf("remoteIP(%T) rejected a socket address", remote)
		}
	}
	if _, ok := remoteIP(stringAddr("127.0.0.1:53")); ok {
		t.Fatal("remoteIP trusted an arbitrary string address")
	}
}

type stringAddr string

func (a stringAddr) Network() string { return "test" }
func (a stringAddr) String() string  { return string(a) }
