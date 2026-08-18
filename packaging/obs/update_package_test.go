package obs_test

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

const fixtureVersion = "1.2.3"

type fixtureRelease struct {
	root         string
	version      string
	apiBase      string
	releaseBase  string
	checksums    string
	archivePaths map[string]string
}

func TestUpdatePackage(t *testing.T) {
	requireCommands(t, "curl", "dpkg-source", "file", "git", "go", "gzip", "python3", "sha256sum", "tar")
	release := createFixtureRelease(t)

	t.Run("prepares consistent RPM and Debian sources", func(t *testing.T) {
		repo, obsDir, env := createCheckout(t, release)
		output, err := runUpdater(repo, obsDir, env, "--revision", "2", "v"+fixtureVersion)
		if err != nil {
			t.Fatalf("update package: %v\n%s", err, output)
		}

		assertContains(t, filepath.Join(obsDir, "tdns.spec"), "Version:        "+fixtureVersion)
		assertContains(t, filepath.Join(obsDir, "tdns.spec"), "Release:        2%{?dist}")
		assertContains(t, filepath.Join(obsDir, "tdns_"+fixtureVersion+"-2.dsc"), "Version: "+fixtureVersion+"-2")
		for _, name := range []string{
			"tdns_Linux_x86_64.tar.gz",
			"tdns_Linux_arm64.tar.gz",
			"tdns_Linux_armv7.tar.gz",
			"tdns_" + fixtureVersion + "_checksums.txt",
			"tdns_" + fixtureVersion + ".orig.tar.gz",
			"tdns_" + fixtureVersion + ".orig-amd64.tar.gz",
			"tdns_" + fixtureVersion + ".orig-arm64.tar.gz",
			"tdns_" + fixtureVersion + ".orig-armhf.tar.gz",
			"tdns_" + fixtureVersion + "-2.debian.tar.xz",
		} {
			if _, err := os.Stat(filepath.Join(obsDir, name)); err != nil {
				t.Errorf("expected OBS source %s: %v", name, err)
			}
		}
	})

	t.Run("prepares a local GoReleaser snapshot without a release tag", func(t *testing.T) {
		const developmentVersion = "1.2.4~dev.gabcdef0"
		localSnapshot := createFixtureReleaseWithVersion(t, developmentVersion)
		repo, obsDir, env := createCheckout(t, localSnapshot)
		localDist := filepath.Join(localSnapshot.root, "releases", "download", "v"+developmentVersion)
		output, err := runUpdater(
			repo,
			obsDir,
			env,
			"--local-dist", localDist,
			"--source-ref", "HEAD",
			"--revision", "3",
		)
		if err != nil {
			t.Fatalf("update package from local snapshot: %v\n%s", err, output)
		}

		assertContains(t, filepath.Join(obsDir, "tdns.spec"), "Version:        "+developmentVersion)
		assertContains(t, filepath.Join(obsDir, "tdns.spec"), "Release:        3%{?dist}")
		assertContains(t, filepath.Join(obsDir, "tdns.spec"), "Package development snapshot "+developmentVersion+" from HEAD")
		assertContains(t, filepath.Join(obsDir, "tdns_"+developmentVersion+"-3.dsc"), "Version: "+developmentVersion+"-3")
		assertContains(t, filepath.Join(obsDir, "tdns_"+developmentVersion+"_checksums.txt"), "tdns_Linux_x86_64.tar.gz")
	})

	t.Run("rejects a local snapshot whose binary version differs", func(t *testing.T) {
		repo, obsDir, env := createCheckout(t, release)
		localDist := t.TempDir()
		copyTree(t, filepath.Join(release.root, "releases", "download", "v"+fixtureVersion), localDist)
		if err := os.Rename(
			filepath.Join(localDist, "tdns_"+fixtureVersion+"_checksums.txt"),
			filepath.Join(localDist, "tdns_1.2.4~dev.gabcdef0_checksums.txt"),
		); err != nil {
			t.Fatal(err)
		}
		assertRejectedWithoutDestinationChange(
			t,
			repo,
			obsDir,
			env,
			"binary version does not match package version 1.2.4~dev.gabcdef0",
			"--local-dist", localDist,
			"--version", "1.2.4~dev.gabcdef0",
		)
	})

	t.Run("rejects a dirty TDNS checkout", func(t *testing.T) {
		repo, obsDir, env := createCheckout(t, release)
		writeFile(t, filepath.Join(repo, "untracked"), "dirty", 0o644)
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "TDNS Git checkout is dirty", "v"+fixtureVersion)
	})

	t.Run("rejects a dirty OBS checkout", func(t *testing.T) {
		repo, obsDir, env := createCheckout(t, release)
		env = append(env, "FAKE_OSC_DIRTY=1")
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "OBS package checkout is dirty", "v"+fixtureVersion)
	})

	t.Run("rejects a local tag that differs from GitHub", func(t *testing.T) {
		repo, obsDir, env := createCheckout(t, release)
		remote := filepath.Join(t.TempDir(), "remote.git")
		run(t, repo, nil, "git", "clone", "-q", "--bare", repo, remote)
		writeFile(t, filepath.Join(repo, "new-commit"), "different tag target\n", 0o644)
		run(t, repo, nil, "git", "add", "new-commit")
		run(t, repo, nil, "git", "commit", "-qm", "move local tag")
		run(t, repo, nil, "git", "tag", "-f", "v"+fixtureVersion)
		env = replaceEnvironment(env, "TDNS_GITHUB_GIT_URL", remote)
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "does not match the GitHub tag commit", "v"+fixtureVersion)
	})

	t.Run("rejects a missing release asset", func(t *testing.T) {
		broken := cloneFixtureRelease(t, release)
		writeReleaseJSON(t, broken, []string{
			broken.checksums,
			"tdns_Linux_x86_64.tar.gz",
			"tdns_Linux_arm64.tar.gz",
		})
		repo, obsDir, env := createCheckout(t, broken)
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "missing assets", "v"+fixtureVersion)
	})

	t.Run("rejects a release tag mismatch", func(t *testing.T) {
		broken := cloneFixtureRelease(t, release)
		apiPath := filepath.Join(broken.root, "repos", "jfardello", "tdns", "releases", "tags", "v"+fixtureVersion)
		contents, err := os.ReadFile(apiPath)
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, apiPath, strings.Replace(string(contents), `"tag_name":"v1.2.3"`, `"tag_name":"v9.9.9"`, 1), 0o644)
		repo, obsDir, env := createCheckout(t, broken)
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "does not match", "v"+fixtureVersion)
	})

	t.Run("rejects a missing checksum entry", func(t *testing.T) {
		broken := cloneFixtureRelease(t, release)
		manifestPath := filepath.Join(broken.root, "releases", "download", "v"+fixtureVersion, broken.checksums)
		contents, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		var retained []string
		for _, line := range strings.Split(string(contents), "\n") {
			if !strings.Contains(line, "tdns_Linux_armv7.tar.gz") {
				retained = append(retained, line)
			}
		}
		writeFile(t, manifestPath, strings.Join(retained, "\n"), 0o644)
		repo, obsDir, env := createCheckout(t, broken)
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "exactly one entry", "v"+fixtureVersion)
	})

	t.Run("rejects a checksum mismatch", func(t *testing.T) {
		broken := cloneFixtureRelease(t, release)
		archive := filepath.Join(broken.root, "releases", "download", "v"+fixtureVersion, "tdns_Linux_armv7.tar.gz")
		file, err := os.OpenFile(archive, os.O_APPEND|os.O_WRONLY, 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString("corrupt"); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		repo, obsDir, env := createCheckout(t, broken)
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "FAILED", "v"+fixtureVersion)
	})

	t.Run("rejects latest release URLs", func(t *testing.T) {
		repo, obsDir, env := createCheckout(t, release)
		env = append(env, "TDNS_RELEASE_BASE_URL=https://example.invalid/latest")
		assertRejectedWithoutDestinationChange(t, repo, obsDir, env, "must not contain latest", "v"+fixtureVersion)
	})

	t.Run("restores the checkout when osc addremove fails", func(t *testing.T) {
		repo, obsDir, env := createCheckout(t, release)
		oldSpec := "Version: old\n"
		writeFile(t, filepath.Join(obsDir, "tdns.spec"), oldSpec, 0o644)
		env = append(env, "FAKE_OSC_ADDREMOVE_FAIL=1")
		output, err := runUpdater(repo, obsDir, env, "v"+fixtureVersion)
		if err == nil {
			t.Fatalf("expected osc failure, got success:\n%s", output)
		}
		contents, readErr := os.ReadFile(filepath.Join(obsDir, "tdns.spec"))
		if readErr != nil || string(contents) != oldSpec {
			t.Fatalf("previous OBS source was not restored: contents=%q err=%v", contents, readErr)
		}
		assertContains(t, filepath.Join(obsDir, "sentinel"), "preserve")
	})
}

