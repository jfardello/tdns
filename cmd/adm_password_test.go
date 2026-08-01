package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

type fakeAdministratorPasswordStore struct {
	username string
	password []byte
	setCalls int
	disabled bool
	closed   bool
}

func (s *fakeAdministratorPasswordStore) SetAdministratorPassword(_ context.Context, username string, password []byte, _ time.Time) error {
	s.username = username
	s.password = append([]byte(nil), password...)
	s.setCalls++
	return nil
}

func (s *fakeAdministratorPasswordStore) DisableAdministrator(context.Context, time.Time) error {
	s.disabled = true
	return nil
}

func (s *fakeAdministratorPasswordStore) Close() error {
	s.closed = true
	return nil
}

func TestAdministratorPasswordSetReadsAndConfirmsSecret(t *testing.T) {
	store := &fakeAdministratorPasswordStore{}
	wantPassword := "correct horse battery staple"
	responses := [][]byte{[]byte(wantPassword), []byte(wantPassword)}
	var prompts []string
	command := newAdministratorPasswordCommand(administratorPasswordCommandDependencies{
		openStore: func(context.Context) (administratorPasswordStore, error) { return store, nil },
		prompt: func(_ *cobra.Command, prompt string) ([]byte, error) {
			prompts = append(prompts, prompt)
			response := responses[len(prompts)-1]
			return response, nil
		},
		now: func() time.Time { return time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC) },
	})
	var output strings.Builder
	command.SetOut(&output)
	command.SetArgs([]string{"set", "--username", "Admin.User"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if strings.Join(prompts, "|") != "Password: |Confirm password: " {
		t.Fatalf("prompts = %q", prompts)
	}
	if store.username != "Admin.User" || string(store.password) != wantPassword || store.setCalls != 1 || !store.closed {
		t.Fatalf("store = %#v", store)
	}
	if strings.Contains(output.String(), wantPassword) {
		t.Fatal("command output exposed the plaintext password")
	}
	for _, response := range responses {
		if !allZero(response) {
			t.Fatal("prompt response was not cleared")
		}
	}
}

func TestAdministratorPasswordCommandsDoNotAcceptPasswordArguments(t *testing.T) {
	store := &fakeAdministratorPasswordStore{}
	command := newTestAdministratorPasswordCommand(store, []string{"secret", "secret"})
	command.SetArgs([]string{"set", "plaintext-password"})
	if err := command.Execute(); err == nil {
		t.Fatal("set accepted a plaintext password argument")
	}
	if store.setCalls != 0 {
		t.Fatal("store was called after an invalid password argument")
	}
}

func TestAdministratorPasswordConfirmationMismatch(t *testing.T) {
	store := &fakeAdministratorPasswordStore{}
	command := newTestAdministratorPasswordCommand(store, []string{"first password value", "second password value"})
	command.SetArgs([]string{"set"})
	err := command.Execute()
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error = %v", err)
	}
	if store.setCalls != 0 {
		t.Fatal("store was called with mismatched confirmation")
	}
}

func TestAdministratorPasswordDisable(t *testing.T) {
	store := &fakeAdministratorPasswordStore{}
	command := newTestAdministratorPasswordCommand(store, nil)
	command.SetArgs([]string{"disable"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if !store.disabled || !store.closed {
		t.Fatalf("disabled = %t, closed = %t", store.disabled, store.closed)
	}
}

func TestReadTerminalPasswordRejectsNonTerminalInput(t *testing.T) {
	command := newTestAdministratorPasswordCommand(&fakeAdministratorPasswordStore{}, nil)
	command.SetIn(strings.NewReader("plaintext"))
	_, err := readTerminalPassword(command, "Password: ")
	if err == nil || !strings.Contains(err.Error(), "interactive terminal") {
		t.Fatalf("error = %v", err)
	}
}

func newTestAdministratorPasswordCommand(store administratorPasswordStore, responses []string) *cobra.Command {
	index := 0
	return newAdministratorPasswordCommand(administratorPasswordCommandDependencies{
		openStore: func(context.Context) (administratorPasswordStore, error) { return store, nil },
		prompt: func(_ *cobra.Command, _ string) ([]byte, error) {
			if index >= len(responses) {
				return nil, errors.New("unexpected password prompt")
			}
			response := []byte(responses[index])
			index++
			return response, nil
		},
		now: time.Now,
	})
}

func allZero(value []byte) bool {
	for _, item := range value {
		if item != 0 {
			return false
		}
	}
	return true
}
