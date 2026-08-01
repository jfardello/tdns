package httpapi

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestPasswordLimiterEnforcesSourceUsernameAndGlobalBudgets(t *testing.T) {
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)

	t.Run("source", func(t *testing.T) {
		limiter := newPasswordLimiter()
		limiter.now = func() time.Time { return now }
		for i := 0; i < passwordSourceBurst; i++ {
			if reason := limiter.Allow("192.0.2.1:1234", fmt.Sprintf("admin-%d", i)); reason != passwordLimitAllowed {
				t.Fatalf("attempt %d reason = %q", i+1, reason)
			}
		}
		if reason := limiter.Allow("192.0.2.1:1234", "another-admin"); reason != passwordLimitSource {
			t.Fatalf("source limit reason = %q", reason)
		}
	})

	t.Run("username", func(t *testing.T) {
		limiter := newPasswordLimiter()
		limiter.now = func() time.Time { return now }
		for i := 0; i < passwordUsernameBurst; i++ {
			if reason := limiter.Allow(fmt.Sprintf("192.0.2.%d:1234", i+1), "admin"); reason != passwordLimitAllowed {
				t.Fatalf("attempt %d reason = %q", i+1, reason)
			}
		}
		if reason := limiter.Allow("198.51.100.1:1234", "admin"); reason != passwordLimitUsername {
			t.Fatalf("username limit reason = %q", reason)
		}
	})

	t.Run("global", func(t *testing.T) {
		limiter := newPasswordLimiter()
		limiter.now = func() time.Time { return now }
		for i := 0; i < passwordGlobalBurst; i++ {
			if reason := limiter.Allow(fmt.Sprintf("192.0.2.%d:1234", i+1), fmt.Sprintf("admin-%d", i)); reason != passwordLimitAllowed {
				t.Fatalf("attempt %d reason = %q", i+1, reason)
			}
		}
		if reason := limiter.Allow("198.51.100.1:1234", "last-admin"); reason != passwordLimitGlobal {
			t.Fatalf("global limit reason = %q", reason)
		}
	})
}

func TestPasswordLimiterBoundsAndExpiresTrackedState(t *testing.T) {
	limiter := newPasswordLimiter()
	limiter.global = rate.NewLimiter(rate.Inf, passwordGlobalBurst)
	now := time.Date(2026, 8, 2, 10, 0, 0, 0, time.UTC)
	limiter.now = func() time.Time { return now }
	for i := 0; i < passwordLimitCapacity+100; i++ {
		address := fmt.Sprintf("10.%d.%d.%d:1234", (i>>16)&255, (i>>8)&255, i&255)
		_ = limiter.Allow(address, fmt.Sprintf("admin-%d", i))
	}
	if len(limiter.sources.entries) != passwordLimitCapacity || len(limiter.usernames.entries) != passwordLimitCapacity {
		t.Fatalf("tracked sources = %d, usernames = %d", len(limiter.sources.entries), len(limiter.usernames.entries))
	}
	now = now.Add(passwordLimitIdle)
	_ = limiter.Allow("198.51.100.1:1234", "admin")
	if len(limiter.sources.entries) != 1 || len(limiter.usernames.entries) != 1 {
		t.Fatalf("idle cleanup retained sources = %d, usernames = %d", len(limiter.sources.entries), len(limiter.usernames.entries))
	}
}

func TestPasswordLimiterRejectsMalformedPeerAddress(t *testing.T) {
	limiter := newPasswordLimiter()
	if reason := limiter.Allow("not-an-address", "admin"); reason != passwordLimitSource {
		t.Fatalf("reason = %q", reason)
	}
	if len(limiter.sources.entries) != 0 || len(limiter.usernames.entries) != 0 {
		t.Fatal("malformed source created limiter state")
	}
}

func TestPasswordLimiterSupportsConcurrentAttempts(t *testing.T) {
	limiter := newPasswordLimiter()
	limiter.global = rate.NewLimiter(rate.Inf, passwordGlobalBurst)
	const attempts = 100
	var wait sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_ = limiter.Allow(fmt.Sprintf("192.0.2.%d:1234", i+1), fmt.Sprintf("admin-%d", i))
		}()
	}
	wait.Wait()
	if len(limiter.sources.entries) != attempts || len(limiter.usernames.entries) != attempts {
		t.Fatalf("tracked sources = %d, usernames = %d", len(limiter.sources.entries), len(limiter.usernames.entries))
	}
}
