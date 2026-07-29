/*
Copyright © 2024 NAME HERE <jmfardello@gmail.com>
*/
package cmd

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html/template"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/log"
)

const (
	defaultDataPath              = "/var/lib/tdns"
	defaultBootstrapTokenTTLDays = 30
)

var (
	certHosts           []string
	basepath            string
	basename            string
	duration            time.Duration
	destination         string
	dataPath            string
	dnsListen           string
	apiListen           string
	generateSystemdUnit bool
)

var unitTemplate = `[Unit]
Description=TDNS DNS resolver
Documentation=https://git.kubewire.net/jfardello/tdns
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
ExecStart={{.Path}} serve -c {{.ConfigFile}}
User=tdns
Group=tdns
UMask=0077
WorkingDirectory=/var/lib/tdns
StateDirectory=tdns
StateDirectoryMode=0750
ConfigurationDirectory=tdns
ConfigurationDirectoryMode=0750
Restart=on-failure
RestartSec=5s

AmbientCapabilities=CAP_NET_BIND_SERVICE
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
LockPersonality=true
MemoryDenyWriteExecute=true
NoNewPrivileges=true
PrivateDevices=true
PrivateTmp=true
ProtectClock=true
ProtectControlGroups=true
ProtectHome=true
ProtectKernelLogs=true
ProtectKernelModules=true
ProtectKernelTunables=true
ProtectSystem=strict
ReadOnlyPaths=/etc/tdns
ReadWritePaths=/var/lib/tdns
RestrictAddressFamilies=AF_INET AF_INET6
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
`

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Generate a sample configuration",
	Long: `Generates a starting configuration for both, server and client with
		 self-signed certificates and a random signing key.`,
	Run: func(cmd *cobra.Command, args []string) {
		if destination == "" {
			fmt.Println("output-dir is a mandatory option.")
			err := cmd.Help()
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
			return
		}
		if info, err := os.Stat(destination); err != nil {
			if !os.IsNotExist(err) {
				panic(err)
			}
			if err := os.MkdirAll(destination, 0o750); err != nil {
				panic(err)
			}
		} else if !info.IsDir() {
			panic(fmt.Errorf("output path %s is not a directory", destination))
		}
		certname, keyname := config.Generate384Cert(destination, basename, duration, certHosts)

		certname = deploymentPath(certname, destination, basepath)
		keyname = deploymentPath(keyname, destination, basepath)

		WriteSampleConfig("tdns.yaml", certname, keyname)
		if generateSystemdUnit {
			abs, _ := os.Executable()
			createUnit(abs, path.Join(basepath, "tdns.yaml"), path.Join(destination, "tdns.service"))
		}
		fmt.Printf("Sample config written to %s.\n", path.Join(destination, "tdns.yaml"))
		fmt.Printf("Runtime data such as the blacklist and database files will be stored in %s.\n", dataPath)
		fmt.Printf("Create %s with ownership restricted to the TDNS service account before starting the server.\n", dataPath)
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.PersistentFlags().DurationVarP(&duration, "validfor", "f", 365*24*time.Hour, "Valid-for string ej 1y 2m 265d.")
	//configCmd.PersistentFlags().DurationVarP(&duration, "validfor", "v", 365*24*time.Hour, "Valid-for string ej 1y 2m 265d.")
	configCmd.PersistentFlags().StringSliceVarP(&certHosts, "hosts", "H", []string{"127.0.0.1", "localhost"}, "Certificate host.")
	configCmd.PersistentFlags().StringVarP(&basename, "basename", "b", "tdns_", "basename for the generated certificates.")
	configCmd.PersistentFlags().StringVarP(&basepath, "basepath", "p", "/etc/tdns", "basepath for the generated config.")
	configCmd.PersistentFlags().StringVarP(&destination, "output-dir", "o", "", "output for the generated config.")
	configCmd.PersistentFlags().StringVar(&dataPath, "data-path", defaultDataPath, "runtime data directory used in the generated config.")
	configCmd.PersistentFlags().StringVarP(&dnsListen, "listendns", "l", "127.0.0.1:53", "Listen addr for DNS")
	configCmd.PersistentFlags().StringVarP(&apiListen, "listenapi", "a", "127.0.0.1:8443", "Listen addr for rest API")
	configCmd.PersistentFlags().BoolVar(&generateSystemdUnit, "systemd-unit", true, "generate a systemd service unit.")

	viper.SetDefault("server.listen_addr", configCmd.PersistentFlags().Lookup("listendns").DefValue)
	_ = viper.BindPFlag("server.listen_addr", configCmd.PersistentFlags().Lookup("listendns"))

}