func createFixtureRelease(t *testing.T) fixtureRelease {
	return createFixtureReleaseWithVersion(t, fixtureVersion)
}

func createFixtureReleaseWithVersion(t *testing.T, version string) fixtureRelease {
	t.Helper()
	root := t.TempDir()
	downloadDir := filepath.Join(root, "releases", "download", "v"+version)
	if err := os.MkdirAll(downloadDir, 0o755); err != nil {
		t.Fatal(err)
	}

	program := filepath.Join(root, "fixture.go")
	writeFile(t, program, `package main
import (
    "fmt"
    "os"
    "path/filepath"
)
func main() {
    if len(os.Args) == 2 && os.Args[1] == "--version" {
        fmt.Println("tdns version `+version+`")
        return
    }
    if len(os.Args) == 4 && os.Args[1] == "man" && os.Args[2] == "--output-dir" {
        _ = os.MkdirAll(os.Args[3], 0755)
        _ = os.WriteFile(filepath.Join(os.Args[3], "tdns.1"), []byte(".TH TDNS 1\n"), 0644)
        return
    }
    os.Exit(2)
}
`, 0o644)

	targets := []struct {
		archive string
		goarch  string
		goarm   string
	}{
		{"tdns_Linux_x86_64.tar.gz", "amd64", ""},
		{"tdns_Linux_arm64.tar.gz", "arm64", ""},
		{"tdns_Linux_armv7.tar.gz", "arm", "7"},
	}

	archivePaths := make(map[string]string, len(targets))
	for _, target := range targets {
		binary := filepath.Join(root, "tdns-"+target.goarch+target.goarm)
		command := exec.Command("go", "build", "-trimpath", "-ldflags=-s -w", "-o", binary, program)
		command.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=linux", "GOARCH="+target.goarch)
		if target.goarm != "" {
			command.Env = append(command.Env, "GOARM="+target.goarm)
		}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("build %s fixture: %v\n%s", target.archive, err, output)
		}
		archivePath := filepath.Join(downloadDir, target.archive)
		createArchive(t, archivePath, binary)
		archivePaths[target.archive] = archivePath
	}

	checksums := "tdns_" + version + "_checksums.txt"
	names := make([]string, 0, len(archivePaths))
	for name := range archivePaths {
		names = append(names, name)
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		contents, err := os.ReadFile(archivePaths[name])
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(contents)
		fmt.Fprintf(&manifest, "%s  %s\n", hex.EncodeToString(digest[:]), name)
	}
	writeFile(t, filepath.Join(downloadDir, checksums), manifest.String(), 0o644)
	metadata, err := json.Marshal(map[string]string{"version": version})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(downloadDir, "metadata.json"), string(metadata), 0o644)

	release := fixtureRelease{
		root:         root,
		version:      version,
		apiBase:      "file://" + root,
		releaseBase:  "file://" + filepath.Join(root, "releases", "download"),
		checksums:    checksums,
		archivePaths: archivePaths,
	}
	writeReleaseJSON(t, release, append([]string{checksums}, names...))
	return release
}

