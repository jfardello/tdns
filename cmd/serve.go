package cmd

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-co-op/gocron"
	"github.com/jfardello/tdns/sched"
	"github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/server"
	"github.com/miekg/dns"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	upstream        []string
	stubs           []string
	hostFile        string
	bholeFile       string
	zenFile         string
	zenTime         int
	blue            = "\033[34m"
	reset           = "\033[0m"
	DefaultUpstream = "tls://1.1.1.1:853#cloudflare-dns.com"
	timeout         int
	upstreamTimeout int
)

// serveCmd represents the serve command.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start tls-dns forwarder",
	Long: `TDNS is a TLS dns forwarder that accepts plain DNS calls locally and forwards 
	queries to different upstreams based on its routing configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		setPersistentOps()
		initConfig()
		run()
	},
}

func initConfig() {
	logger := log.GetLogger("serve", "InintFlags")
	viper.SetConfigName("tdns")
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("tdns")
	viper.AutomaticEnv()
	if configFile == "" {
		viper.AddConfigPath("/etc/tdns/")
		viper.AddConfigPath("$HOME/.config/tdns")
		viper.AddConfigPath(".")
	} else {
		viper.SetConfigFile(configFile)
	}

	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {

			logger.Infof("config file not found %v", err)
			return
		}
	}
	logger.Infof("Loaded config file %s", viper.ConfigFileUsed())

	c := &config.Config{}
	err := viper.Unmarshal(c)
	if err != nil {
		panic(err)
	}
	config.SetRunningConfig(c)

}

func init() {

	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringSliceVarP(&upstream, "upstream", "u", []string{DefaultUpstream}, "default upstream")
	serveCmd.PersistentFlags().StringSliceVarP(&stubs, "stub", "s", []string{}, "Stubs servers for domains ex: domain.tld,udp://8.8.8.8")
	serveCmd.PersistentFlags().StringVarP(&hostFile, "hosts", "f", "", "Respond with Anchor Resource sets from this file.")
	serveCmd.PersistentFlags().StringVarP(&bholeFile, "blackhole", "b", "", "Black hole list file to filter ads & tracking systems.")
	serveCmd.PersistentFlags().StringVarP(&zenFile, "zenfile", "z", "", "Temporarily filter domains from this file.")
	serveCmd.PersistentFlags().IntVarP(&zenTime, "zentime", "T", 20, "Zen mode session time (in minutes).")
	serveCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 1000, "Global timeout for forwarding DNS requests")
	serveCmd.PersistentFlags().IntVarP(&upstreamTimeout, "upstreamtimeout", "U", 300, "Upstream timeout for forwarding DNS requests")

	//Viper will try pflags, environment variables and config file, in that order, default values
	//are mapped to oflags if they exist, or just viper default in case there is no config option
	//defined

	viper.SetDefault("upstreams", []string{DefaultUpstream})
	_ = viper.BindPFlag("upstreams", serveCmd.PersistentFlags().Lookup("upstream"))
	viper.SetDefault("stubs.config.stubs", serveCmd.PersistentFlags().Lookup("stub").DefValue)
	_ = viper.BindPFlag("stubs.config.stubs", serveCmd.PersistentFlags().Lookup("stub"))
	viper.SetDefault("static.config.file", serveCmd.PersistentFlags().Lookup("hosts").DefValue)
	_ = viper.BindPFlag("static_response_file", serveCmd.PersistentFlags().Lookup("hosts"))
	viper.SetDefault("blackhole_file", serveCmd.PersistentFlags().Lookup("blackhole").DefValue)
	_ = viper.BindPFlag("blackhole_file", serveCmd.PersistentFlags().Lookup("blackhole"))
	viper.SetDefault("zenmode.config.file", serveCmd.PersistentFlags().Lookup("zenfile").DefValue)
	viper.SetDefault("zenmode.config.time", serveCmd.PersistentFlags().Lookup("zentime").DefValue)
	_ = viper.BindPFlag("zenmode.config.time", serveCmd.PersistentFlags().Lookup("zentime"))
	_ = viper.BindPFlag("zenmode.config.file", serveCmd.PersistentFlags().Lookup("zenfile"))
	viper.SetDefault("timeout", serveCmd.PersistentFlags().Lookup("timeout").DefValue)
	viper.SetDefault("upstreamtimeout", serveCmd.PersistentFlags().Lookup("upstreamtimeout").DefValue)
	_ = viper.BindPFlag("timeout", serveCmd.PersistentFlags().Lookup("timeout"))
	_ = viper.BindPFlag("upstreamtimeout", serveCmd.PersistentFlags().Lookup("upstreamtimeout"))
	viper.SetDefault("status.enabled", true)
	viper.SetDefault("dns_log.enabled", true)
	viper.SetDefault("dns_log.file", "/var/lib/tdns/tdns.sqlite")
	viper.SetDefault("dns_log.purge", "180d")

	viper.SetDefault("loglevel", "INFO")

}

func run() {
	logger := log.GetLogger("newServer", "lookup")
	c := &config.Config{}
	err := viper.Unmarshal(c)
	if err != nil {
		logger.Fatal(err)
	}
	config.SetRunningConfig(c)

	if !verbose {
		if strings.ToUpper(c.LogLevel) == "INFO" {
			log.SetLevel(logrus.InfoLevel)
		} else if strings.ToUpper(c.LogLevel) == "DEBUG" {
			log.SetLevel(logrus.DebugLevel)
		} else if strings.ToUpper(c.LogLevel) == "ERROR" {
			log.SetLevel(logrus.ErrorLevel)
		}
	}
	fmt.Print(blue + `
   __      __          
  / /_____/ /___  _____
 / __/ __  / __ \/ ___/
