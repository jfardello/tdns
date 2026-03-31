/*
Copyright © 2024 NAME HERE <jmfardello@gmail.com>
*/
package cmd

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
)

var (
	certHosts   []string
	basepath    string
	basename    string
	duration    time.Duration
	destination string
	dnsListen   string
	apiListen   string
)

var unitTemplate = `[Unit]
Description=tDNS service unit file.

[Service]
ExecStart={{.Path}} serve -c {{.ConfigFile}}
User=root
[Install]
WantedBy=multi-user.target`

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Genarate a sample configuration",
	Long: `Genarates a starting configuration for both, server and client with
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
		if _, err := os.Stat(destination); os.IsNotExist(err) {
			err := os.Mkdir(destination, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
		certname, keyname := config.Generate384Cert(destination, basename, duration, certHosts)

		certname = strings.Replace(certname, destination, basepath, 1)
		keyname = strings.Replace(keyname, destination, basepath, 1)

		WriteSampleConfig("tdns.yaml", certname, keyname)
		abs, _ := os.Executable()
		createUnit(abs, path.Join(basepath, "etc"), path.Join(destination, "tdns.service"))
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
	configCmd.PersistentFlags().StringVarP(&dnsListen, "listendns", "l", ":53", "Listen addr for DNS")
	configCmd.PersistentFlags().StringVarP(&apiListen, "listenapi", "a", ":8443", "Listen addr for rest API")

	viper.SetDefault("server.listen_addr", configCmd.PersistentFlags().Lookup("listendns").DefValue)
	_ = viper.BindPFlag("server.listen_addr", configCmd.PersistentFlags().Lookup("listenaddr"))

}

func WriteSampleConfig(fname, cert, key string) {
	logger := log.GetLogger("config", "WriteSampleConfig")
	c := newConf()
	k := config.GenKey()
	c.Server.SigningKey = base64.StdEncoding.EncodeToString(*k)

	config.SetRunningConfig(c)
	t, err := api.IssueToken(365, "admin")
	if err != nil {
		logger.Fatalf("Error generating token:%v", err)
	}
	c.Client.Token = t
	c.Server.APICertFile = cert
	c.Server.APIKeyFile = key
	c.Client.CAcert = cert

	yamlOut, err := os.Create(path.Join(destination, fname))
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

}

func newConf() *config.Config {
	c := &config.Config{
		Timeout:         1000,
		UpstreamTimeout: 300,
		VerifyTLS:       true,
		Upstreams:       []string{"tls://1.1.1.1:853#cloudflare-dns.com", "tls://1.0.0.1:853#cloudflare-dns.com"},
		Blacklist: config.BlacklistConfig{
			Enabled: true,
			File:    "fixtures/bhole_testfile",
		},
		StaticResponse: config.StaticResponseConf{
			Enabled: true,
			File:    "fixtures/hosts_testfile",
		},
		ZenMode: config.ZenModeConfig{
			Enabled: true,
			Domains: []string{"x.com"},
			Time:    20,
		},
		StubResolver: config.StubResolverConf{Enabled: true,
			Stubs: []string{"google.com,udp://8.8.8.8"},
		},
		Database: config.DatabaseConf{
			File: "/var/lib/tdns/tdns.sqlite",
		},
		DNSLog: config.DNSLogConf{
			Enabled: true,
			Purge:   "180d",
		},
		Tagger: config.TaggerConf{
			Enabled: true,
		},
		Server: config.Server{
			ListenAddr:  dnsListen,
			APIAddr:     apiListen,
			APICertFile: "",
			APIKeyFile:  "",
			SigningKey:  "",
		},
		Client: config.Client{
			Server: "https://localhost:8443",
			Token:  "",
			CAcert: "",
		},
		Status: config.StatusConf{
			Enabled:      true,
			ExposeUptime: true,
			ExposeStats:  true,
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
	unitOut, err := os.Create(unitpath)
	if err != nil {
		logger.Fatalf("Failed to open %s for writing: %v", unitpath, err)
	}
	err = t.Execute(unitOut, data)
	if err != nil {
		logger.Fatalf("Error writing unit file: %v", err)
	}

}
