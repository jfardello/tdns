/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jfardello/tdns/internal/apiclient"

	"github.com/spf13/cobra"
)

// stopCmd represents the stop command.
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stops interceptor services.",
	Run: func(cmd *cobra.Command, args []string) {
		setPersistentOps()
		err := cmd.Help()
		if err != nil {
			panic(err)
		}
	},
}

// stopStubsCmd represents the stubs command.
var stopStubsCmd = &cobra.Command{
	Use:   "stub-resolver",
	Short: "Stops the stub resolver middleware",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := runManagementOperation(cmd.Context(), func(ctx context.Context, client *apiclient.Client) (*apiclient.Response, error) {
			return client.StubResolverToggle(ctx, "stop")
		})
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var stoppStaticCmd = &cobra.Command{
	Use:   "static-response",
	Short: "Stops the static response middleware",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := runManagementOperation(cmd.Context(), func(ctx context.Context, client *apiclient.Client) (*apiclient.Response, error) {
			return client.StaticResponseToggle(ctx, "stop")
		})
		if err != nil {
			fmt.Println(err.Error())
		}
		jstr, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(jstr))
	},
}

var stopBlacklistCmd = &cobra.Command{
	Use:   "blacklist",
	Short: "Stops the blacklist middleware.",
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := runManagementOperation(cmd.Context(), func(ctx context.Context, client *apiclient.Client) (*apiclient.Response, error) {
			return client.BlacklistToggle(ctx, "stop")
		})
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
	stopCmd.AddCommand(stopBlacklistCmd)

}
