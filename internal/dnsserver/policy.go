package dnsserver

import (
	"container/list"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/jfardello/tdns/config"
	"golang.org/x/time/rate"
)

const (
	maxCIDRs            = 256
	maxConfiguredRate   = 1_000_000
	maxConfiguredBurst  = 2_000_000
	maxConcurrentWork   = 4096
	maxClientState      = 65_536
	maxClientIdlePeriod = 24 * time.Hour
)

type Policy struct {
	allowed       []netip.Prefix
	clients       *clientLimiters
	responseLimit *rate.Limiter
	now           func() time.Time
}

func NewPolicy(conf config.DNSAccessConf) (*Policy, error) {
	if len(conf.AllowedClientCIDRs) > maxCIDRs {
		return nil, fmt.Errorf("dns_access.allowed_client_cidrs has %d entries, maximum is %d", len(conf.AllowedClientCIDRs), maxCIDRs)
	}
	if err := validateLimit("client_queries_per_second", conf.ClientQueriesPerSecond, maxConfiguredRate); err != nil {
		return nil, err
	}
	if err := validateLimit("client_burst", conf.ClientBurst, maxConfiguredBurst); err != nil {
		return nil, err
	}
	if err := validateLimit("global_responses_per_second", conf.GlobalResponsesPerSecond, maxConfiguredRate); err != nil {
		return nil, err
	}
	if err := validateLimit("global_response_burst", conf.GlobalResponseBurst, maxConfiguredBurst); err != nil {
		return nil, err
	}
	if err := validateLimit("max_concurrent_upstreams", conf.MaxConcurrentUpstreams, maxConcurrentWork); err != nil {
		return nil, err
	}
	if err := validateLimit("max_tracked_clients", conf.MaxTrackedClients, maxClientState); err != nil {
		return nil, err
	}

	idleTimeout, err := time.ParseDuration(conf.ClientIdleTimeout)
	if err != nil {
		return nil, fmt.Errorf("dns_access.client_idle_timeout: %w", err)
	}
	if idleTimeout <= 0 || idleTimeout > maxClientIdlePeriod {
		return nil, fmt.Errorf("dns_access.client_idle_timeout must be greater than zero and at most %s", maxClientIdlePeriod)
	}

	allowed := make([]netip.Prefix, 0, len(conf.AllowedClientCIDRs))
	seen := make(map[netip.Prefix]struct{}, len(conf.AllowedClientCIDRs))
	for _, value := range conf.AllowedClientCIDRs {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("dns_access.allowed_client_cidrs contains %q: %w", value, err)
		}
		if prefix.Addr().Is4In6() {
			prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
		}
		prefix = prefix.Masked()
		if _, exists := seen[prefix]; exists {
			return nil, fmt.Errorf("dns_access.allowed_client_cidrs contains duplicate prefix %q", prefix)
		}
		seen[prefix] = struct{}{}
		allowed = append(allowed, prefix)
	}

	return &Policy{
		allowed: allowed,
		clients: newClientLimiters(
			rate.Limit(conf.ClientQueriesPerSecond),
			conf.ClientBurst,
			conf.MaxTrackedClients,
			idleTimeout,
		),
		responseLimit: rate.NewLimiter(rate.Limit(conf.GlobalResponsesPerSecond), conf.GlobalResponseBurst),
		now:           time.Now,
	}, nil
}

func validateLimit(name string, value, maximum int) error {
	if value <= 0 || value > maximum {
		return fmt.Errorf("dns_access.%s must be between 1 and %d", name, maximum)
	}
	return nil
}

func (p *Policy) admit(remote net.Addr) (netip.Addr, string, bool) {
	addr, ok := remoteIP(remote)
	if !ok || !p.isAllowed(addr) {
		dnsRejections.WithLabelValues("acl").Inc()
		return netip.Addr{}, "acl", false
	}
	if !p.clients.allow(addr, p.now()) {
		dnsRejections.WithLabelValues("client_rate").Inc()
		return addr, "client_rate", false
	}
	return addr, "", true
}

func (p *Policy) allowResponse() bool {
	if p.responseLimit.AllowN(p.now(), 1) {
		return true
	}
	dnsRejections.WithLabelValues("response_rate").Inc()
	return false
}

func (p *Policy) isAllowed(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.IsLoopback() {
		return true
	}
	for _, prefix := range p.allowed {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func remoteIP(remote net.Addr) (netip.Addr, bool) {
	var ip net.IP
	switch addr := remote.(type) {
	case *net.UDPAddr:
		ip = addr.IP
	case *net.TCPAddr:
		ip = addr.IP
	case *net.IPAddr:
		ip = addr.IP
	default:
		return netip.Addr{}, false
	}
	parsed, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, false
	}
	return parsed.Unmap(), true
}

type clientLimiters struct {
	mu          sync.Mutex
	limit       rate.Limit
	burst       int
	maxEntries  int
	idleTimeout time.Duration
	entries     map[netip.Addr]*list.Element
	lru         list.List
}

type clientLimiterEntry struct {
	addr     netip.Addr
	limiter  *rate.Limiter
	lastSeen time.Time
}

func newClientLimiters(limit rate.Limit, burst, maxEntries int, idleTimeout time.Duration) *clientLimiters {
	return &clientLimiters{
		limit:       limit,
		burst:       burst,
		maxEntries:  maxEntries,
		idleTimeout: idleTimeout,
		entries:     make(map[netip.Addr]*list.Element),
	}
}

func (c *clientLimiters) allow(addr netip.Addr, now time.Time) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.removeExpired(now)
	if element, exists := c.entries[addr]; exists {
		entry := element.Value.(*clientLimiterEntry)
		entry.lastSeen = now
		c.lru.MoveToFront(element)
		return entry.limiter.AllowN(now, 1)
	}

	if len(c.entries) >= c.maxEntries {
		c.removeOldest()
	}
	entry := &clientLimiterEntry{
		addr:     addr,
		limiter:  rate.NewLimiter(c.limit, c.burst),
		lastSeen: now,
	}
	element := c.lru.PushFront(entry)
	c.entries[addr] = element
	trackedDNSClients.Set(float64(len(c.entries)))
	return entry.limiter.AllowN(now, 1)
}

func (c *clientLimiters) removeExpired(now time.Time) {
	for {
		element := c.lru.Back()
		if element == nil {
			return
		}
		entry := element.Value.(*clientLimiterEntry)
		if now.Sub(entry.lastSeen) < c.idleTimeout {
			return
		}
		c.remove(element)
	}
}

func (c *clientLimiters) removeOldest() {
	if element := c.lru.Back(); element != nil {
		c.remove(element)
	}
}

func (c *clientLimiters) remove(element *list.Element) {
	entry := element.Value.(*clientLimiterEntry)
	delete(c.entries, entry.addr)
	c.lru.Remove(element)
	trackedDNSClients.Set(float64(len(c.entries)))
}
