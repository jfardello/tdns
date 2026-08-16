package overrides

import (
	"slices"
	"strconv"
	"strings"

	"github.com/jfardello/tdns/config"
)

func Apply(conf *config.Config, rows []Row) error {
	if conf == nil {
		return nil
	}

	for _, row := range rows {
		switch row.Kind {
		case OverrideStaticHost:
			if conf.StaticResponse.ExtraHosts == nil {
				conf.StaticResponse.ExtraHosts = map[string]string{}
			}
			target := normalizeDomain(row.Target)
			if target == "" || row.Value == "" {
				continue
			}
			conf.StaticResponse.ExtraHosts[target] = strings.TrimSpace(row.Value)
		case OverrideZenDomain:
			target := normalizeDomain(row.Target)
			if target == "" {
				continue
			}
			conf.ZenMode.PersistedDomains = appendUnique(conf.ZenMode.PersistedDomains, target)
		case OverrideZenExclude:
			target := normalizeSelector(row.Target)
			if target == "" {
				continue
			}
			conf.ZenMode.PersistedExcludes = appendUnique(conf.ZenMode.PersistedExcludes, target)
		case OverrideBlacklistHost:
			target := normalizeDomain(row.Target)
			if target == "" {
				continue
			}
			conf.Blacklist.ExtraHosts = appendUnique(conf.Blacklist.ExtraHosts, target)
		case OverrideBlacklistExclude:
			target := normalizeSelector(row.Target)
			if target == "" {
				continue
			}
			conf.Blacklist.PersistedExcludes = appendUnique(conf.Blacklist.PersistedExcludes, target)
		case OverrideCacheEnabled:
			enabled, err := strconv.ParseBool(strings.TrimSpace(row.Value))
			if err != nil {
				return err
			}
			conf.Cache.Enabled = enabled
		case OverrideCacheExclude:
			target := normalizeCacheSelector(row.Target)
			if target == "" {
				continue
			}
			conf.Cache.Excludes = appendUnique(conf.Cache.Excludes, target)
		case OverrideDNSLogEnabled:
			enabled, err := strconv.ParseBool(strings.TrimSpace(row.Value))
			if err != nil {
				return err
			}
			conf.DNSLog.Enabled = enabled
		}
	}
	return nil
}

func NormalizeDomains(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		target := normalizeDomain(value)
		if target == "" {
			continue
		}
		normalized = appendUnique(normalized, target)
	}
	return normalized
}

func NormalizeSelectors(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		target := normalizeSelector(value)
		if target == "" {
			continue
		}
		normalized = appendUnique(normalized, target)
	}
	return normalized
}

func NormalizeCacheSelectors(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		target := normalizeCacheSelector(value)
		if target == "" {
			continue
		}
		normalized = appendUnique(normalized, target)
	}
	return normalized
}

func appendUnique(values []string, value string) []string {
	if value == "" || slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

func normalizeDomain(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.TrimSuffix(value, ".")
}

func normalizeSelector(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	if strings.HasPrefix(value, "label:") {
		label := strings.TrimSpace(strings.TrimPrefix(value, "label:"))
		if label == "" {
			return ""
		}
		return "label:" + label
	}
	return normalizeDomain(value)
}

func normalizeCacheSelector(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	switch {
	case strings.HasPrefix(value, "label:"):
		label := strings.TrimSpace(strings.TrimPrefix(value, "label:"))
		if label == "" {
			return ""
		}
		return "label:" + label
	case strings.HasPrefix(value, "ip:"):
		ip := strings.TrimSpace(strings.TrimPrefix(value, "ip:"))
		if ip == "" {
			return ""
		}
		return "ip:" + ip
	case strings.HasPrefix(value, "cidr:"):
		cidr := strings.TrimSpace(strings.TrimPrefix(value, "cidr:"))
		if cidr == "" {
			return ""
		}
		return "cidr:" + cidr
	}
	return value
}
