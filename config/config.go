package config

import (
	"crypto/rand"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	str2duration "github.com/xhit/go-str2duration/v2"
)

const (
	DefaultBrowserRememberDays = 10
	MinBrowserRememberDays     = 1
	MaxBrowserRememberDays     = 30
	DefaultBlacklistRepository = "https://github.com/StevenBlack/hosts.git"
	DefaultBlacklistBranch     = "master"
	DefaultBlacklistFile       = "alternates/gambling/hosts"
	DefaultBlacklistSchedule   = "0 */6 * * *"
	DefaultDNSLogRetention     = "30d"
	MaximumDNSLogRetention     = 180 * 24 * time.Hour
	DefaultDiagnosticsAddress  = "127.0.0.1:6060"
)

var CtxKey = "values"

type CtxValue struct {
	RemoteAddr net.Addr
	Labels     []string
	Values     map[string]string
}

var mu sync.Mutex
var conf *Config

func GetRunningConfig() *Config {
	mu.Lock()
	defer mu.Unlock()
	if conf == nil {
		panic("Uninitialized config")
	}
	return conf
}

func SetRunningConfig(c *Config) {
	mu.Lock()
	defer mu.Unlock()
	conf = c

}

func Lock() {
	mu.Lock()
}

func Unlock() {
	mu.Unlock()
}

type Config struct {
	Timeout         int                `mapstructure:"timeout" yaml:"timeout,omitempty"`
	UpstreamTimeout int                `mapstructure:"upstream_timeout" yaml:"upstream_timeout,omitempty"`
	LogLevel        string             `mapstructure:"loglevel" yaml:"loglevel,omitempty"`
	VerifyTLS       bool               `mapstructure:"verify_tls" yaml:"verify_tls,omitempty"`
	EnableAPI       bool               `mapstructure:"enable_api" yaml:"enable_api,omitempty"`
	Upstreams       []string           `mapstructure:"upstreams" yaml:"upstreams,omitempty"`
	Cache           CacheConf          `mapstructure:"cache" yaml:"cache,omitempty"`
	CORS            CORSConf           `mapstructure:"cors" yaml:"cors,omitempty"`
	Blacklist       BlacklistConfig    `mapstructure:"blacklist" yaml:"blacklist,omitempty"`
	StaticResponse  StaticResponseConf `mapstructure:"static_response" yaml:"static_response,omitempty"`
	ZenMode         ZenModeConfig      `mapstructure:"zen_mode" yaml:"zen_mode,omitempty"`
	Database        DatabaseConf       `mapstructure:"database" yaml:"database,omitempty"`
	Diagnostics     DiagnosticsConf    `mapstructure:"diagnostics" yaml:"diagnostics"`
	DNSAccess       DNSAccessConf      `mapstructure:"dns_access" yaml:"dns_access"`
	Auth            AuthConf           `mapstructure:"auth" yaml:"auth"`
	DNSLog          DNSLogConf         `mapstructure:"dns_log" yaml:"dns_log,omitempty"`
	Tagger          TaggerConf         `mapstructure:"tagger" yaml:"tagger,omitempty"`
	Status          StatusConf         `mapstructure:"status" yaml:"status,omitempty"`
	StubResolver    StubResolverConf   `mapstructure:"stub_resolver" yaml:"stub_resolver,omitempty"`
	Client          Client             `mapstructure:"client" yaml:"client,omitempty"`
	Server          Server             `mapstructure:"server" yaml:"server,omitempty"`
}

type DatabaseConf struct {
	File string `mapstructure:"file" yaml:"file,omitempty"`
}

type DNSAccessConf struct {
	AllowedClientCIDRs       []string `mapstructure:"allowed_client_cidrs" yaml:"allowed_client_cidrs"`
	ClientQueriesPerSecond   int      `mapstructure:"client_queries_per_second" yaml:"client_queries_per_second"`
	ClientBurst              int      `mapstructure:"client_burst" yaml:"client_burst"`
	GlobalResponsesPerSecond int      `mapstructure:"global_responses_per_second" yaml:"global_responses_per_second"`
	GlobalResponseBurst      int      `mapstructure:"global_response_burst" yaml:"global_response_burst"`
	MaxConcurrentUpstreams   int      `mapstructure:"max_concurrent_upstreams" yaml:"max_concurrent_upstreams"`
	MaxTrackedClients        int      `mapstructure:"max_tracked_clients" yaml:"max_tracked_clients"`
	ClientIdleTimeout        string   `mapstructure:"client_idle_timeout" yaml:"client_idle_timeout"`
}

