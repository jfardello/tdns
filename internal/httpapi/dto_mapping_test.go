package httpapi

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/storage"
)

func TestDTOMappingsPreserveJSON(t *testing.T) {
	tests := []struct {
		name   string
		domain any
		dto    any
	}{
		{
			name: "blacklist status",
			domain: middleware.BlacklistStatus{
				Enabled: true, File: "block.txt", ExternalRepo: "repo", Excludes: []string{"example.com"},
				PersistedHosts: []string{"blocked.test"}, RuntimeWhitelist: []string{"allowed.test"}, BlockfileTotalEntries: 42,
			},
			dto: blacklistStatusDTO(middleware.BlacklistStatus{
				Enabled: true, File: "block.txt", ExternalRepo: "repo", Excludes: []string{"example.com"},
				PersistedHosts: []string{"blocked.test"}, RuntimeWhitelist: []string{"allowed.test"}, BlockfileTotalEntries: 42,
			}),
		},
		{
			name: "zen status",
			domain: middleware.ZenModeStatus{
				Enabled: true, DurationMinutes: 30, ConfiguredDomains: []string{"school.test"},
				PersistedExcludes: []string{"label:staff"}, RemainingSeconds: 120,
			},
			dto: zenModeStatusDTO(middleware.ZenModeStatus{
				Enabled: true, DurationMinutes: 30, ConfiguredDomains: []string{"school.test"},
				PersistedExcludes: []string{"label:staff"}, RemainingSeconds: 120,
			}),
		},
		{
			name: "static response status",
			domain: middleware.StaticResponseStatus{
				Enabled: true, File: "hosts", Labels: []string{"kids"},
				ConfiguredHosts: []middleware.HostEntry{{Domain: "example.test", Address: "192.0.2.1"}},
			},
			dto: staticResponseStatusDTO(middleware.StaticResponseStatus{
				Enabled: true, File: "hosts", Labels: []string{"kids"},
				ConfiguredHosts: []middleware.HostEntry{{Domain: "example.test", Address: "192.0.2.1"}},
			}),
		},
		{
			name:   "tag members",
			domain: []storage.TagMember{{Address: "192.0.2.1", Host: "office", HasHostAlias: true}},
			dto:    tagMemberDTOs([]storage.TagMember{{Address: "192.0.2.1", Host: "office", HasHostAlias: true}}),
		},
		{
			name: "dashboard summary",
			domain: middleware.DashboardSummary{
				TotalQueries: 100, BlockedQueries: 25, AllowedQueries: 75, CacheHits: 40, CacheMisses: 60,
			},
			dto: dashboardSummaryDTO(middleware.DashboardSummary{
				TotalQueries: 100, BlockedQueries: 25, AllowedQueries: 75, CacheHits: 40, CacheMisses: 60,
			}),
		},
		{
			name: "DNS-log status",
			domain: middleware.DNSLogStatus{
				Enabled: true, DomainsPseudonymized: true, KeyConfigured: true, QueuedEvents: 3,
			},
			dto: dnsLogStatusDTO(middleware.DNSLogStatus{
				Enabled: true, DomainsPseudonymized: true, KeyConfigured: true, QueuedEvents: 3,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertSameJSON(t, tt.domain, tt.dto)
		})
	}
}

func assertSameJSON(t *testing.T, left any, right any) {
	t.Helper()

	decode := func(value any) any {
		encoded, err := json.Marshal(value)
		if err != nil {
			t.Fatalf("marshal JSON: %v", err)
		}
		var decoded any
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("unmarshal JSON: %v", err)
		}
		return decoded
	}

	if got, want := decode(right), decode(left); !reflect.DeepEqual(got, want) {
		t.Fatalf("mapped JSON = %#v, want %#v", got, want)
	}
}