func WriteSampleConfig(fname, cert, key string) {

	logger := log.GetLogger("config", "WriteSampleConfig")
	c := newConf()
	k := config.GenKey()
	c.Auth.ActiveKey.ID = generatedKeyID()
	c.Auth.ActiveKey.Value = base64.StdEncoding.EncodeToString(*k)

	config.SetRunningConfig(c)
	issuanceConfig := c.Auth
	issuanceConfig.ActiveKey.Environment = ""
	authManager, err := auth.NewManager(issuanceConfig, "", auth.Options{})
	if err != nil {
		logger.Fatalf("Error loading generated signing key:%v", err)
	}
	t, err := authManager.IssueBearer(
		"admin",
		auth.ScopeWrite,
		defaultBootstrapTokenTTLDays*24*time.Hour,
	)
	if err != nil {
		logger.Fatalf("Error generating token:%v", err)
	}
	c.Client.Token = t
	c.Server.APICertFile = cert
	c.Server.APIKeyFile = key
	c.Client.CAcert = cert

	yamlOut, err := os.OpenFile(path.Join(destination, fname), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		logger.Fatalf("Failed to open %s for writing: %v", path.Join(basepath, fname), err)
	}
	out, err := yaml.Marshal(c)
	if err != nil {
		logger.Fatalf("Error marshalling yaml config: %v", err)
	}
	_, err = yamlOut.Write(out)
	if err != nil {
		logger.Fatalf("Error writing yaml config: %v", err)
	}
	if err := yamlOut.Close(); err != nil {
		logger.Fatalf("Error closing yaml config: %v", err)
	}
	err = os.WriteFile(path.Join(destination, "hostsfile_list"), []byte("#127.0.0.1 foo.a.net"), 0644)
	if err != nil {
		logger.Fatalf("Error writing sample hostfile: %v", err)

	}

}

func deploymentPath(generatedPath, outputDir, runtimeDir string) string {
	relative, err := filepath.Rel(outputDir, generatedPath)
	if err != nil || relative == "." || relative == ".." ||
		strings.HasPrefix(relative, ".."+string(os.PathSeparator)) || filepath.IsAbs(relative) {
		return generatedPath
	}
	return filepath.Join(runtimeDir, relative)
}

func newConf() *config.Config {
	c := &config.Config{
		Timeout:         1000,
		UpstreamTimeout: 300,
		VerifyTLS:       true,
		Upstreams:       []string{"tls://1.1.1.1:853#cloudflare-dns.com", "tls://1.0.0.1:853#cloudflare-dns.com"},
		Cache: config.CacheConf{
			Enabled: true,
			Ttl:     5,
		},
		Blacklist: config.BlacklistConfig{
			Enabled: true,
			File:    path.Join(dataPath, "bhole_list"),
		},
		StaticResponse: config.StaticResponseConf{
			Enabled: true,
			File:    path.Join(basepath, "hostsfile_list"),
		},
		ZenMode: config.ZenModeConfig{
			Enabled: true,
			Domains: []string{"x.com"},
			Time:    20,
		},
		StubResolver: config.StubResolverConf{Enabled: true,
			Stubs: []string{"google.com,udp://8.8.8.8"},
		},
		CORS: config.CORSConf{
			Enabled:        false,
			AllowedOrigins: []string{"http://localhost:3000", "https://localhost:3000"},
		},
		Database: config.DatabaseConf{
			File: path.Join(dataPath, "tdns.sqlite"),
		},
		DNSAccess: config.DNSAccessConf{
			AllowedClientCIDRs:       []string{},
			ClientQueriesPerSecond:   100,
			ClientBurst:              200,
			GlobalResponsesPerSecond: 1000,
			GlobalResponseBurst:      2000,
			MaxConcurrentUpstreams:   128,
			MaxTrackedClients:        4096,
			ClientIdleTimeout:        "10m",
		},
		Auth: config.AuthConf{
			Issuer:         auth.DefaultIssuer,
			BearerAudience: auth.DefaultBearerAudience,
			ActiveKey: config.SigningKeyConf{
				Environment: "TDNS_AUTH_ACTIVE_KEY",
			},
			PreviousKey: config.SigningKeyConf{
				Environment: "TDNS_AUTH_PREVIOUS_KEY",
			},
		},
		DNSLog: config.DNSLogConf{
			Enabled: true,
			Purge:   "180d",
		},
		Tagger: config.TaggerConf{
			Enabled: true,
		},
		Server: config.Server{
			ListenAddr:     dnsListen,
			APIAddr:        apiListen,
			APICertFile:    "",
			APIKeyFile:     "",
			SigningKey:     "",
			PProfAddr:      "",
			SwaggerEnabled: false,
		},
		Client: config.Client{
			Server: "https://localhost:8443",
			Token:  "",
			CAcert: "",
		},
		Status: config.StatusConf{
			Enabled:      true,
			ExposeUptime: false,
			ExposeStats:  false,
		},
	}
	return c
}

func createUnit(tdnspath, cfgName, unitpath string) {
	logger := log.GetLogger("config", "createUnit")
	type T struct {
		Path       string
		ConfigFile string
	}
	t, err := template.New("unit").Parse(unitTemplate)
	if err != nil {
		panic(err)
	}
	data := &T{Path: tdnspath, ConfigFile: cfgName}
	unitOut, err := os.OpenFile(unitpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		logger.Fatalf("Failed to open %s for writing: %v", unitpath, err)
	}
	err = t.Execute(unitOut, data)
	if err != nil {
		logger.Fatalf("Error writing unit file: %v", err)
	}
	if err := unitOut.Close(); err != nil {
		logger.Fatalf("Error closing unit file: %v", err)
	}
}

func generatedKeyID() string {
	value := make([]byte, 8)
	if _, err := rand.Read(value); err != nil {
		panic(fmt.Errorf("generate signing key identifier: %w", err))
	}
	return "key-" + hex.EncodeToString(value)
}