type AuthConf struct {
	Issuer                 string          `mapstructure:"issuer" yaml:"issuer"`
	BearerAudience         string          `mapstructure:"bearer_audience" yaml:"bearer_audience"`
	ActiveKey              SigningKeyConf  `mapstructure:"active_key" yaml:"active_key"`
	PreviousKey            SigningKeyConf  `mapstructure:"previous_key" yaml:"previous_key"`
	PreviousKeyAcceptUntil string          `mapstructure:"previous_key_accept_until" yaml:"previous_key_accept_until"`
	Browser                BrowserAuthConf `mapstructure:"browser" yaml:"browser"`
}

type BrowserAuthConf struct {
	RememberDays int `mapstructure:"remember_days" yaml:"remember_days"`
}

func Validate(c *Config) error {
	if c == nil {
		return fmt.Errorf("configuration must not be nil")
	}
	if days := c.Auth.Browser.RememberDays; days < MinBrowserRememberDays || days > MaxBrowserRememberDays {
		return fmt.Errorf(
			"auth.browser.remember_days must be between %d and %d",
			MinBrowserRememberDays,
			MaxBrowserRememberDays,
		)
	}
	if c.DNSLog.Enabled || strings.TrimSpace(c.DNSLog.Purge) != "" {
		if _, err := ParseDNSLogRetention(c.DNSLog.Purge); err != nil {
			return err
		}
	}
	privacy := c.DNSLog.Pseudonymization
	if privacy.Domains || privacy.Clients {
		if strings.TrimSpace(privacy.KeyFile) == "" && strings.TrimSpace(privacy.KeyEnvironment) == "" {
			return fmt.Errorf("dns_log.pseudonymization requires key_file or key_environment")
		}
		if strings.ContainsRune(privacy.KeyEnvironment, '=') {
			return fmt.Errorf("dns_log.pseudonymization.key_environment must be an environment variable name")
		}
	}
	if c.Diagnostics.MetricsEnabled || c.Diagnostics.PProfEnabled {
		if err := ValidateDiagnosticsAddress(c.Diagnostics.ListenAddr); err != nil {
			return err
		}
	}
	return nil
}

func ParseDNSLogRetention(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("dns_log.purge must not be empty")
	}
	duration, err := str2duration.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("dns_log.purge must be a valid duration: %w", err)
	}
	if duration <= 0 {
		return 0, fmt.Errorf("dns_log.purge must be greater than zero")
	}
	if duration > MaximumDNSLogRetention {
		return 0, fmt.Errorf("dns_log.purge must not exceed 180 days")
	}
	return duration, nil
}

func ValidateDiagnosticsAddress(value string) error {
	host, port, err := net.SplitHostPort(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("diagnostics.listen_addr must be an IP address and port: %w", err)
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return fmt.Errorf("diagnostics.listen_addr host must be a numeric IP address")
	}
	if address.IsUnspecified() {
		return fmt.Errorf("diagnostics.listen_addr must not use a wildcard address")
	}
	if !address.IsLoopback() && !address.IsGlobalUnicast() {
		return fmt.Errorf("diagnostics.listen_addr must use a loopback or unicast address")
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("diagnostics.listen_addr port must be between 1 and 65535")
	}
	return nil
}

type SigningKeyConf struct {
	ID          string `mapstructure:"id" yaml:"id"`
	Environment string `mapstructure:"environment" yaml:"environment"`
	File        string `mapstructure:"file" yaml:"file"`
	Value       string `mapstructure:"value" yaml:"value"`
}

type CORSConf struct {
	Enabled        bool     `mapstructure:"enabled" yaml:"enabled"`
	AllowedOrigins []string `mapstructure:"allowed_origins" yaml:"allowed_origins,omitempty"`
}

type TaggerConf struct {
	Enabled bool `mapstructure:"enabled" yaml:"enabled,omitempty"`
}

type DNSLogConf struct {
	Enabled          bool                       `mapstructure:"enabled" yaml:"enabled,omitempty"`
	Purge            string                     `mapstructure:"purge" yaml:"purge,omitempty"`
	Pseudonymization DNSLogPseudonymizationConf `mapstructure:"pseudonymization" yaml:"pseudonymization,omitempty"`
}

type DNSLogPseudonymizationConf struct {
	Domains        bool   `mapstructure:"domains" yaml:"domains,omitempty"`
	Clients        bool   `mapstructure:"clients" yaml:"clients,omitempty"`
	KeyFile        string `mapstructure:"key_file" yaml:"key_file,omitempty"`
	KeyEnvironment string `mapstructure:"key_environment" yaml:"key_environment,omitempty"`
}

