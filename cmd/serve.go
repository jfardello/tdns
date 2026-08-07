package cmd

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/jfardello/tdns/internal/browserauth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/jfardello/tdns/internal/diagnostics"
	"github.com/jfardello/tdns/internal/dnsserver"
	"github.com/jfardello/tdns/internal/httpapi"
	"github.com/jfardello/tdns/internal/overrides"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/sched"
	"github.com/jfardello/tdns/server"
	webui "github.com/jfardello/tdns/web"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	upstream        []string
	stubs           []string
	hostFile        string
	blacklistFile   string
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

	if err := readConfiguration(viper.GetViper()); err != nil {
		logger.Fatal(err)
	}
	if viper.ConfigFileUsed() == "" {
		logger.Info("config file not found; using command-line options and defaults")
		return
	}
	logger.Infof("Loaded config file %s", viper.ConfigFileUsed())

	c := &config.Config{}
	err := viper.Unmarshal(c)
	if err != nil {
		panic(err)
	}
	config.SetRunningConfig(c)

}

func readConfiguration(v *viper.Viper) error {
	if err := v.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			return nil
		}
		return fmt.Errorf("read configuration: %w", err)
	}
	return nil
}

func init() {

	rootCmd.AddCommand(serveCmd)
	serveCmd.PersistentFlags().StringSliceVarP(&upstream, "upstream", "u", []string{DefaultUpstream}, "default upstream")
	serveCmd.PersistentFlags().StringSliceVarP(&stubs, "stub", "s", []string{}, "Stubs servers for domains ex: domain.tld,udp://8.8.8.8")
	serveCmd.PersistentFlags().StringVarP(&hostFile, "hosts", "f", "", "Respond with Anchor Resource sets from this file.")
	serveCmd.PersistentFlags().StringVarP(&blacklistFile, "blacklist", "b", "", "Blacklist file to filter ads and tracking systems.")
	serveCmd.PersistentFlags().StringVarP(&zenFile, "zenfile", "z", "", "Temporarily filter domains from this file.")
	serveCmd.PersistentFlags().IntVarP(&zenTime, "zentime", "T", 20, "Zen mode session time (in minutes).")
	serveCmd.PersistentFlags().IntVarP(&timeout, "timeout", "t", 1000, "Global timeout for forwarding DNS requests")
	serveCmd.PersistentFlags().IntVarP(&upstreamTimeout, "upstreamtimeout", "U", 300, "Upstream timeout for forwarding DNS requests")

	//Viper will try pflags, environment variables and config file, in that order, default values
	//are mapped to pflags if they exist, or just viper default in case there is no config option
	//defined

	viper.SetDefault("upstreams", []string{DefaultUpstream})
	_ = viper.BindPFlag("upstreams", serveCmd.PersistentFlags().Lookup("upstream"))
	viper.SetDefault("stub_resolver.stubs", serveCmd.PersistentFlags().Lookup("stub").DefValue)
	_ = viper.BindPFlag("stub_resolver.stubs", serveCmd.PersistentFlags().Lookup("stub"))
	viper.SetDefault("static_response.file", serveCmd.PersistentFlags().Lookup("hosts").DefValue)
	_ = viper.BindPFlag("static_response.file", serveCmd.PersistentFlags().Lookup("hosts"))
	viper.SetDefault("blacklist.file", serveCmd.PersistentFlags().Lookup("blacklist").DefValue)
	_ = viper.BindPFlag("blacklist.file", serveCmd.PersistentFlags().Lookup("blacklist"))
	viper.SetDefault("zen_mode.file", serveCmd.PersistentFlags().Lookup("zenfile").DefValue)
	viper.SetDefault("zen_mode.time", serveCmd.PersistentFlags().Lookup("zentime").DefValue)
	_ = viper.BindPFlag("zen_mode.time", serveCmd.PersistentFlags().Lookup("zentime"))
	_ = viper.BindPFlag("zen_mode.file", serveCmd.PersistentFlags().Lookup("zenfile"))
	viper.SetDefault("timeout", serveCmd.PersistentFlags().Lookup("timeout").DefValue)
	viper.SetDefault("upstream_timeout", serveCmd.PersistentFlags().Lookup("upstreamtimeout").DefValue)
	_ = viper.BindPFlag("timeout", serveCmd.PersistentFlags().Lookup("timeout"))
	_ = viper.BindPFlag("upstream_timeout", serveCmd.PersistentFlags().Lookup("upstreamtimeout"))
	viper.SetDefault("status.enabled", true)
	viper.SetDefault("database.file", db.DefaultFile)
	viper.SetDefault("cors.enabled", false)
	viper.SetDefault("dns_log.enabled", true)
	viper.SetDefault("dns_log.purge", config.DefaultDNSLogRetention)
	viper.SetDefault("diagnostics.listen_addr", config.DefaultDiagnosticsAddress)
	viper.SetDefault("diagnostics.metrics_enabled", true)
	viper.SetDefault("diagnostics.pprof_enabled", false)
	viper.SetDefault("dns_access.allowed_client_cidrs", []string{})
	viper.SetDefault("dns_access.client_queries_per_second", 100)
	viper.SetDefault("dns_access.client_burst", 200)
	viper.SetDefault("dns_access.global_responses_per_second", 1000)
	viper.SetDefault("dns_access.global_response_burst", 2000)
	viper.SetDefault("dns_access.max_concurrent_upstreams", 128)
	viper.SetDefault("dns_access.max_tracked_clients", 4096)
	viper.SetDefault("dns_access.client_idle_timeout", "10m")
	viper.SetDefault("auth.issuer", auth.DefaultIssuer)
	viper.SetDefault("auth.bearer_audience", auth.DefaultBearerAudience)
	viper.SetDefault("auth.active_key.environment", "TDNS_AUTH_ACTIVE_KEY")
	viper.SetDefault("auth.previous_key.environment", "TDNS_AUTH_PREVIOUS_KEY")
	viper.SetDefault("auth.browser.remember_days", config.DefaultBrowserRememberDays)
	viper.SetDefault("tagger.enabled", true)
	viper.SetDefault("cache.enabled", true)

	viper.SetDefault("loglevel", "INFO")

}

