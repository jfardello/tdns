package cmd

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
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
