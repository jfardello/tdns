package httpapi

import (
	"container/list"
	"crypto/sha256"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	passwordLimitCapacity      = 1024
	passwordLimitIdle          = 15 * time.Minute
	passwordSourceBurst        = 5
	passwordUsernameBurst      = 5
	passwordGlobalBurst        = 10
	passwordSourcePeriod       = 30 * time.Second
	passwordUsernamePeriod     = 30 * time.Second
	passwordGlobalPeriod       = time.Second
	passwordRetryAfter         = "30"
	invalidPasswordUsernameKey = "invalid"
)

type passwordLimitReason string

const (
	passwordLimitAllowed  passwordLimitReason = "allowed"
	passwordLimitGlobal   passwordLimitReason = "global"
	passwordLimitSource   passwordLimitReason = "source"
	passwordLimitUsername passwordLimitReason = "username"
)

type passwordLimitEntry[K comparable] struct {
	key      K
	limiter  *rate.Limiter
	lastSeen time.Time
	element  *list.Element
}

type passwordLimitBucket[K comparable] struct {
	entries map[K]*passwordLimitEntry[K]
	order   list.List
	period  time.Duration
	burst   int
}

type passwordLimiter struct {
	mu        sync.Mutex
	global    *rate.Limiter
	sources   passwordLimitBucket[netip.Addr]
	usernames passwordLimitBucket[[sha256.Size]byte]
	now       func() time.Time
}

func newPasswordLimiter() *passwordLimiter {
	return &passwordLimiter{
		global: rate.NewLimiter(rate.Every(passwordGlobalPeriod), passwordGlobalBurst),
		sources: passwordLimitBucket[netip.Addr]{
			entries: make(map[netip.Addr]*passwordLimitEntry[netip.Addr]),
			period:  passwordSourcePeriod,
			burst:   passwordSourceBurst,
		},
		usernames: passwordLimitBucket[[sha256.Size]byte]{
			entries: make(map[[sha256.Size]byte]*passwordLimitEntry[[sha256.Size]byte]),
			period:  passwordUsernamePeriod,
			burst:   passwordUsernameBurst,
		},
		now: time.Now,
	}
}

func (l *passwordLimiter) Allow(remoteAddress, normalizedUsername string) passwordLimitReason {
	address, ok := remoteIP(remoteAddress)
	if !ok {
		return passwordLimitSource
	}
	now := l.now()
	usernameKey := sha256.Sum256([]byte(normalizedUsername))

	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.global.AllowN(now, 1) {
		return passwordLimitGlobal
	}
	if !allowPasswordBucket(&l.sources, address, now) {
		return passwordLimitSource
	}
	if !allowPasswordBucket(&l.usernames, usernameKey, now) {
		return passwordLimitUsername
	}
	return passwordLimitAllowed
}

func allowPasswordBucket[K comparable](bucket *passwordLimitBucket[K], key K, now time.Time) bool {
	removeIdlePasswordEntries(bucket, now)
	entry, exists := bucket.entries[key]
	if !exists {
		if len(bucket.entries) >= passwordLimitCapacity {
			removeOldestPasswordEntry(bucket)
		}
		entry = &passwordLimitEntry[K]{
			key:      key,
			limiter:  rate.NewLimiter(rate.Every(bucket.period), bucket.burst),
			lastSeen: now,
		}
		entry.element = bucket.order.PushFront(entry)
		bucket.entries[key] = entry
	} else {
		entry.lastSeen = now
		bucket.order.MoveToFront(entry.element)
	}
	return entry.limiter.AllowN(now, 1)
}

func removeIdlePasswordEntries[K comparable](bucket *passwordLimitBucket[K], now time.Time) {
	for element := bucket.order.Back(); element != nil; {
		entry := element.Value.(*passwordLimitEntry[K])
		if now.Sub(entry.lastSeen) < passwordLimitIdle {
			return
		}
		previous := element.Prev()
		removePasswordEntry(bucket, entry)
		element = previous
	}
}

func removeOldestPasswordEntry[K comparable](bucket *passwordLimitBucket[K]) {
	if element := bucket.order.Back(); element != nil {
		removePasswordEntry(bucket, element.Value.(*passwordLimitEntry[K]))
	}
}

func removePasswordEntry[K comparable](bucket *passwordLimitBucket[K], entry *passwordLimitEntry[K]) {
	delete(bucket.entries, entry.key)
	bucket.order.Remove(entry.element)
}
