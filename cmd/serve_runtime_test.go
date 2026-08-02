package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/jfardello/tdns/config"
	"github.com/spf13/viper"
)

func TestServingUmaskProtectsRuntimeFiles(t *testing.T) {
	oldMask := syscall.Umask(0)
	syscall.Umask(oldMask)
	t.Cleanup(func() {
		syscall.Umask(oldMask)
	})
	setServingUmask()

	path := filepath.Join(t.TempDir(), "runtime-state")
	if err := os.WriteFile(path, []byte("state"), 0o666); err != nil {
		t.Fatalf("write runtime state: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat runtime state: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("runtime state mode = %04o, want 0600", got)
	}
}

func TestRemovedPProfAddressFailsWithMigrationMessage(t *testing.T) {
	v := viper.New()
	v.Set("server.pprof_addr", "127.0.0.1:6060")
	err := validateRemovedConfigOptions(v)
	if err == nil || !strings.Contains(err.Error(), "diagnostics.pprof_enabled") {
		t.Fatalf("validateRemovedConfigOptions error = %v", err)
	}
}

func TestStartupSecurityWarnings(t *testing.T) {
	dir := t.TempDir()
	private := filepath.Join(dir, "private")
	permissive := filepath.Join(dir, "permissive")
	if err := os.WriteFile(private, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(permissive, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	conf := &config.Config{
		Server: config.Server{
			ListenAddr: ":53",
			APIAddr:    "0.0.0.0:8443",
			APIKeyFile: permissive,
			SigningKey: "",
		},
		Auth: config.AuthConf{ActiveKey: config.SigningKeyConf{File: private}},
	}
	warnings := strings.Join(startupSecurityWarnings(conf, private), "\n")
	for _, expected := range []string{"DNS listener uses a wildcard", "management listener uses a wildcard", "management TLS private key permissions 0644"} {
		if !strings.Contains(warnings, expected) {
			t.Errorf("warnings missing %q:\n%s", expected, warnings)
		}
	}
	if strings.Contains(warnings, "active signing key") {
		t.Fatalf("warnings rejected a private signing key:\n%s", warnings)
	}
}
