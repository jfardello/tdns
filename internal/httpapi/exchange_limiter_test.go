package httpapi

import (
	"fmt"
	"testing"
	"time"
)

func TestExchangeLimiterPerClientAndBoundedState(t *testing.T) {
	limiter := newExchangeLimiter()
	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	for i := 0; i < exchangeClientBurst; i++ {
		if !limiter.Allow("192.0.2.1:1234") {
			t.Fatalf("request %d unexpectedly limited", i+1)
		}
	}
	if limiter.Allow("192.0.2.1:1234") {
		t.Fatal("per-client burst was not enforced")
	}

	for i := 0; i < exchangeClientLimit+100; i++ {
		address := fmt.Sprintf("10.%d.%d.%d:1234", (i>>16)&255, (i>>8)&255, i&255)
		_ = limiter.Allow(address)
	}
	if len(limiter.clients) != exchangeClientLimit {
		t.Fatalf("tracked clients = %d, want %d", len(limiter.clients), exchangeClientLimit)
	}
	now = now.Add(exchangeClientIdle)
	_ = limiter.Allow("198.51.100.1:1234")
	if len(limiter.clients) != 1 {
		t.Fatalf("idle cleanup retained %d clients", len(limiter.clients))
	}
}

func TestRemoteIPIgnoresForwardingHeadersByConstruction(t *testing.T) {
	address, ok := remoteIP("[::ffff:192.0.2.1]:8443")
	if !ok || address.String() != "192.0.2.1" {
		t.Fatalf("remoteIP = %v, %t", address, ok)
	}
	if _, ok := remoteIP("not-an-address"); ok {
		t.Fatal("remoteIP accepted malformed input")
	}
}

func TestExchangeLimiterGlobalBurst(t *testing.T) {
	limiter := newExchangeLimiter()
	now := time.Date(2026, 7, 30, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	for i := 0; i < exchangeGlobalBurst; i++ {
		if !limiter.Allow(fmt.Sprintf("192.0.2.%d:1234", i+1)) {
			t.Fatalf("global request %d unexpectedly limited", i+1)
		}
	}
	if limiter.Allow("198.51.100.1:1234") {
		t.Fatal("global burst was not enforced")
	}
}
