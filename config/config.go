package config

import (
	"crypto/rand"
	"encoding/base64"
	"sync"

	"github.com/jfardello/tdns/log"
)

var mu sync.Mutex
var conf *Config

func GetRunningConfig() *Config {
	mu.Lock()
	defer mu.Unlock()
	if conf == nil {
		panic("Unitialized config")
	}
	conf.Server.loadSigningKey()
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
	Timeout            int      `mapstructure:"timeout" yaml:"timeout,omitempty"`
	VerifyTLS          bool     `mapstructure:"verify_tls" yaml:"verify_tls,omitempty"`
	EnablAPI           bool     `mapstructure:"enable_api" yaml:"enable_api,omitempty"`
	Upstreams          []string `mapstructure:"upstreams" yaml:"upstreams,omitempty"`
	BlackHole          bool     `mapstructure:"enable_blackhole" yaml:"enable_blackhole,omitempty"`
	BlackHoleFile      string   `mapstructure:"blackhole_file" yaml:"blackhole_file,omitempty"`
	BlackHoleExempt    []string `mapstructure:"blackhole_exempt" yaml:"blackhole_exempt,omitempty"`
	StaticResponse     bool     `mapstructure:"enable_static_response" yaml:"enable_static_response,omitempty"`
	StaticReposnsefile string   `mapstructure:"static_response_file" yaml:"static_response_file,omitempty"`
	ZenMode            bool     `mapstructure:"enable_zenmode" yaml:"enable_zenmode,omitempty"`
	ZenModeFile        string   `mapstructure:"zenmode_file" yaml:"zenmode_file,omitempty"`
	ZenModeDomains     []string `mapstructure:"zenmode_domains" yaml:"zenmode_domains,omitempty"`
	ZenModeTime        int      `mapstructure:"zenmode_time" yaml:"zenmode_time,omitempty"`
	Status             bool     `mapstructure:"enable_status" yaml:"enable_status,omitempty"`
	StubResolver       bool     `mapstructure:"enable_stubs" yaml:"enable_stubs,omitempty"`
	StubResolverStubs  []string `mapstructure:"stubs" yaml:"stubs,omitempty"`
	Client             Client   `mapstructure:"client" yaml:"client,omitempty"`
	Server             Server   `mapstructure:"server" yaml:"server,omitempty"`
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
	signingKey  []byte
}

func (s *Server) loadSigningKey() {
	logger := log.GetLogger("config", "loadSigningKey")
	var err error
	if s.SigningKey == "" {
		s.signingKey = *GenKey()
		sk := base64.StdEncoding.EncodeToString(s.signingKey)
		logger.Infof("Generated a temporal key for testing purposes (%s), please generate a persistent one.", sk)
		return
	}
	s.signingKey, err = base64.StdEncoding.DecodeString(s.SigningKey)
	if err != nil {
		logger.Fatalf("Cannot load key:, %s", err.Error())
	}

}

func (s *Server) GetSigningKey() []byte {
	if len(s.signingKey) == 0 {
		s.loadSigningKey()
	}
	return s.signingKey

}

func GenKey() *[]byte {
	key := make([]byte, 64)
	_, err := rand.Read(key)
	if err != nil {
		panic(err)
	}
	return &key
}
