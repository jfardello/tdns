package cmd

import (
	"errors"
	"fmt"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/auth"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	browserCodeTTL     time.Duration
	browserCodeSubject string
	browserCodeScope   string
)

var browserCodeCmd = &cobra.Command{
	Use:   "browser-code",
	Short: "create a single-use browser login code",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		initConfig()
		setPersistentOps()
		c := &config.Config{}
		if err := viper.Unmarshal(c); err != nil {
			return err
		}
		lifetime, err := browserCodeLifetime(browserCodeTTL)
		if err != nil {
			return err
		}
		scope, err := parseTokenScope(browserCodeScope)
		if err != nil {
			return err
		}
		authManager, err := auth.NewManager(c.Auth, c.Server.SigningKey, auth.Options{})
		if err != nil {
			return err
		}
		code, err := authManager.IssueBrowserCode(browserCodeSubject, scope, lifetime)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), code)
		return err
	},
}

func init() {
	manageCmd.AddCommand(browserCodeCmd)
	browserCodeCmd.Flags().DurationVar(&browserCodeTTL, "ttl", auth.BrowserCodeTTL, "Login code lifetime")
	browserCodeCmd.Flags().StringVarP(&browserCodeSubject, "sub", "s", "admin@tdns", "Login subject")
	browserCodeCmd.Flags().StringVar(&browserCodeScope, "scope", "read-write", "Session scope: read-only or read-write")
}

func browserCodeLifetime(lifetime time.Duration) (time.Duration, error) {
	if lifetime <= 0 {
		return 0, errors.New("browser login code lifetime must be greater than zero")
	}
	if lifetime > auth.BrowserCodeTTL {
		return 0, fmt.Errorf("browser login code lifetime exceeds the %s maximum", auth.BrowserCodeTTL)
	}
	return lifetime, nil
}