func run() {
	setServingUmask()

	logger := log.GetLogger("newServer", "lookup")
	c := &config.Config{}
	if err := validateRemovedConfigOptions(viper.GetViper()); err != nil {
		logger.Fatal(err)
	}
	err := viper.Unmarshal(c)
	if err != nil {
		logger.Fatal(err)
	}
	if err := config.Validate(c); err != nil {
		logger.Fatal(err)
	}
	log.Configure(c.LogLevel, verbose)
	dnsPolicy, err := dnsserver.NewPolicy(c.DNSAccess)
	if err != nil {
		logger.Fatal(err)
	}
	var browserStore *browserauth.Store
	if c.Database.File != "" {
		dbPath, err := db.Bootstrap(context.Background(), c.Database.File)
		if err != nil {
			logger.Fatal(err)
		}
		c.Database.File = dbPath

		store, err := overrides.Open(context.Background(), dbPath)
		if err != nil {
			logger.Fatal(err)
		}
		defer func() {
			_ = store.Close()
		}()

		rows, err := store.List(context.Background())
		if err != nil {
			logger.Fatal(err)
		}
		if err := overrides.Apply(c, rows); err != nil {
			logger.Fatal(err)
		}
		if err := config.Validate(c); err != nil {
			logger.Fatal(err)
		}
		browserStore, err = browserauth.Open(context.Background(), dbPath)
		if err != nil {
			logger.Fatal(err)
		}
		if _, _, err := browserStore.PurgeExpired(
			context.Background(),
			time.Now(),
			browserauth.DefaultPurgeLimit,
		); err != nil {
			logger.Fatal(err)
		}
		dnsPolicy, err = dnsserver.NewPolicy(c.DNSAccess)
		if err != nil {
			logger.Fatal(err)
		}
	}
	authManager, err := auth.NewManager(c.Auth, c.Server.SigningKey, auth.Options{AllowEphemeral: true})
	if err != nil {
		logger.Fatal(err)
	}
	config.SetRunningConfig(c)
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
	logStartupSecurityWarnings(c)
	diagnosticsServer := startDiagnosticsServer(c.Diagnostics)
	newServer := server.NewServer(
		server.WithStaticResponse(),
		server.WithUpstreams(c.Upstreams, c.Timeout, c.UpstreamTimeout),
		server.WithStubs(c.StubResolver.Stubs, c.Timeout, c.UpstreamTimeout),
		server.WithBlacklist(),
		server.WithCacheGet(),
		server.WithCacheSet(),
		server.WithZenMode(),
		server.WithStatus(),
		server.WithDNSLog(),
		server.WithTagger(),
	)
	httpServer, err := startHTTPServer(c, newServer, authManager, browserStore)
	if err != nil {
		logger.Fatal(err)
	}

	dnsHandler := dnsserver.NewHandler(dnsPolicy, newServer)

	var scheduler *gocron.Scheduler
	if len(sched.TaskRegistry) > 0 || browserStore != nil {
		scheduler = gocron.NewScheduler(time.UTC)
		for _, task := range sched.TaskRegistry {
			_, err = sched.AddCron(scheduler, task.Expr, task.Fn)
			if err != nil {
				//ToDo: check all the Fatal calls that could be Error
				logger.Fatal(err)
			}
		}
		if browserStore != nil {
			_, err = scheduler.Every(15).Minutes().SingletonMode().Do(func() {
				sessions, codes, purgeErr := browserStore.PurgeExpired(
					context.Background(),
					time.Now(),
					browserauth.DefaultPurgeLimit,
				)
				if purgeErr != nil {
					logger.WithError(purgeErr).Error("Failed to purge expired browser authentication records.")
					return
				}
				if sessions > 0 || codes > 0 {
					logger.WithFields(logrus.Fields{
						"browser_codes": codes,
						"sessions":      sessions,
					}).Info("Purged expired browser authentication records.")
				}
			})
			if err != nil {
				logger.Fatal(err)
			}
		}
		scheduler.StartAsync()
	}

	srv := &dns.Server{Addr: c.Server.ListenAddr, Net: "udp", Handler: dnsHandler}
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
	if diagnosticsServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := diagnosticsServer.Shutdown(shutdownCtx); err != nil {
			logger.Error(err)
		}
		cancel()
	}
	if httpServer != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error(err)
		}
		cancel()
	}
	if scheduler != nil {
		scheduler.Stop()
	}
	if browserStore != nil {
		if err := browserStore.Close(); err != nil {
			logger.Error(err)
		}
	}
	if p, ok := newServer.Middlewares["tagger"]; ok {
		if closer, ok := p.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				logger.Fatal(err)
			}
		}
	}
	if p, ok := newServer.Middlewares["dns-log"]; ok {
		if stopper, ok := p.(interface{ Stop() }); ok {
			stopper.Stop()
		}
	}
	os.Exit(0)
}

