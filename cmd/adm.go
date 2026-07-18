/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"

	"github.com/jfardello/tdns/apiclient"
	"github.com/jfardello/tdns/config"
	"github.com/spf13/cobra"
)

// manageCmd represents the manage command.
var manageCmd = &cobra.Command{
	Use:   "adm",
	Short: "TDNS management commands",
	Long:  `Iteract with remote TDNS instances ReST API.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		initConfig()
	},
}

type managementOperation func(context.Context, *apiclient.Client) (*apiclient.Response, error)

func runManagementOperation(ctx context.Context, operation managementOperation) (*apiclient.Response, error) {
	client, err := apiclient.NewFromConfig(config.GetRunningConfig().Client)
	if err != nil {
		return nil, err
	}
	return operation(ctx, client)
}

func init() {
	rootCmd.AddCommand(manageCmd)

}
