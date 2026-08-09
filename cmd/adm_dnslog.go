package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jfardello/tdns/apiclient"
	"github.com/jfardello/tdns/log"
	"github.com/spf13/cobra"
	"net"
	"os"
	"text/tabwriter"
)

var (
	topLimit      int
	hostName      string
	ipAddress     string
	since         string
	topStatus     string
	topClient     string
	topClientMode string
)

var topCmd = &cobra.Command{

	Use:   "top",
	Short: "Get top queried DNS records",
	Long:  `Get top queried DNS records grouped by consuming client.`,
	Run: func(cmd *cobra.Command, args []string) {
		setPersistentOps()
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
	payLoad := apiclient.DNSLogAliasRequest{
		Name: hostName,
		Addr: ipAddress,
	}
	resp, err := runManagementOperation(context.Background(), func(ctx context.Context, client *apiclient.Client) (*apiclient.Response, error) {
		return client.DNSLogAliasSet(ctx, payLoad)
	})
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
	manageCmd.AddCommand(aliasCmd)
	topCmd.Flags().StringVarP(&since, "since", "s", "1w", "Return logs newer than a relative duration like 5s, 2m, or 3h. (default 1w)")
	topCmd.Flags().IntVarP(&topLimit, "limit", "l", 20, "Top query limit")
	topCmd.Flags().StringVar(&topStatus, "status", "", "Filter by query status: blocked or allowed")
	topCmd.Flags().StringVar(&topClient, "client", "", "Filter by client alias or IP address")
	topCmd.Flags().StringVar(&topClientMode, "client-mode", "", "Client filter mode: host or ip. If omitted it is inferred from the client value")
	aliasCmd.Flags().StringVarP(&hostName, "hostname", "n", "", "hostname")
	aliasCmd.Flags().StringVarP(&ipAddress, "address", "a", "", "hostname")

}

func getTop() error {
	logger := log.GetLogger("cmd", "getTop")
	params := &apiclient.DNSLogTopParams{}
	if since != "" {
		params.Since = &since
	}
	if topStatus != "" {
		status := apiclient.DNSLogTopStatus(topStatus)
		params.Status = &status
	}
	if topClient != "" {
		params.Client = &topClient
		mode := topClientMode
		if mode == "" {
			if net.ParseIP(topClient) != nil {
				mode = "ip"
			} else {
				mode = "host"
			}
		}
		clientMode := apiclient.DNSLogTopClientMode(mode)
		params.ClientMode = &clientMode
	}
	resp, err := runManagementOperation(context.Background(), func(ctx context.Context, client *apiclient.Client) (*apiclient.Response, error) {
		return client.DNSLogTop(ctx, topLimit, params)
	})
	if err != nil {
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 10, 4, 3, ' ', tabwriter.AlignRight)
	if len(resp.LogItems) == 0 {

		logger.Debug("no records found")
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
