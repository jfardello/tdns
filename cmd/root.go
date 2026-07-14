package cmd

import (
	"fmt"
	"os"

	"github.com/jfardello/tdns/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ver         *string
	gitcommit   *string
	compiledate *string
	verbose     bool
	configFile  string
	showVersion bool
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "tdns",
	Short: "A DNSoT capable caching dns forwarder with black hole features.",
	Long: `TDNS is a dns forwarder, with scheduled black hole capabilities and ReST admin interface.

It can change non primary "domain" stub servers on runtime, so that scripts that react to
new interfaces like wifi or VPN tun/tap can configure internal network specific DNS
servers for its search domains.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		setPersistentOps()
	},
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			printVersion()
		} else {
			_ = cmd.Help()
		}
	},
}

func Execute(version, commit, date string) {
	ver = &version
	gitcommit = &commit
	compiledate = &date
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rf := rootCmd.PersistentFlags()
	rf.BoolVarP(&verbose, "verbose", "v", false, "Show verbose output.")
	rf.StringVarP(&configFile, "configfile", "c", "", "Config file.")
	rootCmd.Flags().BoolVar(&showVersion, "version", false, "Print version information and exit.")

}

func setPersistentOps() {
	if configFile != "" {
		viper.SetConfigFile(configFile)
	}
	log.Configure("", verbose)
}

func printVersion() {
	fmt.Print(formatVersion())
}

func formatVersion() string {
	version := valueOrDefault(ver, "dev")
	commit := valueOrDefault(gitcommit, "none")
	date := valueOrDefault(compiledate, "unknown")
	return fmt.Sprintf("tdns version %s\ncommit %s\nbuilt %s\n", version, commit, date)
}

func valueOrDefault(value *string, fallback string) string {
	if value == nil || *value == "" {
		return fallback
	}
	return *value
}