func writeReleaseJSON(t *testing.T, release fixtureRelease, assets []string) {
	t.Helper()
	apiPath := filepath.Join(release.root, "repos", "jfardello", "tdns", "releases", "tags", "v"+release.version)
	if err := os.MkdirAll(filepath.Dir(apiPath), 0o755); err != nil {
		t.Fatal(err)
	}
	type asset struct {
		Name string `json:"name"`
	}
	payload := struct {
		TagName string  `json:"tag_name"`
		Draft   bool    `json:"draft"`
		Assets  []asset `json:"assets"`
	}{TagName: "v" + release.version}
	for _, name := range assets {
		payload.Assets = append(payload.Assets, asset{Name: name})
	}
	contents, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, apiPath, string(contents), 0o644)
}

func cloneFixtureRelease(t *testing.T, source fixtureRelease) fixtureRelease {
	t.Helper()
	destination := t.TempDir()
	copyTree(t, source.root, destination)
	return fixtureRelease{
		root:         destination,
		version:      source.version,
		apiBase:      "file://" + destination,
		releaseBase:  "file://" + filepath.Join(destination, "releases", "download"),
		checksums:    source.checksums,
		archivePaths: source.archivePaths,
	}
}

func createCheckout(t *testing.T, release fixtureRelease) (string, string, []string) {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repositorySource := filepath.Clean(filepath.Join(working, "..", ".."))
	repo := filepath.Join(t.TempDir(), "tdns")
	for _, relative := range []string{"packaging/common", "packaging/rpm", "packaging/debian", "packaging/obs/update-package.sh"} {
		copyTree(t, filepath.Join(repositorySource, relative), filepath.Join(repo, relative))
	}
	writeFile(t, filepath.Join(repo, "LICENSE"), "fixture license\n", 0o644)
	writeFile(t, filepath.Join(repo, "README.md"), "fixture readme\n", 0o644)
	run(t, repo, nil, "git", "init", "-q")
	run(t, repo, nil, "git", "config", "user.name", "Test User")
	run(t, repo, nil, "git", "config", "user.email", "test@example.invalid")
	run(t, repo, nil, "git", "add", ".")
	run(t, repo, nil, "git", "commit", "-qm", "fixture")
	if !strings.Contains(release.version, "~") {
		run(t, repo, nil, "git", "tag", "v"+release.version)
	}

	obsDir := filepath.Join(t.TempDir(), "obs")
	if err := os.MkdirAll(filepath.Join(obsDir, ".osc"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(obsDir, "sentinel"), "preserve\n", 0o644)

	binDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(binDir, "osc"), `#!/bin/sh
if [ "$1" = "status" ] && [ "${FAKE_OSC_DIRTY:-0}" = "1" ]; then
    echo "M dirty"
fi
if [ "$1" = "addremove" ] && [ "${FAKE_OSC_ADDREMOVE_FAIL:-0}" = "1" ]; then
    exit 1
fi
exit 0
`, 0o755)
	env := append(os.Environ(),
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"TDNS_GITHUB_API_BASE="+release.apiBase,
		"TDNS_GITHUB_GIT_URL="+repo,
		"TDNS_RELEASE_BASE_URL="+release.releaseBase,
		"SOURCE_DATE_EPOCH=1786320000",
	)
	return repo, obsDir, env
}

