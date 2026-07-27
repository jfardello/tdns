package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGenerate384CertUsesRestrictiveModes(t *testing.T) {
	dir := t.TempDir()
	certPath, keyPath := Generate384Cert(dir, "tdns_", time.Hour, []string{"localhost", "127.0.0.1"})

	tests := []struct {
		name string
		path string
		mode os.FileMode
	}{
		{name: "certificate", path: certPath, mode: 0o644},
		{name: "private key", path: keyPath, mode: 0o600},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := os.Stat(filepath.Clean(tt.path))
			if err != nil {
				t.Fatalf("stat %s: %v", tt.path, err)
			}
			if got := info.Mode().Perm(); got != tt.mode {
				t.Fatalf("mode = %04o, want %04o", got, tt.mode)
			}
		})
	}
}
