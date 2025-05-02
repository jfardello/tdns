package cmd

import (
	"encoding/base64"
	"fmt"

	"github.com/jfardello/tdns/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// replaceCmd represents the replace command.
var signingCmd = &cobra.Command{

	Use:   "genkey",
	Short: "Write signing key to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		setPersitentOpst()
		c := &config.Config{}
		err := viper.Unmarshal(c)
		if err != nil {
			panic(err)
		}
		config.SetRunningConfig(c)
		k := config.GenKey()
		fmt.Println(base64.StdEncoding.EncodeToString(*k))

	},
}

func init() {
	manageCmd.AddCommand(signingCmd)

}
