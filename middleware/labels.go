package middleware

import (
	"net"
	"slices"
	"sort"
	"strings"
)

type selectorSet struct {
	Domains []string
	Labels  []string
}

func normalizeLabels(labels []string) []string {
	normalized := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label == "" || slices.Contains(normalized, label) {
			continue
		}
		normalized = append(normalized, label)
	}
	sort.Strings(normalized)
	return normalized
}

func parseSelectors(values []string) selectorSet {
	selectors := selectorSet{
		Domains: []string{},
		Labels:  []string{},
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if label, ok := strings.CutPrefix(value, "label:"); ok {
			label = strings.TrimSpace(label)
			if label == "" || slices.Contains(selectors.Labels, label) {
				continue
			}
			selectors.Labels = append(selectors.Labels, label)
			continue
		}

		domain := normalizeDomainPattern(value)
		if domain == "" || slices.Contains(selectors.Domains, domain) {
			continue
		}
		selectors.Domains = append(selectors.Domains, domain)
	}
	sort.Strings(selectors.Domains)
	sort.Strings(selectors.Labels)
	return selectors
}

func normalizeSelectorValues(values []string) []string {
	selectors := parseSelectors(values)
	normalized := make([]string, 0, len(selectors.Domains)+len(selectors.Labels))
	normalized = append(normalized, selectors.Domains...)
	for _, label := range selectors.Labels {
		normalized = append(normalized, "label:"+label)
	}
	return normalized
}

func matchesAnyLabel(have []string, wanted []string) bool {
	if len(have) == 0 || len(wanted) == 0 {
		return false
	}
	for _, label := range have {
		if slices.Contains(wanted, label) {
			return true
		}
	}
	return false
}

func matchesClientScope(message *Message, labels []string) bool {
	if len(labels) == 0 {
		return true
	}
	return matchesAnyLabel(message.Labels(), labels)
}

func labelFingerprint(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	normalized := normalizeLabels(labels)
	return strings.Join(normalized, ",")
}

type cidrLabels struct {
	network *net.IPNet
	labels  []string
}

type requestSelectorSet struct {
	Labels []string
	IPs    []string
	CIDRs  []*net.IPNet
}

func parseRequestSelectors(values []string) requestSelectorSet {
	selectors := requestSelectorSet{
		Labels: []string{},
		IPs:    []string{},
		CIDRs:  []*net.IPNet{},
	}

	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
		if value == "" {
			continue
		}

		switch {
		case strings.HasPrefix(value, "label:"):
			label := strings.TrimSpace(strings.TrimPrefix(value, "label:"))
			if label == "" || slices.Contains(selectors.Labels, label) {
				continue
			}
			selectors.Labels = append(selectors.Labels, label)
		case strings.HasPrefix(value, "ip:"):
			ip := strings.TrimSpace(strings.TrimPrefix(value, "ip:"))
			if ip == "" || slices.Contains(selectors.IPs, ip) {
				continue
			}
			selectors.IPs = append(selectors.IPs, ip)
		case strings.HasPrefix(value, "cidr:"):
			cidr := strings.TrimSpace(strings.TrimPrefix(value, "cidr:"))
			if _, network, err := net.ParseCIDR(cidr); err == nil {
				selectors.CIDRs = append(selectors.CIDRs, network)
			}
		default:
			if _, network, err := net.ParseCIDR(value); err == nil {
				selectors.CIDRs = append(selectors.CIDRs, network)
				continue
			}
			if ip := net.ParseIP(value); ip != nil {
				ipStr := ip.String()
				if !slices.Contains(selectors.IPs, ipStr) {
					selectors.IPs = append(selectors.IPs, ipStr)
				}
			}
		}
	}

	sort.Strings(selectors.Labels)
	sort.Strings(selectors.IPs)
	return selectors
}

func normalizeRequestSelectors(values []string) []string {
	selectors := parseRequestSelectors(values)
	normalized := make([]string, 0, len(selectors.Labels)+len(selectors.IPs)+len(selectors.CIDRs))
	for _, label := range selectors.Labels {
		normalized = append(normalized, "label:"+label)
	}
	for _, ip := range selectors.IPs {
		normalized = append(normalized, "ip:"+ip)
	}
	for _, network := range selectors.CIDRs {
		if network == nil {
			continue
		}
		normalized = append(normalized, "cidr:"+network.String())
	}
	return normalized
}

func remoteAddressIP(addr net.Addr) string {
	switch typed := addr.(type) {
	case *net.UDPAddr:
		return typed.IP.String()
	case *net.TCPAddr:
		return typed.IP.String()
	default:
		if addr == nil {
			return ""
		}
		host, _, err := net.SplitHostPort(addr.String())
		if err == nil {
			return host
		}
		return addr.String()
	}
}

func matchesRequestSelectors(message *Message, selectors requestSelectorSet) bool {
	if matchesAnyLabel(message.Labels(), selectors.Labels) {
		return true
	}

	cv, err := message.GetCtxValue()
	if err != nil {
		return false
	}

	remote := remoteAddressIP(cv.RemoteAddr)
	if remote == "" {
		return false
	}
	if slices.Contains(selectors.IPs, remote) {
		return true
	}
	if len(selectors.CIDRs) == 0 {
		return false
	}

	ip := net.ParseIP(remote)
	if ip == nil {
		return false
	}
	for _, network := range selectors.CIDRs {
		if network != nil && network.Contains(ip) {
			return true
		}
	}
	return false
}
