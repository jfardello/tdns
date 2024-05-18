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
	Timeout            int      `mapstructure:"timeout"`
	VerifyTLS          bool     `mapstructure:"verify_tls"`
	EnablAPI           bool     `mapstructure:"enable_api"`
	Upstreams          []string `mapstructure:"upstreams"`
	BlackHole          bool     `mapstructure:"enable_blackhole"`
	BlackHoleFile      string   `mapstructure:"blackhole_file"`
	BlackHoleExempt    []string `mapstructure:"blackhole_exempt"`
	StaticResponse     bool     `mapstructure:"enable_static_response"`
	StaticReposnsefile string   `mapstructure:"static_response_file"`
	ZenMode            bool     `mapstructure:"enable_zenmode"`
	ZenModeFile        string   `mapstructure:"zenmode_file"`
	ZenModeDomains     []string `mapstructure:"zenmode_domains"`
	ZenModeTime        int      `mapstructure:"zenmode_time"`
	StubResolver       bool     `mapstructure:"enable_stubs"`
	StubResolverStubs  []string `mapstructure:"stubs"`
	Client             Client   `mapstructure:"client"`
	Server             Server   `mapstructure:"server"`
}

type Client struct {
	Server string `mapstructure:"server"`
	CAcert string `mapstructure:"ca_cert"`
	Token  string `mapstructure:"token"`
}

type Server struct {
	ListenAddr  string `mapstructure:"listen_addr"`
	APIAddr     string `mapstructure:"api_addr"`
	APICertFile string `mapstructure:"api_cert_file"`
	APIKeyFile  string `mapstructure:"api_key_file"`
	SigningKey  string `mapstructure:"signing_key"`
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
