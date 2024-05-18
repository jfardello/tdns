/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/jfardello/tdns/api"

	"github.com/spf13/cobra"
)

// stopCmd represents the stop command
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops interceptor services.",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

// stopStubsCmd represents the stubs command
var stopStubsCmd = &cobra.Command{
	Use:   "stubs",
	Short: "Stops the stub-server interceptors",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := api.Post(cmd.Context(), "/api/stubs/stop", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var stoppStaticCmd = &cobra.Command{
	Use:   "static",
	Short: "A brief description of your command",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := api.Post(cmd.Context(), "/api/static/stop", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var stopBholeCmd = &cobra.Command{
	Use:   "bhole",
	Short: "Stops the black hole intertceptor.",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := api.Post(cmd.Context(), "/api/bhole/stop", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

func init() {
	manageCmd.AddCommand(stopCmd)
	stopCmd.AddCommand(stopStubsCmd)
	stopCmd.AddCommand(stoppStaticCmd)
	stopCmd.AddCommand(stopBholeCmd)

}
