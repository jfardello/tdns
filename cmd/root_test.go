package cmd

import "testing"

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