type DiagnosticsConf struct {
	ListenAddr     string `mapstructure:"listen_addr" yaml:"listen_addr"`
	MetricsEnabled bool   `mapstructure:"metrics_enabled" yaml:"metrics_enabled"`
	PProfEnabled   bool   `mapstructure:"pprof_enabled" yaml:"pprof_enabled"`
}

type CacheConf struct {
	Enabled  bool     `mapstructure:"enabled" yaml:"enabled" json:"enabled"`
	Ttl      int      `mapstructure:"ttl" yaml:"ttl,omitempty" json:"ttl,omitempty"`
	Excludes []string `mapstructure:"excludes" yaml:"excludes,omitempty" json:"excludes,omitempty"`
}
type BlacklistConfig struct {
	Enabled            bool     `mapstructure:"enabled" yaml:"enabled"`
	File               string   `mapstructure:"file" yaml:"file"`
	ExternalFile       string   `mapstructure:"external_file" yaml:"external_file,omitempty"`
	ExternalRepo       string   `mapstructure:"external_repo" yaml:"external_repo,omitempty"`
	ExternalRepoBranch string   `mapstructure:"external_repo_branch" yaml:"external_repo_branch,omitempty"`
	ExternalPullPeriod string   `mapstructure:"external_pull_period" yaml:"external_pull_period,omitempty"`
	Excludes           []string `mapstructure:"excludes" yaml:"excludes"`
	PersistedExcludes  []string `mapstructure:"-" yaml:"-"`
	ExtraHosts         []string `mapstructure:"-" yaml:"-"`
}

type StaticResponseConf struct {
	Enabled    bool              `mapstructure:"enabled" yaml:"enabled"`
	File       string            `mapstructure:"file" yaml:"file"`
	Labels     []string          `mapstructure:"labels" yaml:"labels,omitempty"`
	ExtraHosts map[string]string `mapstructure:"-" yaml:"-"`
}

type StatusConf struct {
	Enabled      bool `mapstructure:"enabled" yaml:"enabled,omitempty"`
	ExposeUptime bool `mapstructure:"expose_uptime" yaml:"expose_uptime,omitempty"`
	ExposeStats  bool `mapstructure:"expose_stats" yaml:"expose_stats,omitempty"`
}

type ZenModeConfig struct {
	Enabled           bool     `mapstructure:"enabled" yaml:"enabled,omitempty"`
	File              string   `mapstructure:"file" yaml:"file,omitempty"`
	Domains           []string `mapstructure:"domains" yaml:"domains,omitempty"`
	Excludes          []string `mapstructure:"excludes" yaml:"excludes,omitempty"`
	PersistedDomains  []string `mapstructure:"-" yaml:"-"`
	PersistedExcludes []string `mapstructure:"-" yaml:"-"`
	Labels            []string `mapstructure:"labels" yaml:"labels,omitempty"`
	Time              int      `mapstructure:"time" yaml:"time,omitempty"`
}

type StubResolverConf struct {
	Enabled bool     `mapstructure:"enabled" yaml:"enabled,omitempty"`
	Stubs   []string `mapstructure:"stubs" yaml:"stubs,omitempty"`
}

type Client struct {
	Server string `mapstructure:"server" yaml:"server,omitempty"`
	CAcert string `mapstructure:"ca_cert" yaml:"ca_cert,omitempty"`
	Token  string `mapstructure:"token" yaml:"token,omitempty"`
}

type Server struct {
	ListenAddr  string `mapstructure:"listen_addr" yaml:"listen_addr,omitempty"`
	APIAddr     string `mapstructure:"api_addr" yaml:"api_addr,omitempty"`
	APICertFile string `mapstructure:"api_cert_file" yaml:"api_cert_file,omitempty"`
	APIKeyFile  string `mapstructure:"api_key_file" yaml:"api_key_file,omitempty"`
	SigningKey  string `mapstructure:"signing_key" yaml:"signing_key,omitempty"`
	// SwaggerEnabled exposes the Swagger UI and raw Swagger/OpenAPI documents.
	SwaggerEnabled bool `mapstructure:"swagger_enabled" yaml:"swagger_enabled,omitempty"`
}

func GenKey() *[]byte {
	key := make([]byte, 64)
	_, err := rand.Read(key)
	if err != nil {
		panic(err)
	}
	return &key
}
