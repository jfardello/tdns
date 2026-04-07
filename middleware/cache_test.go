package middleware

import (
	"context"
	"net"
	"testing"

	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

func TestCacheKeyIncludesLabels(t *testing.T) {
	cache := &Cache{}
	question := dns.Question{Name: "example.org.", Qtype: dns.TypeA, Qclass: dns.ClassINET}

	keyA := cache.Key(&question, []string{"kids"})
	keyB := cache.Key(&question, []string{"work"})
	keyC := cache.Key(&question, []string{"work", "kids"})
	keyD := cache.Key(&question, []string{"kids", "work"})

	if keyA == keyB {
		t.Fatalf("expected different labels to produce different keys, got %q", keyA)
	}
	if keyC != keyD {
		t.Fatalf("expected key to be stable regardless of label order, got %q and %q", keyC, keyD)
	}
}

func TestCacheShouldBypassForSelectors(t *testing.T) {
	cache := &Cache{}
	cache.ReplaceExcludes([]string{"label:kids", "ip:192.168.1.50", "cidr:10.0.0.0/24"})

	message := &Message{}
	message.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("192.168.1.50")},
		Labels:     []string{"guest"},
		Values:     map[string]string{},
	}))

	if !cache.ShouldBypass(message) {
		t.Fatal("expected ip selector to bypass cache")
	}

	message.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("172.16.0.20")},
		Labels:     []string{"kids"},
		Values:     map[string]string{},
	}))
	if !cache.ShouldBypass(message) {
		t.Fatal("expected label selector to bypass cache")
	}
}
