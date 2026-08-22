package middleware

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

const (
	DefaultWildcardDomain = "tdns.home.arpa"
	DefaultWildcardTTL    = uint32(60)
)

var (
	ErrInvalidWildcardName    = errors.New("invalid wildcard DNS name")
	ErrUnsafeWildcardAddress  = errors.New("wildcard DNS address is not local")
	defaultWildcardExtraZones = []string{"nip.io", "sslip.io", "xip.io"}
)

type WildcardStatus struct {
	Enabled               bool
	PrimaryDomain         string
	AvailableExtraDomains []string
	EnabledExtraDomains   []string
	AllowPublicAddresses  bool
	TTL                   uint32
}

type Wildcard struct {
	mu                    sync.RWMutex
	enabled               bool
	primaryDomain         string
	availableExtraDomains []string
	enabledExtraDomains   []string
	managedDomains        []string
	allowPublicAddresses  bool
	ttl                   uint32
}

func (w *Wildcard) Config(conf config.Config) error {
	wildcardConf := conf.Wildcard
	primary := wildcardConf.PrimaryDomain
	if strings.TrimSpace(primary) == "" {
		primary = DefaultWildcardDomain
	}
	primary, err := normalizeWildcardDomain(primary)
	if err != nil {
		return fmt.Errorf("configure wildcard primary domain: %w", err)
	}

	availableValues := wildcardConf.AvailableExtraDomains
	if len(availableValues) == 0 {
		availableValues = defaultWildcardExtraZones
	}
	available, err := normalizeWildcardDomains(availableValues)
	if err != nil {
		return fmt.Errorf("configure available wildcard domains: %w", err)
	}
	enabled, err := normalizeWildcardDomains(wildcardConf.EnabledExtraDomains)
	if err != nil {
		return fmt.Errorf("configure enabled wildcard domains: %w", err)
	}
	for _, domain := range enabled {
		if !slices.Contains(available, domain) {
			return fmt.Errorf("configure enabled wildcard domains: %q is not in available_extra_domains", domain)
		}
	}

	ttl := wildcardConf.TTL
	if ttl == 0 {
		ttl = DefaultWildcardTTL
	}
	managed := append([]string{primary}, enabled...)
	slices.SortFunc(managed, func(left, right string) int {
		return len(right) - len(left)
	})

	w.mu.Lock()
	w.enabled = wildcardConf.Enabled
	w.primaryDomain = primary
	w.availableExtraDomains = available
	w.enabledExtraDomains = enabled
	w.managedDomains = managed
	w.allowPublicAddresses = wildcardConf.AllowPublicAddresses
	w.ttl = ttl
	w.mu.Unlock()
	return nil
}

func (w *Wildcard) Init() error {
	return nil
}

func (w *Wildcard) Info() (string, Stage) {
	return "wildcard", PreRouting
}

func (w *Wildcard) Run(message *Message) (*Message, error) {
	w.mu.RLock()
	enabled := w.enabled
	domains := append([]string(nil), w.managedDomains...)
	allowPublic := w.allowPublicAddresses
	ttl := w.ttl
	w.mu.RUnlock()
	if !enabled {
		return message, nil
	}

	request, err := message.GetMsg()
	if err != nil {
		return message, err
	}
	if len(request.Question) == 0 {
		return message, nil
	}
	question := request.Question[0]
	address, managed, parseErr := parseWildcardName(question.Name, domains)
	if !managed {
		return message, nil
	}
	if parseErr == nil && !allowPublic && !isLocalWildcardAddress(address) {
		parseErr = fmt.Errorf("%w: %s", ErrUnsafeWildcardAddress, address)
	}

	response := new(dns.Msg)
	if parseErr != nil {
		response.SetRcode(request, dns.RcodeNameError)
	} else {
		response.SetReply(request)
		if question.Qclass == dns.ClassINET {
			switch {
			case question.Qtype == dns.TypeA && address.Is4():
				response.Answer = append(response.Answer, &dns.A{
					Hdr: dns.RR_Header{Name: question.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   net.IP(address.AsSlice()),
				})
			case question.Qtype == dns.TypeAAAA && address.Is6():
				response.Answer = append(response.Answer, &dns.AAAA{
					Hdr:  dns.RR_Header{Name: question.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
					AAAA: net.IP(address.AsSlice()),
				})
			}
		}
		if err := message.AddValue("tdns/wildcard", "true"); err != nil {
			return message, err
		}
	}
	response.Authoritative = true
	response.AuthenticatedData = false
	message.SetMsg(response)
	message.Resolved(true)
	return message, nil
}

func (w *Wildcard) SetEnabled(enabled bool) {
	w.mu.Lock()
	w.enabled = enabled
	w.mu.Unlock()
}

func (w *Wildcard) ValidateEnabledExtraDomains(values []string) ([]string, error) {
	enabled, err := normalizeWildcardDomains(values)
	if err != nil {
		return nil, err
	}

	w.mu.RLock()
	available := append([]string(nil), w.availableExtraDomains...)
	w.mu.RUnlock()
	for _, domain := range enabled {
		if !slices.Contains(available, domain) {
			return nil, fmt.Errorf("wildcard domain %q is not available", domain)
		}
	}
	return enabled, nil
}

func (w *Wildcard) ReplaceEnabledExtraDomains(values []string) error {
	enabled, err := w.ValidateEnabledExtraDomains(values)
	if err != nil {
		return err
	}

	w.mu.Lock()
	w.enabledExtraDomains = enabled
	w.managedDomains = append([]string{w.primaryDomain}, enabled...)
	slices.SortFunc(w.managedDomains, func(left, right string) int {
		return len(right) - len(left)
	})
	w.mu.Unlock()
	return nil
}

func (w *Wildcard) IsEnabled() bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.enabled
}

func (w *Wildcard) Status() WildcardStatus {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return WildcardStatus{
		Enabled:               w.enabled,
		PrimaryDomain:         w.primaryDomain,
		AvailableExtraDomains: append([]string(nil), w.availableExtraDomains...),
		EnabledExtraDomains:   append([]string(nil), w.enabledExtraDomains...),
		AllowPublicAddresses:  w.allowPublicAddresses,
		TTL:                   w.ttl,
	}
}

func parseWildcardName(name string, domains []string) (netip.Addr, bool, error) {
	name = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(name)), ".")
	for _, domain := range domains {
		if name == domain {
			return netip.Addr{}, true, ErrInvalidWildcardName
		}
		suffix := "." + domain
		if !strings.HasSuffix(name, suffix) {
			continue
		}
		prefix := strings.TrimSuffix(name, suffix)
		address, err := parseWildcardAddress(prefix)
		if err != nil {
			return netip.Addr{}, true, err
		}
		return address.Unmap(), true, nil
	}
	return netip.Addr{}, false, nil
}

