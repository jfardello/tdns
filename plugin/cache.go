package plugin

import (
	"context"
	"fmt"
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
	cacheInit bool = false
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

func (cs *CacheGet) Run(m *dns.Msg) (*dns.RR, bool, error) {
	logger := log.GetLogger("CacheGet", "Run")
	q := m.Question[0]
	r, err := cs.cache.backend.Get(cache.Key(&q))
	if err != nil {
		misses.Inc()
		return nil, false, err
	}
	logger.Debugf("Responding from cache for %s", q.Name)
	rr, err := dns.NewRR(string(r))
	if err != nil {
		return nil, false, err
	}

	hits.Inc()
	return &rr, true, nil

}

func (cs *CacheGet) Info() (string, Ptype) {
	return "cacheget", PreRouting
}
func (cs *CacheGet) Config(c config.Config) error {
	return nil

}
func (cs *CacheGet) Init() error {
	cs.cache = GetCache()
	return nil
}

type CacheSet struct {
	cache *Cache
}

func (cs *CacheSet) Run(m *dns.Msg) (*dns.RR, bool, error) {
	logger := log.GetLogger("CacheSet", "Run")
	if m.Rcode == dns.RcodeSuccess {
		if len(m.Question) > 0 {
			q := m.Question[0]
			logger.Debugf("Setting cache for %s", q.Name)
			err := cache.Set(cache.Key(&q), m)
			return nil, false, err
		}
	}
	return nil, false, nil
}

func (cs *CacheSet) Info() (string, Ptype) {
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
	backend *bigcache.BigCache
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
	if k == "" {
		//Don't fail if theres no cache key
		logger := log.GetLogger("Cache", "Set")
		logger.Debugf("No cache key, skipping chache phase.")
		return nil
	}

	idx := len(m.Answer) - 1

	//Just cache Anchoss, ivv6 anchors and cnames.
	if t, ok := m.Answer[idx].(*dns.A); ok {
		return c.backend.Set(k, []byte(t.String()))
	} else if t, ok := m.Answer[idx].(*dns.CNAME); ok {
		return c.backend.Set(k, []byte(t.String()))
	} else if t, ok := m.Answer[idx].(*dns.AAAA); ok {
		return c.backend.Set(k, []byte(t.String()))
	}
	return nil
}

func (c *Cache) Key(q *dns.Question) string {

	switch q.Qtype {
	case dns.TypeA, dns.TypeCNAME, dns.TypeAAAA:
		return fmt.Sprintf("%d-%d-%s", q.Qtype, q.Qclass, q.Name)
	}
	return ""

}
func (c *Cache) Clear() error {
	return c.backend.Reset()
}

func (c *Cache) Status() bigcache.Stats {
	return c.backend.Stats()
}

func NewCache() *Cache {
	config := bigcache.Config{
		Shards:           64,
		HardMaxCacheSize: 32,
		Verbose:          true,
		MaxEntrySize:     256,
		CleanWindow:      1 * time.Minute,
		LifeWindow:       3 * time.Minute,
	}
	c, _ := bigcache.New(context.Background(), config)
	return &Cache{
		backend: c,
	}
}
