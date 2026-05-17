package middleware

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/allegro/bigcache/v3"
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

func TestCacheStoresAndRestoresFullResponse(t *testing.T) {
	cache := newTestCache(t)
	request := new(dns.Msg)
	request.SetQuestion("download.opensuse.org.", dns.TypeA)
	key := cache.Key(&request.Question[0], nil)

	response := new(dns.Msg)
	response.SetReply(request)
	response.Answer = mustRRs(t,
		"download.opensuse.org. 300 IN CNAME downloadcontent.opensuse.org.",
		"downloadcontent.opensuse.org. 300 IN A 195.135.223.226",
		"downloadcontent.opensuse.org. 300 IN A 195.135.223.227",
	)

	if err := cache.Set(key, response); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	nextRequest := new(dns.Msg)
	nextRequest.SetQuestion("download.opensuse.org.", dns.TypeA)
	raw, err := cache.backend.Get(key)
	if err != nil {
		t.Fatalf("backend.Get error: %v", err)
	}
	got, ok, err := cache.responseFromEntry(raw, nextRequest)
	if err != nil {
		t.Fatalf("responseFromEntry error: %v", err)
	}
	if !ok {
		t.Fatal("expected cached response to be usable")
	}
	if got.Id != nextRequest.Id {
		t.Fatalf("cached response id got %d, want %d", got.Id, nextRequest.Id)
	}
	if len(got.Answer) != 3 {
		t.Fatalf("answer count got %d, want 3: %v", len(got.Answer), got.Answer)
	}
	if got.Answer[0].Header().Rrtype != dns.TypeCNAME {
		t.Fatalf("first answer got %s, want CNAME", dns.TypeToString[got.Answer[0].Header().Rrtype])
	}
	if got.Answer[1].Header().Name != "downloadcontent.opensuse.org." || got.Answer[2].Header().Name != "downloadcontent.opensuse.org." {
		t.Fatalf("expected canonical-target A records to be preserved, got %v", got.Answer)
	}
}

func TestCacheHitAdjustsTTLAndExpiresWholeAnswer(t *testing.T) {
	cache := newTestCache(t)
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)
	key := cache.Key(&request.Question[0], nil)

	response := new(dns.Msg)
	response.SetReply(request)
	response.Answer = mustRRs(t, "example.org. 60 IN A 192.0.2.1")

	if err := cache.Set(key, response); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	ageCacheEntry(t, cache, key, 5*time.Second)

	raw, err := cache.backend.Get(key)
	if err != nil {
		t.Fatalf("backend.Get error: %v", err)
	}
	got, ok, err := cache.responseFromEntry(raw, request)
	if err != nil {
		t.Fatalf("responseFromEntry error: %v", err)
	}
	if !ok {
		t.Fatal("expected cached response to still be usable")
	}
	if got.Answer[0].Header().Ttl > 55 {
		t.Fatalf("ttl got %d, want at most 55", got.Answer[0].Header().Ttl)
	}

	ageCacheEntry(t, cache, key, 61*time.Second)
	raw, err = cache.backend.Get(key)
	if err != nil {
		t.Fatalf("backend.Get error: %v", err)
	}
	_, ok, err = cache.responseFromEntry(raw, request)
	if err != nil {
		t.Fatalf("responseFromEntry expired error: %v", err)
	}
	if ok {
		t.Fatal("expected cached response to expire when an answer TTL is exhausted")
	}
}

func TestCacheSetSkipsCacheHits(t *testing.T) {
	cache := newTestCache(t)
	request := new(dns.Msg)
	request.SetQuestion("example.org.", dns.TypeA)
	key := cache.Key(&request.Question[0], nil)

	response := new(dns.Msg)
	response.SetReply(request)
	response.Answer = mustRRs(t, "example.org. 60 IN A 192.0.2.1")

	message := &Message{}
	message.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		Values: map[string]string{cacheHitValue: "true"},
	}))
	message.SetMsg(response)

	cacheSet := &CacheSet{cache: cache}
	if _, err := cacheSet.Run(message); err != nil {
		t.Fatalf("CacheSet.Run error: %v", err)
	}
	if _, err := cache.backend.Get(key); err == nil {
		t.Fatal("expected cache hit response not to be cached again")
	}
}

func newTestCache(t *testing.T) *Cache {
	t.Helper()
	backend, err := bigcache.New(context.Background(), bigcache.Config{
		Shards:           1,
		LifeWindow:       time.Hour,
		CleanWindow:      time.Minute,
		MaxEntrySize:     8192,
		HardMaxCacheSize: 1,
	})
	if err != nil {
		t.Fatalf("bigcache.New error: %v", err)
	}
	return &Cache{backend: backend, enabled: true, ttl: 60}
}

func mustRRs(t *testing.T, values ...string) []dns.RR {
	t.Helper()
	records := make([]dns.RR, 0, len(values))
	for _, value := range values {
		rr, err := dns.NewRR(value)
		if err != nil {
			t.Fatalf("dns.NewRR(%q) error: %v", value, err)
		}
		records = append(records, rr)
	}
	return records
}

func ageCacheEntry(t *testing.T, cache *Cache, key string, age time.Duration) {
	t.Helper()
	raw, err := cache.backend.Get(key)
	if err != nil {
		t.Fatalf("backend.Get error: %v", err)
	}
	entry := cachedMessage{}
	if err := json.Unmarshal(raw, &entry); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}
	entry.CachedAt = time.Now().Add(-age).Unix()
	raw, err = json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	if err := cache.backend.Set(key, raw); err != nil {
		t.Fatalf("backend.Set error: %v", err)
	}
}