/ /_/ /_/ / / / (__  ) 
\__/\__,_/_/ /_/____/  
`)

	fmt.Printf("\nVersion   : %s\n", *ver)
	fmt.Printf("Build date: %s\n", *compiledate)
	fmt.Printf("Git commit: %s\n\n"+reset, *gitcommit)

	if err != nil {
		logger.Fatal(err)
	}
	newServer := server.NewServer(
		server.WithStaticResponse(),
		server.WithUpstreams(c.Upstreams, c.Timeout, c.UpstreamTimeout),
		server.WithStubs(c.StubResolver.Stubs, c.Timeout, c.UpstreamTimeout),
		server.WithBHoleList(),
		server.WithCacheGet(),
		server.WithCacheSet(),
		server.WithZenPlugin(),
		server.WithStatus(),
		server.WithDNSLog(),
		server.WithTagger(),
	)

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		switch r.Opcode {
		case dns.OpcodeQuery:
			m, err := newServer.Handler(r, w.RemoteAddr())
			if err != nil {
				logger.Errorf("Failed lookup for %s with error: %s\n", r, err.Error())
				//dns.HandleFailed(w, r)
				m := new(dns.Msg)
				m.SetRcode(r, dns.RcodeServerFailure)
				// does not matter if this WriteMsg call fails
				_ = w.WriteMsg(m)
				logger.Error(err)
				return
			}
			if len(m.Answer) > 0 {
				pattern := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
				ipAddress := pattern.FindAllString(m.Answer[0].String(), -1)

				if len(ipAddress) > 0 {
					logger.Debugf("Lookup for %s with ip %s\n", m.Answer[0].Header().Name, ipAddress[0])
				} else {
					logger.Debugf("Lookup for %s with response %s\n", m.Answer[0].Header().Name, m.Answer[0])
				}
			}
			m.SetReply(r)
			err = w.WriteMsg(m)
			if err != nil {
				logger.Error(err)
			}
		}
	})

	go func() {
		api.Serve(newServer)
	}()

	if len(sched.TaskRegistry) > 0 {
		scheduler := gocron.NewScheduler(time.UTC)
		for _, task := range sched.TaskRegistry {
			_, err = sched.AddCron(scheduler, task.Expr, task.Fn)
			if err != nil {
				//ToDo: check all the Fatal calls that could be Error
				logger.Fatal(err)
			}
		}
		scheduler.StartAsync()
	}

	srv := &dns.Server{Addr: c.Server.ListenAddr, Net: "udp"}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			logger.Fatalf("Failed to set udp listener %s\n", err.Error())
		}

	}()

	<-ctx.Done()
	//close the badger database
	logger.Info("Shutting down server...")
	err = srv.Shutdown()
	if err != nil {
		logger.Fatal(err)
	}
	if p, ok := newServer.Plugins["tagger"]; ok {
		if closer, ok := p.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				logger.Fatal(err)
			}
		}
	}
	if p, ok := newServer.Plugins["dnslog"]; ok {
		if stopper, ok := p.(interface{ Stop() }); ok {
			stopper.Stop()
		}
	}
	os.Exit(0)

}