func validateRemovedConfigOptions(v *viper.Viper) error {
	if v.IsSet("server.pprof_addr") {
		return errors.New("server.pprof_addr is no longer supported; configure diagnostics.listen_addr and diagnostics.pprof_enabled")
	}
	return nil
}

func setServingUmask() {
	syscall.Umask(0o077)
}

func startHTTPServer(
	c *config.Config,
	dnsServer *server.Server,
	authManager *auth.Manager,
	browserStore *browserauth.Store,
) (*http.Server, error) {
	logger := log.GetLogger("serve", "http-server")
	handler, err := newHTTPHandler(dnsServer, authManager, browserStore)
	if err != nil {
		return nil, err
	}

	srv := &http.Server{
		Addr:              c.Server.APIAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12},
	}
	go func() {
		logger.Infof("Starting https server at %s, (crt:%s, keyfile:%s)", c.Server.APIAddr, c.Server.APICertFile, c.Server.APIKeyFile)
		if err := srv.ListenAndServeTLS(c.Server.APICertFile, c.Server.APIKeyFile); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(err)
		}
	}()

	return srv, nil
}

func newHTTPHandler(
	dnsServer *server.Server,
	authManager *auth.Manager,
	browserStore *browserauth.Store,
) (http.Handler, error) {
	apiHandler, err := httpapi.NewHandler(dnsServer, authManager, browserStore)
	if err != nil {
		return nil, err
	}
	uiHandlers, err := webui.NewHandlers("")
	if err != nil {
		return nil, fmt.Errorf("prepare embedded web ui: %w", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", apiHandler)
	mux.Handle("/metrics", http.NotFoundHandler())
	mux.Handle("/debug/pprof/", http.NotFoundHandler())
	mux.Handle("/swagger/", apiHandler)
	mux.Handle("/_nuxt/", uiHandlers.Static)
	mux.Handle("/", uiHandlers.SPA)

	return withSecurityHeaders(mux), nil
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}

func startDiagnosticsServer(conf config.DiagnosticsConf) *http.Server {
	if !conf.MetricsEnabled && !conf.PProfEnabled {
		return nil
	}
	logger := log.GetLogger("serve", "diagnostics")
	srv := &http.Server{
		Addr:              conf.ListenAddr,
		Handler:           diagnostics.NewHandler(conf.MetricsEnabled, conf.PProfEnabled),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		logger.WithFields(logrus.Fields{
			"address": conf.ListenAddr,
			"metrics": conf.MetricsEnabled,
			"pprof":   conf.PProfEnabled,
		}).Info("Starting diagnostics server.")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(err)
		}
	}()
	return srv
}

func logStartupSecurityWarnings(c *config.Config) {
	logger := log.GetLogger("serve", "security")
	for _, warning := range startupSecurityWarnings(c, viper.ConfigFileUsed()) {
		logger.Warn(warning)
	}
}

func startupSecurityWarnings(c *config.Config, configPath string) []string {
	warnings := make([]string, 0, 6)
	for _, listener := range []struct {
		name    string
		address string
	}{
		{name: "DNS", address: c.Server.ListenAddr},
		{name: "management", address: c.Server.APIAddr},
	} {
		host, _, err := net.SplitHostPort(listener.address)
		ip := net.ParseIP(host)
		if err == nil && (host == "" || ip != nil && ip.IsUnspecified()) {
			warnings = append(warnings, listener.name+" listener uses a wildcard address; restrict it to a trusted interface in production")
		}
	}
	if c.Server.SwaggerEnabled {
		warnings = append(warnings, "Swagger is enabled on the management listener; expose it only in a trusted environment")
	}
	for _, file := range []struct {
		name      string
		path      string
		forbidden os.FileMode
	}{
		{name: "configuration", path: configPath, forbidden: 0o027},
		{name: "management TLS private key", path: c.Server.APIKeyFile, forbidden: 0o027},
		{name: "active signing key", path: c.Auth.ActiveKey.File, forbidden: 0o027},
		{name: "previous signing key", path: c.Auth.PreviousKey.File, forbidden: 0o027},
		{name: "SQLite database", path: c.Database.File, forbidden: 0o077},
	} {
		if warning := sensitiveFileWarning(file.name, file.path, file.forbidden); warning != "" {
			warnings = append(warnings, warning)
		}
	}
	if c.Database.File != "" {
		if warning := sensitiveDirectoryWarning("SQLite directory", filepath.Dir(c.Database.File)); warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func sensitiveFileWarning(name, path string, forbidden os.FileMode) string {
	if path == "" {
		return ""
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		return fmt.Sprintf("cannot inspect %s permissions: %v", name, err)
	}
	if !info.Mode().IsRegular() {
		return name + " is not a regular file"
	}
	if info.Mode().Perm()&forbidden != 0 {
		return fmt.Sprintf("%s permissions %04o are too permissive", name, info.Mode().Perm())
	}
	return ""
}

func sensitiveDirectoryWarning(name, path string) string {
	if path == "" || path == "." {
		return ""
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return ""
	}
	if err != nil {
		return fmt.Sprintf("cannot inspect %s permissions: %v", name, err)
	}
	if !info.IsDir() {
		return name + " is not a directory"
	}
	if info.Mode().Perm()&0o027 != 0 {
		return fmt.Sprintf("%s permissions %04o permit group write or other access", name, info.Mode().Perm())
	}
	return ""
}
