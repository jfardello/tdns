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
	Use:   "zen",
	Short: "Starts zen mode period interceptor",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := api.Post(cmd.Context(), "/api/zen/start", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var startBholeCmd = &cobra.Command{
	Use:   "bhole",
	Short: "Start Black hole interceptor",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := api.Post(cmd.Context(), "/api/bhole/start", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var startStubsCmd = &cobra.Command{
	Use:   "stubs",
	Short: "Start stub server interceptor",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := api.Post(cmd.Context(), "/api/stubs/start", nil)
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

// startStaticCmd represents the static command.
var startStaticCmd = &cobra.Command{
	Use:   "static",
	Short: "Start static file respose interceptor",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := api.Post(cmd.Context(), "/api/static/start", nil)
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
	startCmd.AddCommand(startBholeCmd)
	startCmd.AddCommand(startStubsCmd)
	startCmd.AddCommand(startStaticCmd)

}
