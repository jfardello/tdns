package cmd

import (
	"os"

	"github.com/jfardello/tdns/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	ver         *string
	gitcommit   *string
	compiledate *string
	verbose     bool
	configFile  string
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "tdns",
	Short: "A DNSoT capable caching dns forwarder with black hole features.",
	Long: `TDNS is a dns forwarder, with scheduled black hole capabilities and ReST admin interface.

It can change non primary "domain" stub servers on runtime, so that scripts that react to
new interfaces like wifi or VPN tun/tap can configure internal network specific DNS
servers for its search domains.`,
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
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "V", false, "Verbose output.")
	rootCmd.PersistentFlags().StringVarP(&configFile, "configfile", "c", "", "Config file.")
	if configFile != "" {
		viper.SetConfigFile(configFile)

	}
	if verbose {
		log.SetLevel(logrus.DebugLevel)
	}
}
