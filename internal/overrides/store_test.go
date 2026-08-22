package overrides

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/db"
)

func TestStoreUpsertListAndDeleteByKind(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "overrides.sqlite")
	if _, err := db.Bootstrap(context.Background(), dbPath); err != nil {
		t.Fatalf("Bootstrap error: %v", err)
	}

	store, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}
	defer store.Close()

	if err := store.Upsert(context.Background(), OverrideCacheEnabled, OverrideSet, "enabled", "false"); err != nil {
		t.Fatalf("Upsert cache enabled error: %v", err)
	}
	if err := store.Upsert(context.Background(), OverrideCacheExclude, OverrideUpsert, "label:kids", ""); err != nil {
		t.Fatalf("Upsert cache exclude error: %v", err)
	}
	if err := store.UpsertBatch(context.Background(), []Row{
		{Kind: OverrideDNSLogDomainsPseudonymized, Op: OverrideSet, Target: "enabled", Value: "true"},
		{Kind: OverrideDNSLogClientsPseudonymized, Op: OverrideSet, Target: "enabled", Value: "false"},
	}); err != nil {
		t.Fatalf("UpsertBatch DNS-log privacy error: %v", err)
	}
	for kind, want := range map[Kind]string{
		OverrideDNSLogDomainsPseudonymized: "true",
		OverrideDNSLogClientsPseudonymized: "false",
	} {
		row, err := store.GetValue(context.Background(), kind, "enabled")
		if err != nil || row == nil || row.Value != want {
			t.Fatalf("privacy override %d = %#v, %v; want %q", kind, row, err, want)
		}
	}

	rows, err := store.ListByKind(context.Background(), OverrideCacheExclude)
	if err != nil {
		t.Fatalf("ListByKind error: %v", err)
	}
	if len(rows) != 1 || rows[0].Target != "label:kids" {
		t.Fatalf("unexpected rows: %#v", rows)
	}

	if err := store.DeleteByKind(context.Background(), OverrideCacheExclude); err != nil {
		t.Fatalf("DeleteByKind error: %v", err)
	}

	rows, err = store.ListByKind(context.Background(), OverrideCacheExclude)
	if err != nil {
		t.Fatalf("ListByKind error: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected no rows after delete, got %#v", rows)
	}
}

func TestApplyOverrides(t *testing.T) {
	conf := &config.Config{
		Cache: config.CacheConf{
			Enabled: true,
		},
		DNSLog: config.DNSLogConf{Enabled: true},
	}

	rows := []Row{
		{Kind: OverrideCacheEnabled, Value: "false"},
		{Kind: OverrideDNSLogEnabled, Value: "false"},
		{Kind: OverrideDNSLogDomainsPseudonymized, Value: "true"},
		{Kind: OverrideDNSLogClientsPseudonymized, Value: "false"},
		{Kind: OverrideWildcardEnabled, Value: "true"},
		{Kind: OverrideWildcardDomains, Value: `["NIP.IO.","xip.io"]`},
		{Kind: OverrideCacheExclude, Target: "LABEL:Kids"},
		{Kind: OverrideStaticHost, Target: "ads.example.", Value: "0.0.0.0"},
		{Kind: OverrideZenExclude, Target: "label:nozen"},
	}

	if err := Apply(conf, rows); err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	if conf.Cache.Enabled {
		t.Fatal("expected cache enabled override to disable cache")
	}
	if conf.DNSLog.Enabled {
		t.Fatal("expected DNS-log enabled override to disable DNS logging")
	}
	if !conf.DNSLog.Pseudonymization.Domains || conf.DNSLog.Pseudonymization.Clients {
		t.Fatalf("unexpected DNS-log pseudonymization overrides: %#v", conf.DNSLog.Pseudonymization)
	}
	if !conf.Wildcard.Enabled {
		t.Fatal("expected wildcard enabled override to enable wildcard resolution")
	}
	if got, want := conf.Wildcard.EnabledExtraDomains, []string{"nip.io", "xip.io"}; !slices.Equal(got, want) {
		t.Fatalf("wildcard domains = %#v, want %#v", got, want)
	}
	if len(conf.Cache.Excludes) != 1 || conf.Cache.Excludes[0] != "label:kids" {
		t.Fatalf("unexpected cache excludes: %#v", conf.Cache.Excludes)
	}
	if conf.StaticResponse.ExtraHosts["ads.example"] != "0.0.0.0" {
		t.Fatalf("unexpected static host overrides: %#v", conf.StaticResponse.ExtraHosts)
	}
	if len(conf.ZenMode.PersistedExcludes) != 1 || conf.ZenMode.PersistedExcludes[0] != "label:nozen" {
		t.Fatalf("unexpected zen excludes: %#v", conf.ZenMode.PersistedExcludes)
	}
}

func TestApplyWildcardDomainsRejectsInvalidJSON(t *testing.T) {
	conf := &config.Config{}
	err := Apply(conf, []Row{{Kind: OverrideWildcardDomains, Value: "not-json"}})
	if err == nil {
		t.Fatal("Apply accepted invalid wildcard domains JSON")
	}
	if json.Valid([]byte("not-json")) {
		t.Fatal("test fixture unexpectedly contains valid JSON")
	}
}
