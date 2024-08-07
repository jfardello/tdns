/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
)

var (
	certHosts  []string
	basepath   string
	basename   string
	duration   time.Duration
	destinaton string
	dnsListen  string
	apiListen  string
)

var unitTemplate = `[Unit]
Description=tDNS service unit file.

[Service]
ExecStart={{.Path}} -c {{.ConfigFile}}

[Install]
WantedBy=multi-user.target`

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Genarate a sample configuration",
	Long: `Genarates a starting configuration for both, server and client with
	 self-signed certificates and a random signing key.`,
	Run: func(cmd *cobra.Command, args []string) {
		if destinaton == "" {
			fmt.Println("output-dir is a mandatory option.")
			cmd.Help()
			return
		}
		if _, err := os.Stat(destinaton); os.IsNotExist(err) {
			err := os.Mkdir(destinaton, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
		certname, keyname := config.Generate384Cert(destinaton, basename, duration, certHosts)

		WriteSampleConfig("tdns.yaml", certname, keyname)
		abs, _ := os.Executable()
		createUnit(abs, path.Join(basepath, "etc"), path.Join(destinaton, "tdns.service"))
	},
}

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.PersistentFlags().DurationVarP(&duration, "validfor", "v", 365*24*time.Hour, "Valid-for string ej 1y 2m 265d.")
	configCmd.PersistentFlags().StringSliceVarP(&certHosts, "hosts", "H", []string{"127.0.0.1", "localhost"}, "Certificate host.")
	configCmd.PersistentFlags().StringVarP(&basename, "basename", "b", "tdns_", "basename for the generated certificates.")
	configCmd.PersistentFlags().StringVarP(&basepath, "basepath", "p", "/etc/tdns", "basepath for the generated config.")
	configCmd.PersistentFlags().StringVarP(&destinaton, "output-dir", "o", "", "basepath for the generated config.")
	configCmd.PersistentFlags().StringVarP(&dnsListen, "listendns", "l", ":53", "Listen addr for DNS")
	configCmd.PersistentFlags().StringVarP(&apiListen, "listenapi", "a", ":8443", "Listen addr for rest API")

}

func WriteSampleConfig(fname, cert, key string) {

	logger := log.GetLogger("config", "writesampleconfig")
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

	yamlOut, err := os.Create(path.Join(destinaton, fname))
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
		Timeout:            1000,
		VerifyTLS:          true,
		Upstreams:          []string{"tls://1.1.1.1:853#cloudflare-dns.com", "tls://1.0.0.1:853#cloudflare-dns.com"},
		BlackHole:          true,
		BlackHoleFile:      path.Join(basepath, "bhole.hosts"),
		StaticResponse:     true,
		StaticReposnsefile: path.Join(basepath, "static.hosts"),
		ZenMode:            true,
		ZenModeDomains:     []string{"facebook.com", "x.com", "instagram.com"},
		ZenModeTime:        20,
		StubResolver:       true,
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
