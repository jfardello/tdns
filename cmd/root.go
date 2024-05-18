package cmd

import (
	"os"

	"github.com/jfardello/tdns/config"
	"github.com/spf13/cobra"
)

var verbose bool
var conf config.Config

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "tdns",
	Short: "A DNSoT capable caching dns forwarder with black hole features.",
	Long: `TDNS is a dns forwarder, with scheduled black hole capabilities and ReST admin interface.

It can change non primary "domain" stub servers on runtime, so that scripts that react to
new interfaces like wifi or VPN tun/tap can configure internal network specific DNS
servers for its search domains.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// when this action is called directly.

	initConfig()
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output.")

}

//TODO: unittest

//TODO: lint everithing
