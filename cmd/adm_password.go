package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/internal/browserauth"
	"github.com/jfardello/tdns/internal/db"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

type administratorPasswordStore interface {
	SetAdministratorPassword(context.Context, string, []byte, time.Time) error
	DisableAdministrator(context.Context, time.Time) error
	Close() error
}

type passwordPrompt func(*cobra.Command, string) ([]byte, error)

type administratorPasswordCommandDependencies struct {
	openStore func(context.Context) (administratorPasswordStore, error)
	prompt    passwordPrompt
	now       func() time.Time
}

func init() {
	manageCmd.AddCommand(newAdministratorPasswordCommand(administratorPasswordCommandDependencies{
		openStore: openConfiguredAdministratorStore,
		prompt:    readTerminalPassword,
		now:       time.Now,
	}))
}

func newAdministratorPasswordCommand(deps administratorPasswordCommandDependencies) *cobra.Command {
	passwordCmd := &cobra.Command{
		Use:   "password",
		Short: "manage the local administrator password",
	}

	var username string
	setCmd := &cobra.Command{
		Use:   "set",
		Short: "set or replace the local administrator password",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := deps.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer store.Close()

			password, err := deps.prompt(cmd, "Password: ")
			if err != nil {
				return err
			}
			defer clearSecret(password)
			confirmation, err := deps.prompt(cmd, "Confirm password: ")
			if err != nil {
				return err
			}
			defer clearSecret(confirmation)
			if !bytes.Equal(password, confirmation) {
				return errors.New("password confirmation does not match")
			}
			if err := store.SetAdministratorPassword(cmd.Context(), username, password, deps.now()); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Local administrator password updated; password-authenticated sessions revoked.")
			return err
		},
	}
	setCmd.Flags().StringVar(&username, "username", "admin", "Local administrator username")

	disableCmd := &cobra.Command{
		Use:   "disable",
		Short: "disable local administrator password login",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := deps.openStore(cmd.Context())
			if err != nil {
				return err
			}
			defer store.Close()
			if err := store.DisableAdministrator(cmd.Context(), deps.now()); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Local administrator password login disabled; password-authenticated sessions revoked.")
			return err
		},
	}

	passwordCmd.AddCommand(setCmd, disableCmd)
	return passwordCmd
}

func openConfiguredAdministratorStore(ctx context.Context) (administratorPasswordStore, error) {
	setPersistentOps()
	initConfig()
	c := &config.Config{}
	if err := viper.Unmarshal(c); err != nil {
		return nil, err
	}
	resolvedPath, err := db.Bootstrap(ctx, c.Database.File)
	if err != nil {
		return nil, err
	}
	return browserauth.Open(ctx, resolvedPath)
}

func readTerminalPassword(cmd *cobra.Command, prompt string) ([]byte, error) {
	input, ok := cmd.InOrStdin().(*os.File)
	if !ok || !term.IsTerminal(int(input.Fd())) {
		return nil, errors.New("password input requires an interactive terminal")
	}
	if _, err := fmt.Fprint(cmd.ErrOrStderr(), prompt); err != nil {
		return nil, err
	}
	password, err := term.ReadPassword(int(input.Fd()))
	_, newlineErr := fmt.Fprintln(cmd.ErrOrStderr())
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	if newlineErr != nil {
		clearSecret(password)
		return nil, newlineErr
	}
	return password, nil
}

func clearSecret(secret []byte) {
	for i := range secret {
		secret[i] = 0
	}
}
