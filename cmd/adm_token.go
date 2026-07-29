package cmd

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	exp                 int
	sub                 string
	tokenScope          string
	allowLongLivedToken bool
)

// replaceCmd represents the replace command.
var tokenCmd = &cobra.Command{

	Use:   "token",
	Short: "create a token for the cli command",
	RunE: func(cmd *cobra.Command, args []string) error {
		initConfig()
		setPersistentOps()
		c := &config.Config{}
		if err := viper.Unmarshal(c); err != nil {
			return err
		}
		lifetime, err := tokenLifetime(exp, allowLongLivedToken)
		if err != nil {
			return err
		}
		scope, err := parseTokenScope(tokenScope)
		if err != nil {
			return err
		}
		authManager, err := auth.NewManager(c.Auth, c.Server.SigningKey, auth.Options{})
		if err != nil {
			return err
		}
		var t string
		if allowLongLivedToken {
			t, err = authManager.IssueLongLivedBearer(sub, scope, lifetime)
		} else {
			t, err = authManager.IssueBearer(sub, scope, lifetime)
		}
		if err != nil {
			return err
		}
		fmt.Printf("Creating token with %d expiration days.\n", exp)
		fmt.Println("Access token (keep it secret): \n\t", t)
		return nil
	},
}

func init() {

	manageCmd.AddCommand(tokenCmd)
	tokenCmd.PersistentFlags().IntVarP(&exp, "exp", "e", auth.DefaultTokenDays, "Expiry time in days")
	tokenCmd.PersistentFlags().StringVarP(&sub, "sub", "s", "admin@tdns", "JWT subject claim")
	tokenCmd.PersistentFlags().StringVar(&tokenScope, "scope", "read-write", "Token scope: read-only or read-write")
	tokenCmd.PersistentFlags().BoolVar(&allowLongLivedToken, "allow-long-lived", false, "Allow a token lifetime above the normal maximum")

}

func tokenLifetime(days int, allowLongLived bool) (time.Duration, error) {
	if days <= 0 {
		return 0, errors.New("expiration must be greater than zero days")
	}
	if days > auth.MaximumTokenDays && !allowLongLived {
		return 0, fmt.Errorf(
			"expiration exceeds the %d-day maximum; use --allow-long-lived to override",
			auth.MaximumTokenDays,
		)
	}
	const maximumDurationDays = int64(^uint64(0)>>1) / int64(24*time.Hour)
	if int64(days) > maximumDurationDays {
		return 0, errors.New("expiration is too large")
	}
	return time.Duration(days) * 24 * time.Hour, nil
}

func parseTokenScope(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "read-only", "ro":
		return auth.ScopeRead, nil
	case "read-write", "rw":
		return auth.ScopeWrite, nil
	default:
		return "", fmt.Errorf("unsupported scope %q: use read-only or read-write", value)
	}
}
