package config

import (
	"crypto/rand"
	"net"
	"sync"
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
	Issuer                 string         `mapstructure:"issuer" yaml:"issuer"`
	BearerAudience         string         `mapstructure:"bearer_audience" yaml:"bearer_audience"`
	ActiveKey              SigningKeyConf `mapstructure:"active_key" yaml:"active_key"`
	PreviousKey            SigningKeyConf `mapstructure:"previous_key" yaml:"previous_key"`
	PreviousKeyAcceptUntil string         `mapstructure:"previous_key_accept_until" yaml:"previous_key_accept_until"`
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
	Enabled bool   `mapstructure:"enabled" yaml:"enabled,omitempty"`
	Purge   string `mapstructure:"purge" yaml:"purge,omitempty"`
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
	PProfAddr   string `mapstructure:"pprof_addr" yaml:"pprof_addr,omitempty"`
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
