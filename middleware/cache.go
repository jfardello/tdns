package middleware

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"

	"github.com/allegro/bigcache/v3"
	"github.com/miekg/dns"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	misses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tdns_missed_calls_total",
		Help: "The total number of dns requests missed from cache",
	})
	hits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tdns_hit_calls_total",
		Help: "The total number of dns requests served from cache",
	})

	cache     *Cache
	cacheInit = false
)

// GetCache returns the cache singleton.
func GetCache() *Cache {
	if cacheInit {
		return cache
	}
	cache = NewCache()
	cacheInit = true
	return cache
}

type CacheGet struct {
	cache *Cache
}

func (cs *CacheGet) Run(mess *Message) (*Message, error) {
	logger := log.GetLogger("CacheGet", "Run")
	m, err := mess.GetMsg()
	if err != nil {
		return nil, err
	}
	if !cs.cache.IsEnabled() || cs.cache.ShouldBypass(mess) {
		return mess, nil
	}

	q := m.Question[0]
	key := cache.Key(&q, mess.Labels())
	r, err := cs.cache.backend.Get(key)
	if err != nil {
		misses.Inc()
		return mess, nil
	}
	logger.Debugf("Responding from cache for %s", q.Name)
	rr, err := dns.NewRR(string(r))
	if err != nil {
		return nil, err
	}

	m.Answer = append(m.Answer, rr)
	mess.SetMsg(m)
	mess.Resolved(true)
	hits.Inc()
	return mess, nil
}

func (cs *CacheGet) Info() (string, Stage) {
	return "cacheget", PreRouting
}

func (cs *CacheGet) Config(_ config.Config) error {
	return nil
}

func (cs *CacheGet) Init() error {
	cs.cache = GetCache()
	return nil
}

type CacheSet struct {
	cache *Cache
}

func (cs *CacheSet) Run(mess *Message) (*Message, error) {
	logger := log.GetLogger("DNSLog", "Run")
	m, err := mess.GetMsg()
	if err != nil {
		return nil, err
	}
	if !cs.cache.IsEnabled() || cs.cache.ShouldBypass(mess) {
		return mess, nil
	}
	if m.Rcode == dns.RcodeSuccess && m.Response && len(m.Answer) > 0 {
		q := m.Question[0]
		key := cache.Key(&q, mess.Labels())
		logger.Debugf("Setting cache for %s, key: %s", q.Name, key)
		err := cache.Set(key, m)
		return mess, err
	}
	return mess, nil
}

func (cs *CacheSet) Info() (string, Stage) {
	return "cacheset", PostRouting
}

func (cs *CacheSet) Config(config.Config) error {
	return nil
}

func (cs *CacheSet) Init() error {
	cs.cache = GetCache()
	return nil
}

type Cache struct {
	backend  *bigcache.BigCache
	mu       sync.RWMutex
	enabled  bool
	excludes []string
	rules    requestSelectorSet
	ttl      int
}

type CacheStatus struct {
	Enabled  bool     `json:"enabled"`
	Ttl      int      `json:"ttl"`
	Excludes []string `json:"excludes,omitempty"`
	Hits     int64    `json:"hits"`
	Misses   int64    `json:"misses"`
}

func (c *Cache) Get(k string) (string, bool) {
	if k == "" {
		return "", false
	}
	val, err := c.backend.Get(k)
	if err == nil {
		return string(val), true
	}
	return "", false
}

func (c *Cache) Set(k string, m *dns.Msg) error {
	logger := log.GetLogger("Cache", "Set")
	if k == "" {
		logger.Debugf("No cache key, skipping chache phase.")
		return nil
	}

	idx := len(m.Answer) - 1
	if idx < 0 {
		return nil
	}

	if t, ok := m.Answer[idx].(*dns.A); ok {
		logger.Debugf("Seting cache for %s", k)
		return c.backend.Set(k, []byte(t.String()))
	} else if t, ok := m.Answer[idx].(*dns.CNAME); ok {
		logger.Debugf("Seting cache for %s", k)
		return c.backend.Set(k, []byte(t.String()))
	} else if t, ok := m.Answer[idx].(*dns.AAAA); ok {
		logger.Debugf("Seting cache for %s", k)
		return c.backend.Set(k, []byte(t.String()))
	}
	return nil
}

func (c *Cache) Key(q *dns.Question, labels []string) string {
	labelPart := labelFingerprint(labels)

	switch q.Qtype {
	case dns.TypeA, dns.TypeCNAME, dns.TypeAAAA:
		return fmt.Sprintf("%d-%d-%s-%s", q.Qtype, q.Qclass, q.Name, labelPart)
	}
	return ""
}

func (c *Cache) Clear() error {
	return c.backend.Reset()
}

func (c *Cache) Status() bigcache.Stats {
	return c.backend.Stats()
}

func (c *Cache) StatusView() CacheStatus {
	stats := c.backend.Stats()

	c.mu.RLock()
	defer c.mu.RUnlock()

	return CacheStatus{
		Enabled:  c.enabled,
		Ttl:      c.ttl,
		Excludes: append([]string(nil), c.excludes...),
		Hits:     stats.Hits,
		Misses:   stats.Misses,
	}
}

func (c *Cache) SetEnabled(state bool) {
	c.mu.Lock()
	c.enabled = state
	c.mu.Unlock()
}

func (c *Cache) IsEnabled() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabled
}

func (c *Cache) ReplaceExcludes(values []string) {
	normalized := normalizeRequestSelectors(values)
	c.mu.Lock()
	c.excludes = normalized
	c.rules = parseRequestSelectors(normalized)
	c.mu.Unlock()
}

func (c *Cache) ShouldBypass(message *Message) bool {
	c.mu.RLock()
	rules := c.rules
	c.mu.RUnlock()
	return matchesRequestSelectors(message, rules)
}

func NewCache() *Cache {
	conf := config.GetRunningConfig()
	cf := bigcache.Config{
		Shards:           64,
		HardMaxCacheSize: 32,
		Verbose:          true,
		MaxEntrySize:     512,
		CleanWindow:      1 * time.Minute,
		LifeWindow:       time.Duration(conf.Cache.Ttl) * time.Minute,
	}
	c, _ := bigcache.New(context.Background(), cf)
	cache := &Cache{
		backend: c,
		enabled: conf.Cache.Enabled,
		ttl:     conf.Cache.Ttl,
	}
	cache.ReplaceExcludes(conf.Cache.Excludes)
	return cache
}
