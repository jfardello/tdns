/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// manageCmd represents the manage command.
var manageCmd = &cobra.Command{
	Use:   "adm",
	Short: "TDNS management commands",
	Long:  `Iteract with remote TDNS instances ReST API.`,
	Run: func(cmd *cobra.Command, args []string) {
		initConfig()
	},
}

func init() {
	rootCmd.AddCommand(manageCmd)

}
