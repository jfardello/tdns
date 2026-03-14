package middleware

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
	q := m.Question[0]
	r, err := cs.cache.backend.Get(cache.Key(&q))
	//ToDO: distinguish EntryNotFoudErr from err
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
	if m.Rcode == dns.RcodeSuccess {
		if m.Response && len(m.Answer) > 0 {
			q := m.Question[0]
			logger.Debugf("Setting cache for %s, key: %s", q.Name, cache.Key(&q))
			err := cache.Set(cache.Key(&q), m)
			return mess, err
		}
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
	logger := log.GetLogger("Cache", "Set")
	if k == "" {
		//Don't fail if there is no cache key
		logger.Debugf("No cache key, skipping chache phase.")
		return nil
	}

	idx := len(m.Answer) - 1
	if idx < 0 {
		return nil
	}

	//ToDo: cache a all the RRs for th answer,
	//Just cache ipv4/ipv6 anchors and aliases.
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
	return &Cache{
		backend: c,
	}
}
