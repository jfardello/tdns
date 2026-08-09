package cmd

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

const deprecatedSQLiteEmbedWarning = "unnecessarily importing github.com/ncruces/go-sqlite3/embed"

func TestFormatVersionUsesInjectedMetadata(t *testing.T) {
	oldVersion := ver
	oldCommit := gitcommit
	oldDate := compiledate
	t.Cleanup(func() {
		ver = oldVersion
		gitcommit = oldCommit
		compiledate = oldDate
	})

	version := "v1.2.3"
	commit := "abc123"
	date := "2026-06-09T12:00:00Z"
	ver = &version
	gitcommit = &commit
	compiledate = &date

	got := formatVersion()
	want := "tdns version v1.2.3\ncommit abc123\nbuilt 2026-06-09T12:00:00Z\n"
	if got != want {
		t.Fatalf("formatVersion() = %q, want %q", got, want)
	}
}

func TestFormatVersionUsesDefaults(t *testing.T) {
	oldVersion := ver
	oldCommit := gitcommit
	oldDate := compiledate
	t.Cleanup(func() {
		ver = oldVersion
		gitcommit = oldCommit
		compiledate = oldDate
	})

	ver = nil
	gitcommit = nil
	compiledate = nil

	got := formatVersion()
	want := "tdns version dev\ncommit none\nbuilt unknown\n"
	if got != want {
		t.Fatalf("formatVersion() = %q, want %q", got, want)
	}
}

func TestCLIStartupDoesNotEmitSQLiteEmbedWarning(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "version", args: []string{"--version"}, want: "tdns version test"},
		{name: "help", args: []string{"--help"}, want: "Usage:"},
		{name: "man", args: []string{"man"}, want: `.TH "TDNS" "1"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{"-test.run=TestCLIHelperProcess", "--"}, tt.args...)
			command := exec.Command(os.Args[0], args...)
			command.Env = append(os.Environ(), "TDNS_CLI_HELPER_PROCESS=1")

			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("run CLI subprocess: %v\n%s", err, output)
			}
			if strings.Contains(string(output), deprecatedSQLiteEmbedWarning) {
				t.Fatalf("CLI emitted deprecated SQLite embed warning:\n%s", output)
			}
			if !strings.Contains(string(output), tt.want) {
				t.Fatalf("CLI output does not contain %q:\n%s", tt.want, output)
			}
		})
	}
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("TDNS_CLI_HELPER_PROCESS") != "1" {
		return
	}

	separator := 0
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i + 1
			break
		}
	}
	version, commit, date := "test", "test-commit", "test-date"
	ver, gitcommit, compiledate = &version, &commit, &date
	rootCmd.SetArgs(os.Args[separator:])
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}
