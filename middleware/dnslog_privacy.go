package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

const (
	dnsLogPrivacyPlainMode   = "plain"
	dnsLogPrivacyHMACMode    = "hmac-sha256-v1"
	dnsLogDomainContext      = "tdns/dnslog/domain/v1"
	dnsLogClientContext      = "tdns/dnslog/client/v1"
	dnsLogDomainToken        = "h1d_"
	dnsLogClientToken        = "h1c_"
	dnsLogMinimumKeyBytes    = 32
	dnsLogPrivacyStateUpsert = `
INSERT INTO dnslog_privacy_state (singleton, domain_mode, client_mode, key_fingerprint)
VALUES (1, ?, ?, ?)
ON CONFLICT(singleton) DO UPDATE SET
  domain_mode = excluded.domain_mode,
  client_mode = excluded.client_mode,
  key_fingerprint = excluded.key_fingerprint`
)

type DNSLogPrivacyStatus struct {
	DomainsPseudonymized bool
	ClientsPseudonymized bool
	KeyConfigured        bool
	RequiresClear        bool
	Reason               string
}

type dnsLogPseudonymizer struct {
	key         []byte
	fingerprint string
}

func newDNSLogPseudonymizer(conf config.DNSLogPseudonymizationConf) (*dnsLogPseudonymizer, error) {
	if !conf.Domains && !conf.Clients {
		return nil, nil
	}
	key, err := loadDNSLogPseudonymizationKey(conf)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(key)
	return &dnsLogPseudonymizer{
		key:         key,
		fingerprint: base64.RawURLEncoding.EncodeToString(digest[:]),
	}, nil
}

func loadDNSLogPseudonymizationKey(conf config.DNSLogPseudonymizationConf) ([]byte, error) {
	encoded := ""
	if environment := strings.TrimSpace(conf.KeyEnvironment); environment != "" {
		if value, exists := os.LookupEnv(environment); exists {
			encoded = strings.TrimSpace(value)
			if encoded == "" {
				return nil, fmt.Errorf("DNS-log pseudonymization environment variable %s is empty", environment)
			}
		}
	}
	if encoded == "" && strings.TrimSpace(conf.KeyFile) != "" {
		path := strings.TrimSpace(conf.KeyFile)
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("inspect DNS-log pseudonymization key file: %w", err)
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("DNS-log pseudonymization key file %s is not a regular file", path)
		}
		if info.Mode().Perm()&0o037 != 0 {
			return nil, fmt.Errorf(
				"DNS-log pseudonymization key file %s permissions %04o allow group write/execute or other access",
				path,
				info.Mode().Perm(),
			)
		}
		value, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read DNS-log pseudonymization key file: %w", err)
		}
		encoded = strings.TrimSpace(string(value))
	}
	if encoded == "" {
		return nil, errors.New("DNS-log pseudonymization key is not available")
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("decode base64 DNS-log pseudonymization key: %w", err)
	}
	if len(key) < dnsLogMinimumKeyBytes {
		return nil, fmt.Errorf("DNS-log pseudonymization key contains %d decoded bytes, minimum is %d", len(key), dnsLogMinimumKeyBytes)
	}
	return key, nil
}

func (p *dnsLogPseudonymizer) domain(value string) string {
	return p.token(dnsLogDomainContext, dnsLogDomainToken, dns.CanonicalName(strings.TrimSpace(value)))
}

func (p *dnsLogPseudonymizer) client(value string) (string, error) {
	value = strings.TrimSpace(value)
	if validDNSLogToken(value, dnsLogClientToken) {
		return value, nil
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return "", fmt.Errorf("invalid address: %s", value)
	}
	return p.token(dnsLogClientContext, dnsLogClientToken, ip.String()), nil
}

func (p *dnsLogPseudonymizer) token(context, prefix, value string) string {
	mac := hmac.New(sha256.New, p.key)
	_, _ = mac.Write([]byte(context))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(value))
	digest := mac.Sum(nil)
	return prefix + base64.RawURLEncoding.EncodeToString(digest)
}

func validDNSLogToken(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	digest, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(value, prefix))
	return err == nil && len(digest) == sha256.Size
}

func privacyMode(enabled bool) string {
	if enabled {
		return dnsLogPrivacyHMACMode
	}
	return dnsLogPrivacyPlainMode
}

