package db

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const DefaultFile = "/var/lib/tdns/tdns.sqlite"

func Bootstrap(ctx context.Context, dbPath string) (string, error) {
	resolvedPath, err := ResolvePath(dbPath)
	if err != nil {
		return "", err
	}

	for _, target := range []Target{TargetDNSLog, TargetTagger, TargetConfig} {
		if err := RunMigrations(ctx, resolvedPath, target); err != nil {
			return "", fmt.Errorf("run %s migrations for %s: %w", target, resolvedPath, err)
		}
	}

	return resolvedPath, nil
}

func ResolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("database file is empty")
	}

	info, err := os.Stat(path)
	switch {
	case err == nil && info.IsDir():
		path = filepath.Join(path, "tdns.sqlite")
	case err == nil:
		return path, nil
	case !errors.Is(err, os.ErrNotExist):
		return "", err
	}

	parent := filepath.Dir(path)
	if parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return "", err
		}
	}

	return path, nil
}
