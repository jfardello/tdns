package blocklist

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ReadRevision(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	body, err := readLimited(file, 128)
	if err != nil {
		return ""
	}
	revision := strings.TrimSpace(string(body))
	if !validRevision(revision) {
		return ""
	}
	return strings.ToLower(revision)
}

func WriteRevision(path, revision string) error {
	if !validRevision(revision) {
		return fmt.Errorf("invalid revision")
	}
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".revision.tmp-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.WriteString(strings.ToLower(revision) + "\n"); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	committed = true
	return nil
}
