package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/jfardello/tdns/apiclient"
	"github.com/jfardello/tdns/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	zenDomains []string
)

// replaceCmd represents the replace command.
var replaceCmd = &cobra.Command{

	Use:   "replace",
	Short: "Replace stub-resolver or zen-mode domains.",
	Long: `Replace runtime config for zen-mode domains or stub servers, this
	won't persist changes.`,
	Run: func(cmd *cobra.Command, args []string) {
		setPersistentOps()
		logger := log.GetLogger("cmd", "replaceCmd")
		if len(stubs) > 0 {
			err := handleStubs(stubs)
			if err != nil {
				logger.Error(err)
			}
		}
		if len(zenDomains) > 0 {
			err := handleZenDomains(zenDomains)
			if err != nil {
				logger.Error(err)
			}
		}

	},
}

func init() {
	manageCmd.AddCommand(replaceCmd)
	replaceCmd.PersistentFlags().StringSliceVarP(&zenDomains, "zen-domains", "z", []string{}, "Forbidden domains for zen mode.")
	replaceCmd.PersistentFlags().StringSliceVarP(&stubs, "stub", "s", []string{}, "Stubs servers for domains ex: domain.tld,udp://8.8.8.8")
	viper.SetDefault("zen_mode.domains", replaceCmd.PersistentFlags().Lookup("zen-domains").DefValue)
	_ = viper.BindPFlag("upstreams", replaceCmd.PersistentFlags().Lookup("upstream"))

}

func handleStubs(stubs []string) error {
	sreq := apiclient.StubReplaceRequest{
		Stubs: stubs,
	}

	resp, err := runManagementOperation(context.Background(), func(ctx context.Context, client *apiclient.Client) (*apiclient.Response, error) {
		return client.StubResolverReplace(ctx, sreq)
	})
	if err != nil {
		return err
	}
	logger := log.GetLogger("adm", "handleStubs")
	logger.Debug(resp.Message)

	logger.Infof("Current stubs: %+v", resp.Items)

	return nil
}

func handleZenDomains(domains []string) error {
	sreq := apiclient.ZenReplaceRequest{
		ZenDomains: domains,
	}

	logger := log.GetLogger("adm", "handleZenDomains")
	resp, err := runManagementOperation(context.Background(), func(ctx context.Context, client *apiclient.Client) (*apiclient.Response, error) {
		return client.ZenModeDomainsReplace(ctx, sreq)
	})
	if err != nil {
		logger.Error(err)
		return err
	}

	logger.Debug(resp.Message)

	logger.Infof("Current domains: %+v", resp.Items)

	jstr, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(jstr))
	return nil
}
