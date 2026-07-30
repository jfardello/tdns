package httpapi

import (
	"container/list"
	"net"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	exchangeClientLimit  = 1024
	exchangeClientIdle   = 10 * time.Minute
	exchangeClientBurst  = 3
	exchangeGlobalBurst  = 10
	exchangeClientPeriod = 10 * time.Second
	exchangeGlobalPeriod = time.Second
)

type exchangeLimitEntry struct {
	address  netip.Addr
	limiter  *rate.Limiter
	lastSeen time.Time
	element  *list.Element
}

type exchangeLimiter struct {
	mu      sync.Mutex
	global  *rate.Limiter
	clients map[netip.Addr]*exchangeLimitEntry
	order   list.List
	now     func() time.Time
}

func newExchangeLimiter() *exchangeLimiter {
	return &exchangeLimiter{
		global:  rate.NewLimiter(rate.Every(exchangeGlobalPeriod), exchangeGlobalBurst),
		clients: make(map[netip.Addr]*exchangeLimitEntry),
		now:     time.Now,
	}
}

func (l *exchangeLimiter) Allow(remoteAddress string) bool {
	address, ok := remoteIP(remoteAddress)
	if !ok {
		return false
	}
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()

	l.removeIdle(now)
	entry, exists := l.clients[address]
	if !exists {
		if len(l.clients) >= exchangeClientLimit {
			l.removeOldest()
		}
		entry = &exchangeLimitEntry{
			address:  address,
			limiter:  rate.NewLimiter(rate.Every(exchangeClientPeriod), exchangeClientBurst),
			lastSeen: now,
		}
		entry.element = l.order.PushFront(entry)
		l.clients[address] = entry
	} else {
		entry.lastSeen = now
		l.order.MoveToFront(entry.element)
	}
	return l.global.AllowN(now, 1) && entry.limiter.AllowN(now, 1)
}

func (l *exchangeLimiter) removeIdle(now time.Time) {
	for element := l.order.Back(); element != nil; {
		entry := element.Value.(*exchangeLimitEntry)
		if now.Sub(entry.lastSeen) < exchangeClientIdle {
			return
		}
		previous := element.Prev()
		l.remove(entry)
		element = previous
	}
}

func (l *exchangeLimiter) removeOldest() {
	if element := l.order.Back(); element != nil {
		l.remove(element.Value.(*exchangeLimitEntry))
	}
}

func (l *exchangeLimiter) remove(entry *exchangeLimitEntry) {
	delete(l.clients, entry.address)
	l.order.Remove(entry.element)
}

func remoteIP(remoteAddress string) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(remoteAddress)
	if err != nil {
		host = remoteAddress
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}
