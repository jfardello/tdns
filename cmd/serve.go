package cmd

import (
	"regexp"

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
	timeOut         int
	DefaultUpstream string = "tls://1.1.1.1:853#cloudflare-dns.com"
)

// serveCmd represents the serve command.
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start tls-dns forwarder",
	Long: `TDNS is a TLS dns forwarder that accepts plain DNS calls locally and forwards 
	queries to different upstreams based on its routing configuration.`,
	Run: func(cmd *cobra.Command, args []string) {
		initConfig()
		run()
	},
}

func initConfig() {
	logger := log.GetLogger("serve", "init")
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
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {

			logger.Infof("config file not found %v", err)
			return
		} else {
			logger.Error(err)
			panic(err)

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
	serveCmd.PersistentFlags().IntVarP(&timeOut, "timeout", "t", 1000, "Global timeout for forwarding DNS requests")

	//Viper will try pflags, environment variables and config file, in that order, default values
	//are mapped to oflags if they exist, or just viper default in case there is no config option
	//defined

	viper.SetDefault("upstreams", []string{DefaultUpstream})
	_ = viper.BindPFlag("upstreams", serveCmd.PersistentFlags().Lookup("upstream"))
	viper.SetDefault("stubs", serveCmd.PersistentFlags().Lookup("stub").DefValue)
	_ = viper.BindPFlag("stubs", serveCmd.PersistentFlags().Lookup("stub"))
	viper.SetDefault("static_response_file", serveCmd.PersistentFlags().Lookup("hosts").DefValue)
	_ = viper.BindPFlag("static_response_file", serveCmd.PersistentFlags().Lookup("hosts"))
	viper.SetDefault("blackhole_file", serveCmd.PersistentFlags().Lookup("blackhole").DefValue)
	_ = viper.BindPFlag("blackhole_file", serveCmd.PersistentFlags().Lookup("blackhole"))
	viper.SetDefault("zenmode_file", serveCmd.PersistentFlags().Lookup("zenfile").DefValue)
	viper.SetDefault("zenmode_time", serveCmd.PersistentFlags().Lookup("zentime").DefValue)
	_ = viper.BindPFlag("zenmode_time", serveCmd.PersistentFlags().Lookup("zentime"))
	_ = viper.BindPFlag("zenmode_file", serveCmd.PersistentFlags().Lookup("zenfile"))

}

func run() {
	logger := log.GetLogger("server", "lookup")
	c := &config.Config{}
	err := viper.Unmarshal(c)
	if err != nil {
		logger.Fatal(err)
	}
	config.SetRunningConfig(c)

	server := server.NewServer(
		server.WithStaticResponse(c.StaticReposnsefile),
		server.WithUpstreams(c.Upstreams),
		server.WithStubs(c.StubResolverStubs),
		server.WithBHoleList(c.BlackHoleFile),
		server.WithCacheGet(),
		server.WithCacheSet(),
		server.WithZenPlugin(),
		server.WithStatus(),
	)

	dns.HandleFunc(".", func(w dns.ResponseWriter, r *dns.Msg) {
		switch r.Opcode {
		case dns.OpcodeQuery:
			m, err := server.Handler(r)
			if err != nil {
				logger.Errorf("Failed lookup for %s with error: %s\n", r, err.Error())

				dns.HandleFailed(w, r)
				m := new(dns.Msg)
				m.SetRcode(r, dns.RcodeServerFailure)
				// does not matter if this write fails
				w.WriteMsg(m)
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
		api.Serve(server)
	}()

	srv := &dns.Server{Addr: c.Server.ListenAddr, Net: "udp"}
	if err := srv.ListenAndServe(); err != nil {
		logger.Fatalf("Failed to set udp listener %s\n", err.Error())
	}

}
