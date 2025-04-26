package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/log"
	"github.com/spf13/cobra"
	"os"
	"text/tabwriter"
)

var (
	topLimit  int
	hostName  string
	ipAddress string
)

var topCmd = &cobra.Command{

	Use:   "top",
	Short: "Get top queried DNS records",
	Long:  `Get top queried DNS records grouped by consumin client.`,
	Run: func(cmd *cobra.Command, args []string) {
		initConfig()
		logger := log.GetLogger("cmd", "topCmd")
		err := getTop()
		if err != nil {
			logger.Error(err)
		}
	},
}

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Set alias records for ip addresses.",
	Long:  `Set alias records for client ip addresses. It will be used the top report if available.`,
	Run: func(cmd *cobra.Command, args []string) {
		initConfig()
		logger := log.GetLogger("cmd", "aliasCmd")
		err := handleAlias(hostName, ipAddress)
		if err != nil {
			logger.Error(err)
		}
	},
}

func handleAlias(hostName, ipAddress string) error {
	payLoad := api.DNSLogAliasRequest{
		Name: hostName,
		Addr: ipAddress,
	}
	resp, err := api.Post(context.Background(), "/api/dnslog/alias", payLoad)
	if err != nil {
		return err
	}
	j, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	fmt.Println(string(j))
	return nil

}

func init() {
	manageCmd.AddCommand(topCmd)
	topCmd.PersistentFlags().IntVarP(&topLimit, "limit", "l", 20, "Top query limit")
	manageCmd.AddCommand(aliasCmd)
	aliasCmd.PersistentFlags().StringVarP(&hostName, "hostname", "n", "", "hostname")
	aliasCmd.PersistentFlags().StringVarP(&ipAddress, "address", "a", "", "hostname")

}

func getTop() error {
	logger := log.GetLogger("cmd", "getTop")
	resp, err := api.Get(context.Background(), fmt.Sprintf("/api/dnslog/top/%d", topLimit))
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 10, 4, 3, ' ', tabwriter.AlignRight)
	if len(resp.LogItems) == 0 {
		fmt.Println("No dnslog records.")
		return nil
	}
	for _, v := range resp.LogItems {

		_, err := fmt.Fprintf(tw, "%s\t%d\t%s\t\n", v.Domain, v.Counter, v.Host)
		if err != nil {
			logger.Error(err)
		}
	}
	_ = tw.Flush()

	return nil
}