func runUpdater(repo, obsDir string, env []string, arguments ...string) (string, error) {
	args := append([]string{filepath.Join(repo, "packaging", "obs", "update-package.sh")}, arguments...)
	args = append(args, obsDir)
	command := exec.Command("sh", args...)
	command.Dir = repo
	command.Env = env
	output, err := command.CombinedOutput()
	return string(output), err
}

func assertRejectedWithoutDestinationChange(t *testing.T, repo, obsDir string, env []string, expected string, arguments ...string) {
	t.Helper()
	output, err := runUpdater(repo, obsDir, env, arguments...)
	if err == nil {
		t.Fatalf("expected updater rejection, got success:\n%s", output)
	}
	if !strings.Contains(output, expected) {
		t.Fatalf("rejection output %q does not contain %q", output, expected)
	}
	contents, readErr := os.ReadFile(filepath.Join(obsDir, "sentinel"))
	if readErr != nil || string(contents) != "preserve\n" {
		t.Fatalf("OBS destination changed after rejection: contents=%q err=%v", contents, readErr)
	}
	entries, readErr := os.ReadDir(obsDir)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(entries) != 2 {
		t.Fatalf("OBS destination contains %d entries after rejection, want 2", len(entries))
	}
}

func createArchive(t *testing.T, destination, binary string) {
	t.Helper()
	output, err := os.Create(destination)
	if err != nil {
		t.Fatal(err)
	}
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	addArchiveFile(t, tarWriter, "tdns", binary, 0o755)
	addArchiveBytes(t, tarWriter, "LICENSE", []byte("fixture license\n"), 0o644)
	addArchiveBytes(t, tarWriter, "README.md", []byte("fixture readme\n"), 0o644)
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
}

func addArchiveFile(t *testing.T, writer *tar.Writer, name, source string, mode int64) {
	t.Helper()
	file, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: info.Size()}); err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(writer, file); err != nil {
		t.Fatal(err)
	}
}

func addArchiveBytes(t *testing.T, writer *tar.Writer, name string, contents []byte, mode int64) {
	t.Helper()
	if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(contents))}); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(contents); err != nil {
		t.Fatal(err)
	}
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	info, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	if !info.IsDir() {
		contents, readErr := os.ReadFile(source)
		if readErr != nil {
			t.Fatal(readErr)
		}
		writeFile(t, destination, string(contents), info.Mode().Perm())
		return
	}
	if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(source)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		copyTree(t, filepath.Join(source, entry.Name()), filepath.Join(destination, entry.Name()))
	}
}

func writeFile(t *testing.T, path, contents string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), mode); err != nil {
		t.Fatal(err)
	}
}

func run(t *testing.T, directory string, env []string, name string, arguments ...string) {
	t.Helper()
	command := exec.Command(name, arguments...)
	command.Dir = directory
	if env != nil {
		command.Env = env
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("%s %s: %v\n%s", name, strings.Join(arguments, " "), err, output)
	}
}

func requireCommands(t *testing.T, commands ...string) {
	t.Helper()
	for _, command := range commands {
		if _, err := exec.LookPath(command); err != nil {
			t.Skipf("required integration-test command %s is unavailable", command)
		}
	}
	if runtime.GOOS != "linux" {
		t.Skip("OBS packaging integration test requires Linux binaries")
	}
}

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	updated := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			updated = append(updated, entry)
		}
	}
	return append(updated, prefix+value)
}

func assertContains(t *testing.T, path, expected string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), expected) {
		t.Fatalf("%s does not contain %q", path, expected)
	}
}
