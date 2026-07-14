/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/jfardello/tdns/internal/apiclient"
	"github.com/spf13/cobra"
)

// startCmd represents the start command.
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts interceptors",
	Run: func(cmd *cobra.Command, args []string) {
		err := cmd.Help()
		if err != nil {
			panic(err)
		}
	},
}

var startZenCmd = &cobra.Command{
	Use:   "zen-mode",
	Short: "Starts zen mode period interceptor",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiclient.Post(cmd.Context(), "/api/zen-mode/start", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var startBlacklistCmd = &cobra.Command{
	Use:   "blacklist",
	Short: "Start blacklist middleware",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiclient.Post(cmd.Context(), "/api/blacklist/start", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var startStubsCmd = &cobra.Command{
	Use:   "stub-resolver",
	Short: "Start stub resolver middleware",
	Run: func(cmd *cobra.Command, args []string) {
		setPersistentOps()
		resp, err := apiclient.Post(cmd.Context(), "/api/stub-resolver/start", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

// startStaticCmd represents the static command.
var startStaticCmd = &cobra.Command{
	Use:   "static-response",
	Short: "Start static response middleware",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiclient.Post(cmd.Context(), "/api/static-response/start", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

func init() {
	manageCmd.AddCommand(startCmd)
	startCmd.AddCommand(startZenCmd)
	startCmd.AddCommand(startBlacklistCmd)
	startCmd.AddCommand(startStubsCmd)
	startCmd.AddCommand(startStaticCmd)

}
