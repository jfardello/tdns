package cmd

import (
	"fmt"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	exp int
	sub string
)

// replaceCmd represents the replace command
var tokenCmd = &cobra.Command{

	Use:   "token",
	Short: "create a token for the cli command",
	Long:  `blah blah.`,
	Run: func(cmd *cobra.Command, args []string) {
		c := &config.Config{}
		viper.Unmarshal(c)
		config.SetRunningConfig(c)
		t, _ := api.IssueToken(exp, sub)
		fmt.Printf("Creating token with %d expiration days.\n", exp)
		fmt.Println("Access token (keep it secret): \n\t", t)

	},
}

func init() {

	manageCmd.AddCommand(tokenCmd)
	tokenCmd.PersistentFlags().IntVarP(&exp, "exp", "e", 500, "Expiry time in days")
	tokenCmd.PersistentFlags().StringVarP(&sub, "sub", "s", "admin@tdns", "JWT subject claim")

}
