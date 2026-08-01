package blocklist

import (
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var (
	githubOwnerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,37}[A-Za-z0-9])?$`)
	githubRepoPattern  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// Source identifies one hosts file in a public or token-accessible GitHub repository.
type Source struct {
	Owner  string
	Repo   string
	Branch string
	File   string
}

// ParseSource validates the externally configured repository, branch, and file path.
func ParseSource(repository, branch, file string) (Source, error) {
	u, err := url.Parse(repository)
	if err != nil {
		return Source{}, newError(KindInvalid, "source", fmt.Errorf("invalid repository URL"))
	}
	if u.Scheme != "https" || !strings.EqualFold(u.Hostname(), "github.com") || u.Port() != "" ||
		u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return Source{}, newError(KindInvalid, "source", fmt.Errorf("repository must be an HTTPS github.com URL without credentials, port, query, or fragment"))
	}

	repositoryPath := strings.TrimSuffix(strings.Trim(u.EscapedPath(), "/"), ".git")
	parts := strings.Split(repositoryPath, "/")
	if len(parts) != 2 {
		return Source{}, newError(KindInvalid, "source", fmt.Errorf("repository path must contain exactly an owner and repository"))
	}
	owner, err := url.PathUnescape(parts[0])
	if err != nil || !githubOwnerPattern.MatchString(owner) {
		return Source{}, newError(KindInvalid, "source", fmt.Errorf("invalid GitHub owner"))
	}
	repo, err := url.PathUnescape(parts[1])
	if err != nil || !githubRepoPattern.MatchString(repo) || repo == "." || repo == ".." {
		return Source{}, newError(KindInvalid, "source", fmt.Errorf("invalid GitHub repository"))
	}

	branch = strings.TrimSpace(branch)
	if branch == "" {
		branch = "master"
	}
	if err := validateBranch(branch); err != nil {
		return Source{}, newError(KindInvalid, "source", err)
	}

	file = strings.TrimSpace(file)
	if err := validateFilePath(file); err != nil {
		return Source{}, newError(KindInvalid, "source", err)
	}

	return Source{Owner: owner, Repo: repo, Branch: branch, File: file}, nil
}

func validateBranch(branch string) error {
	if len(branch) > 255 || strings.HasPrefix(branch, ".") || strings.HasSuffix(branch, ".") ||
		strings.HasPrefix(branch, "/") || strings.HasSuffix(branch, "/") || strings.Contains(branch, "//") ||
		strings.Contains(branch, "..") || strings.Contains(branch, "@{") || strings.HasSuffix(branch, ".lock") {
		return fmt.Errorf("invalid Git branch")
	}
	for _, r := range branch {
		if r < 0x20 || r == 0x7f || strings.ContainsRune(" ~^:?*[\\", r) {
			return fmt.Errorf("invalid Git branch")
		}
	}
	for _, part := range strings.Split(branch, "/") {
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return fmt.Errorf("invalid Git branch")
		}
	}
	return nil
}

func validateFilePath(file string) error {
	if file == "" || len(file) > 1024 || strings.HasPrefix(file, "/") || strings.Contains(file, `\`) ||
		path.Clean(file) != file {
		return fmt.Errorf("external file must be a normalized relative path")
	}
	for _, part := range strings.Split(file, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("external file contains an invalid path component")
		}
	}
	return nil
}