func (cs *DNSLog) configurePrivacy(conf config.DNSLogPseudonymizationConf) error {
	pseudonymizer, err := newDNSLogPseudonymizer(conf)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrDNSLogPrivacyConfig, err)
	}
	desiredDomainMode := privacyMode(conf.Domains)
	desiredClientMode := privacyMode(conf.Clients)
	desiredFingerprint := ""
	if pseudonymizer != nil {
		desiredFingerprint = pseudonymizer.fingerprint
	}

	conn := cs.se.GetConn()

	var logRows, hostRows int
	if err := conn.QueryRow("SELECT COUNT(*) FROM tdnslog").Scan(&logRows); err != nil {
		cs.se.FreeConn(conn)
		return fmt.Errorf("count DNS-log rows: %w", err)
	}
	if err := conn.QueryRow("SELECT COUNT(*) FROM hosts").Scan(&hostRows); err != nil {
		cs.se.FreeConn(conn)
		return fmt.Errorf("count DNS-log aliases: %w", err)
	}

	storedDomainMode := dnsLogPrivacyPlainMode
	storedClientMode := dnsLogPrivacyPlainMode
	storedFingerprint := ""
	err = conn.QueryRow(`
SELECT domain_mode, client_mode, key_fingerprint
FROM dnslog_privacy_state
WHERE singleton = 1`).Scan(&storedDomainMode, &storedClientMode, &storedFingerprint)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		cs.se.FreeConn(conn)
		return fmt.Errorf("read DNS-log privacy state: %w", err)
	}
	cs.se.FreeConn(conn)

	domainData := logRows > 0
	clientData := logRows > 0 || hostRows > 0
	domainMismatch := storedDomainMode != desiredDomainMode && domainData
	clientMismatch := storedClientMode != desiredClientMode && clientData
	keyMismatch := storedFingerprint != desiredFingerprint &&
		((desiredDomainMode == dnsLogPrivacyHMACMode && domainData) ||
			(desiredClientMode == dnsLogPrivacyHMACMode && clientData))
	requiresClear := domainMismatch || clientMismatch || keyMismatch

	status := DNSLogPrivacyStatus{
		DomainsPseudonymized: conf.Domains,
		ClientsPseudonymized: conf.Clients,
		KeyConfigured:        pseudonymizer != nil,
		RequiresClear:        requiresClear,
	}
	if requiresClear {
		status.Reason = "existing DNS-log data or aliases use an incompatible privacy mode or key; clear DNS-log data before logging can resume"
		cs.setPrivacy(pseudonymizer, status)
		return nil
	}

	_, err = cs.se.SyncExec(dnsLogPrivacyStateUpsert,
		[]any{desiredDomainMode, desiredClientMode, desiredFingerprint},
	)
	if err != nil {
		return fmt.Errorf("write DNS-log privacy state: %w", err)
	}
	cs.setPrivacy(pseudonymizer, status)
	return nil
}

func privacyStateArgs(status DNSLogPrivacyStatus, pseudonymizer *dnsLogPseudonymizer) []any {
	fingerprint := ""
	if pseudonymizer != nil {
		fingerprint = pseudonymizer.fingerprint
	}
	return []any{
		privacyMode(status.DomainsPseudonymized),
		privacyMode(status.ClientsPseudonymized),
		fingerprint,
	}
}

func (cs *DNSLog) setPrivacy(pseudonymizer *dnsLogPseudonymizer, status DNSLogPrivacyStatus) {
	cs.privacyMu.Lock()
	defer cs.privacyMu.Unlock()
	cs.pseudonymizer = pseudonymizer
	cs.privacy = status
}

func (cs *DNSLog) PrivacyStatus() DNSLogPrivacyStatus {
	status, _ := cs.privacySettings()
	return status
}

func (cs *DNSLog) privacySettings() (DNSLogPrivacyStatus, *dnsLogPseudonymizer) {
	cs.privacyMu.RLock()
	defer cs.privacyMu.RUnlock()
	return cs.privacy, cs.pseudonymizer
}

func (cs *DNSLog) transformClient(value string) (string, error) {
	status, pseudonymizer := cs.privacySettings()
	if !status.ClientsPseudonymized {
		ip := net.ParseIP(strings.TrimSpace(value))
		if ip == nil {
			return "", fmt.Errorf("invalid address: %s", value)
		}
		return ip.String(), nil
	}
	if pseudonymizer == nil {
		return "", errors.New("DNS-log client pseudonymization key is not configured")
	}
	return pseudonymizer.client(value)
}
