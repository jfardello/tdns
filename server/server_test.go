package server

import (
	"context"
	"errors"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/resolver"
	"github.com/miekg/dns"
)

func TestConfigureDNSLogRejectsUnavailablePseudonymizationKey(t *testing.T) {
	dbPath, err := db.Bootstrap(context.Background(), filepath.Join(t.TempDir(), "tdns.sqlite"))
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}
	const environment = "TDNS_TEST_MISSING_DNS_LOG_KEY"
	t.Setenv(environment, "")
	s := &Server{
		Config: config.Config{
			Database: config.DatabaseConf{File: dbPath},
			DNSLog: config.DNSLogConf{
				Enabled: true,
				Pseudonymization: config.DNSLogPseudonymizationConf{
					Domains:        true,
					Clients:        true,
					KeyEnvironment: environment,
				},
			},
		},
		Middlewares: make(map[string]middleware.Middleware),
	}

	err = configureDNSLog(s)
	if err == nil || !strings.Contains(err.Error(), "pseudonymization environment variable") {
		t.Fatalf("configureDNSLog error = %v, want unavailable key error", err)
	}
	if _, exists := s.Middlewares["dns-log"]; exists {
		t.Fatal("failed DNS-log initialization registered the middleware")
	}
}

type orderedTestMiddleware struct {
	name  string
	stage middleware.Stage
}

type blockingExchanger struct {
	started chan struct{}
	release chan struct{}
}

type countingExchanger struct {
	calls int
}

func (c *countingExchanger) Exchange(message *dns.Msg, _ string) (*dns.Msg, time.Duration, error) {
	c.calls++
	response := new(dns.Msg)
	response.SetReply(message)
	return response, 0, nil
}

func (b *blockingExchanger) Exchange(message *dns.Msg, _ string) (*dns.Msg, time.Duration, error) {
	b.started <- struct{}{}
	<-b.release
	response := new(dns.Msg)
	response.SetReply(message)
	return response, 0, nil
}

func TestServerBoundsConcurrentUpstreamWork(t *testing.T) {
	exchanger := &blockingExchanger{
		started: make(chan struct{}, 1),
		release: make(chan struct{}),
	}
	s := &Server{
		Middlewares:   make(map[string]middleware.Middleware),
		upstreamSlots: make(chan struct{}, 1),
		defaultUpstream: resolver.Mux{Upstreams: []*resolver.Upstream{{
			Address: "127.0.0.1:53",
			Client:  exchanger,
		}}},
	}
	request := new(dns.Msg)
	request.SetQuestion("example.com.", dns.TypeA)
	firstDone := make(chan error, 1)
	go func() {
		_, err := s.Handler(request.Copy(), &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
		firstDone <- err
	}()

	select {
	case <-exchanger.started:
	case <-time.After(time.Second):
		t.Fatal("first upstream request did not start")
	}

	_, err := s.Handler(request.Copy(), &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if !errors.Is(err, ErrUpstreamSaturated) {
		t.Fatalf("second request error = %v, want ErrUpstreamSaturated", err)
	}

	close(exchanger.release)
	select {
	case err := <-firstDone:
		if err != nil {
			t.Fatalf("first request failed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first upstream request did not finish")
	}
	if got := len(s.upstreamSlots); got != 0 {
		t.Fatalf("upstream slots in use after completion = %d", got)
	}
}

func (o orderedTestMiddleware) Run(m *middleware.Message) (*middleware.Message, error) { return m, nil }
func (o orderedTestMiddleware) Info() (string, middleware.Stage)                       { return o.name, o.stage }
func (o orderedTestMiddleware) Config(config.Config) error                             { return nil }
func (o orderedTestMiddleware) Init() error                                            { return nil }

func TestGetIndexesOrdersPreRoutingDeterministically(t *testing.T) {
	s := &Server{
		Middlewares: map[string]middleware.Middleware{
			"cacheget":        orderedTestMiddleware{name: "cacheget", stage: middleware.PreRouting},
			"wildcard":        orderedTestMiddleware{name: "wildcard", stage: middleware.PreRouting},
			"zen-mode":        orderedTestMiddleware{name: "zen-mode", stage: middleware.PreRouting},
			"blacklist":       orderedTestMiddleware{name: "blacklist", stage: middleware.PreRouting},
			"tagger":          orderedTestMiddleware{name: "tagger", stage: middleware.PreRouting},
			"status":          orderedTestMiddleware{name: "status", stage: middleware.PreRouting},
			"static-response": orderedTestMiddleware{name: "static-response", stage: middleware.PreRouting},
		},
	}

	got := s.getIndexes()
	want := []string{"tagger", "status", "wildcard", "blacklist", "zen-mode", "static-response", "cacheget"}
	if len(got.preRouting) != len(want) {
		t.Fatalf("preRouting length got %d, want %d", len(got.preRouting), len(want))
	}
	for i := range want {
		if got.preRouting[i] != want[i] {
			t.Fatalf("preRouting[%d] got %q, want %q", i, got.preRouting[i], want[i])
		}
	}
}

func TestWildcardManagedFailureDoesNotReachUpstream(t *testing.T) {
	wildcard := &middleware.Wildcard{}
	if err := wildcard.Config(config.Config{Wildcard: config.WildcardConf{Enabled: true}}); err != nil {
		t.Fatalf("configure wildcard: %v", err)
	}
	exchanger := &countingExchanger{}
	s := &Server{
		Middlewares: map[string]middleware.Middleware{"wildcard": wildcard},
		defaultUpstream: resolver.Mux{Upstreams: []*resolver.Upstream{{
			Address: "upstream.invalid:53",
			Client:  exchanger,
		}}},
		upstreamSlots: make(chan struct{}, 1),
	}
	request := new(dns.Msg)
	request.SetQuestion("invalid.tdns.home.arpa.", dns.TypeA)
	response, err := s.Handler(request, &net.UDPAddr{IP: net.ParseIP("192.0.2.1")})
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if exchanger.calls != 0 {
		t.Fatalf("upstream calls = %d, want 0", exchanger.calls)
	}
	if response.Rcode != dns.RcodeNameError {
		t.Fatalf("rcode = %s, want NXDOMAIN", dns.RcodeToString[response.Rcode])
	}
}

func TestDisabledWildcardContinuesToUpstream(t *testing.T) {
	wildcard := &middleware.Wildcard{}
	if err := wildcard.Config(config.Config{}); err != nil {
		t.Fatalf("configure wildcard: %v", err)
	}
	exchanger := &countingExchanger{}
	s := &Server{
		Middlewares: map[string]middleware.Middleware{"wildcard": wildcard},
		defaultUpstream: resolver.Mux{Upstreams: []*resolver.Upstream{{
			Address: "upstream.invalid:53",
			Client:  exchanger,
		}}},
		upstreamSlots: make(chan struct{}, 1),
	}
	request := new(dns.Msg)
	request.SetQuestion("10-0-0-8.tdns.home.arpa.", dns.TypeA)
	if _, err := s.Handler(request, &net.UDPAddr{IP: net.ParseIP("192.0.2.1")}); err != nil {
		t.Fatalf("Handler: %v", err)
	}
	if exchanger.calls != 1 {
		t.Fatalf("upstream calls = %d, want 1", exchanger.calls)
	}
}
