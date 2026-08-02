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
	if err := validateDirectory(resolvedPath); err != nil {
		return "", err
	}
	if err := ProtectFiles(resolvedPath); err != nil {
		return "", err
	}

	for _, target := range []Target{TargetDNSLog, TargetTagger, TargetConfig, TargetAuth} {
		if err := RunMigrations(ctx, resolvedPath, target); err != nil {
			return "", fmt.Errorf("run %s migrations for %s: %w", target, resolvedPath, err)
		}
	}
	if err := ProtectFiles(resolvedPath); err != nil {
		return "", err
	}

	return resolvedPath, nil
}

func validateDirectory(path string) error {
	directory := filepath.Dir(path)
	if directory == "" || directory == "." {
		return nil
	}
	info, err := os.Stat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("database parent %s must be a directory", directory)
	}
	if info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("database directory %s must not be group or world writable", directory)
	}
	return nil
}

func ResolvePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("database file is empty")
	}

	info, err := os.Lstat(path)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		return "", fmt.Errorf("database path %s must not be a symbolic link", path)
	case err == nil && info.IsDir():
		path = filepath.Join(path, "tdns.sqlite")
		childInfo, childErr := os.Lstat(path)
		if childErr == nil && childInfo.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("database path %s must not be a symbolic link", path)
		}
		if childErr == nil && !childInfo.Mode().IsRegular() {
			return "", fmt.Errorf("database path %s must be a regular file", path)
		}
		if childErr != nil && !errors.Is(childErr, os.ErrNotExist) {
			return "", childErr
		}
	case err == nil && info.Mode().IsRegular():
		return path, nil
	case err == nil:
		return "", fmt.Errorf("database path %s must be a regular file", path)
	case !errors.Is(err, os.ErrNotExist):
		return "", err
	}

	parent := filepath.Dir(path)
	if parent != "." && parent != "" {
		if err := os.MkdirAll(parent, 0o700); err != nil {
			return "", err
		}
	}

	return path, nil
}

func ProtectFiles(path string) error {
	for _, candidate := range []string{path, path + "-wal", path + "-shm"} {
		info, err := os.Lstat(candidate)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("database artifact %s must be a regular file", candidate)
		}
		if err := os.Chmod(candidate, 0o600); err != nil {
			return fmt.Errorf("protect database artifact %s: %w", candidate, err)
		}
	}
	return nil
}
