package server

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/resolver"
	"github.com/miekg/dns"
)

type orderedTestMiddleware struct {
	name  string
	stage middleware.Stage
}

type blockingExchanger struct {
	started chan struct{}
	release chan struct{}
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
			"zen-mode":        orderedTestMiddleware{name: "zen-mode", stage: middleware.PreRouting},
			"blacklist":       orderedTestMiddleware{name: "blacklist", stage: middleware.PreRouting},
			"tagger":          orderedTestMiddleware{name: "tagger", stage: middleware.PreRouting},
			"static-response": orderedTestMiddleware{name: "static-response", stage: middleware.PreRouting},
		},
	}

	got := s.getIndexes()
	want := []string{"tagger", "blacklist", "zen-mode", "static-response", "cacheget"}
	if len(got.preRouting) != len(want) {
		t.Fatalf("preRouting length got %d, want %d", len(got.preRouting), len(want))
	}
	for i := range want {
		if got.preRouting[i] != want[i] {
			t.Fatalf("preRouting[%d] got %q, want %q", i, got.preRouting[i], want[i])
		}
	}
}