func parseWildcardAddress(prefix string) (netip.Addr, error) {
	labels := strings.Split(prefix, ".")
	if len(labels) >= 4 {
		if address, ok := parseDecimalIPv4(labels[len(labels)-4:]); ok {
			return address, nil
		}
	}

	lastLabel := labels[len(labels)-1]
	parts := strings.Split(lastLabel, "-")
	if len(parts) >= 4 {
		if address, ok := parseDecimalIPv4(parts[len(parts)-4:]); ok {
			return address, nil
		}
	}

	hexadecimal := lastLabel
	if separator := strings.LastIndexByte(lastLabel, '-'); separator >= 0 {
		hexadecimal = lastLabel[separator+1:]
	}
	if len(hexadecimal) == net.IPv4len*2 {
		decoded, err := hex.DecodeString(hexadecimal)
		if err == nil {
			var bytes [net.IPv4len]byte
			copy(bytes[:], decoded)
			return netip.AddrFrom4(bytes), nil
		}
	}

	for start := 0; start < len(lastLabel); start++ {
		if start > 0 && lastLabel[start-1] != '-' {
			continue
		}
		candidate := lastLabel[start:]
		if !strings.Contains(candidate, "-") {
			continue
		}
		address, err := netip.ParseAddr(strings.ReplaceAll(candidate, "-", ":"))
		if err == nil && address.Is6() {
			return address, nil
		}
	}
	return netip.Addr{}, ErrInvalidWildcardName
}

func parseDecimalIPv4(parts []string) (netip.Addr, bool) {
	if len(parts) != net.IPv4len {
		return netip.Addr{}, false
	}
	var bytes [net.IPv4len]byte
	for index, part := range parts {
		if part == "" || len(part) > 3 {
			return netip.Addr{}, false
		}
		for _, character := range part {
			if character < '0' || character > '9' {
				return netip.Addr{}, false
			}
		}
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 || value > 255 {
			return netip.Addr{}, false
		}
		bytes[index] = byte(value)
	}
	return netip.AddrFrom4(bytes), true
}

func isLocalWildcardAddress(address netip.Addr) bool {
	return address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast()
}

func normalizeWildcardDomains(values []string) ([]string, error) {
	domains := make([]string, 0, len(values))
	for _, value := range values {
		domain, err := normalizeWildcardDomain(value)
		if err != nil {
			return nil, err
		}
		if !slices.Contains(domains, domain) {
			domains = append(domains, domain)
		}
	}
	slices.Sort(domains)
	return domains, nil
}

func normalizeWildcardDomain(value string) (string, error) {
	domain := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(value)), ".")
	if domain == "" || !strings.Contains(domain, ".") || strings.Contains(domain, "*") {
		return "", fmt.Errorf("invalid wildcard domain %q", value)
	}
	if _, valid := dns.IsDomainName(dns.Fqdn(domain)); !valid {
		return "", fmt.Errorf("invalid wildcard domain %q", value)
	}
	return domain, nil
}
