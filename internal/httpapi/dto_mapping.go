package httpapi

import (
	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/storage"
)

func logDetailsDTOs(values []middleware.LogDetails) []LogDetails {
	result := make([]LogDetails, len(values))
	for i, value := range values {
		result[i] = LogDetails{Domain: value.Domain, Counter: value.Counter, Host: value.Host}
	}
	return result
}

func clientCandidateDTOs(values []middleware.ClientCandidate) []ClientCandidate {
	result := make([]ClientCandidate, len(values))
	for i, value := range values {
		result[i] = ClientCandidate{Address: value.Address, Host: value.Host}
	}
	return result
}

func dashboardSummaryDTO(value middleware.DashboardSummary) *DashboardSummary {
	return &DashboardSummary{
		TotalQueries: value.TotalQueries, BlockedQueries: value.BlockedQueries,
		AllowedQueries: value.AllowedQueries, CacheHits: value.CacheHits, CacheMisses: value.CacheMisses,
	}
}

func dashboardHourlyDTOs(values []middleware.DashboardHourlyPoint) []DashboardHourlyPoint {
	result := make([]DashboardHourlyPoint, len(values))
	for i, value := range values {
		result[i] = DashboardHourlyPoint{
			HourBucket: value.HourBucket, HourStart: value.HourStart, TotalQueries: value.TotalQueries,
			BlockedQueries: value.BlockedQueries, AllowedQueries: value.AllowedQueries,
		}
	}
	return result
}

func blacklistStatusDTO(value middleware.BlacklistStatus) *BlacklistStatus {
	return &BlacklistStatus{
		Enabled: value.Enabled, File: value.File, ExternalFile: value.ExternalFile,
		ExternalRepo: value.ExternalRepo, ExternalRepoBranch: value.ExternalRepoBranch,
		ExternalPullPeriod: value.ExternalPullPeriod, Excludes: copyStrings(value.Excludes),
		PersistedExcludes: copyStrings(value.PersistedExcludes), PersistedHosts: copyStrings(value.PersistedHosts),
		RuntimeWhitelist: copyStrings(value.RuntimeWhitelist), BlockfileTotalEntries: value.BlockfileTotalEntries,
	}
}

func zenModeStatusDTO(value middleware.ZenModeStatus) *ZenModeStatus {
	return &ZenModeStatus{
		Enabled: value.Enabled, File: value.File, DurationMinutes: value.DurationMinutes,
		ConfiguredDomains: copyStrings(value.ConfiguredDomains), PersistedDomains: copyStrings(value.PersistedDomains),
		ConfiguredExcludes: copyStrings(value.ConfiguredExcludes), PersistedExcludes: copyStrings(value.PersistedExcludes),
		Labels: copyStrings(value.Labels), RuntimeDomains: copyStrings(value.RuntimeDomains),
		StartedAt: value.StartedAt, EndsAt: value.EndsAt, RemainingSeconds: value.RemainingSeconds,
	}
}

func hostEntryDTOs(values []middleware.HostEntry) []HostEntry {
	result := make([]HostEntry, len(values))
	for i, value := range values {
		result[i] = HostEntry{Domain: value.Domain, Address: value.Address}
	}
	return result
}

func staticResponseStatusDTO(value middleware.StaticResponseStatus) *StaticResponseStatus {
	return &StaticResponseStatus{
		Enabled: value.Enabled, File: value.File, Labels: copyStrings(value.Labels),
		ConfiguredHosts: hostEntryDTOs(value.ConfiguredHosts), PersistedHosts: hostEntryDTOs(value.PersistedHosts),
		RuntimeHosts: hostEntryDTOs(value.RuntimeHosts),
	}
}

func stubResolverStatusDTO(value middleware.StubResolverStatus) *StubResolverStatus {
	return &StubResolverStatus{
		Enabled: value.Enabled, ConfiguredStubs: copyStrings(value.ConfiguredStubs),
		RuntimeStubs: copyStrings(value.RuntimeStubs),
	}
}

func cacheStatusDTO(value middleware.CacheStatus) *CacheStatus {
	return &CacheStatus{
		Enabled: value.Enabled, Ttl: value.Ttl, Excludes: copyStrings(value.Excludes),
		Hits: value.Hits, Misses: value.Misses,
	}
}

func wildcardStatusDTO(value middleware.WildcardStatus) *WildcardStatus {
	return &WildcardStatus{
		Enabled: value.Enabled, PrimaryDomain: value.PrimaryDomain,
		AvailableExtraDomains: copyStrings(value.AvailableExtraDomains),
		EnabledExtraDomains:   copyStrings(value.EnabledExtraDomains),
		AllowPublicAddresses:  value.AllowPublicAddresses, TTL: value.TTL,
	}
}

func dnsLogStatusDTO(value middleware.DNSLogStatus) *DNSLogStatus {
	return &DNSLogStatus{
		Enabled: value.Enabled, DomainsPseudonymized: value.DomainsPseudonymized,
		ClientsPseudonymized: value.ClientsPseudonymized, KeyConfigured: value.KeyConfigured,
		RequiresClear: value.RequiresClear, Reason: value.Reason, QueuedEvents: value.QueuedEvents,
	}
}

func tagMemberDTOs(values []storage.TagMember) []TagMember {
	result := make([]TagMember, len(values))
	for i, value := range values {
		result[i] = TagMember{Address: value.Address, Host: value.Host, HasHostAlias: value.HasHostAlias}
	}
	return result
}

func knownHostDTOs(values []storage.KnownHost) []KnownHost {
	result := make([]KnownHost, len(values))
	for i, value := range values {
		result[i] = KnownHost{Address: value.Address, Host: value.Host}
	}
	return result
}
