package middleware

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/armon/go-radix"
	"github.com/jfardello/tdns/config"
	internalblocklist "github.com/jfardello/tdns/internal/blocklist"
	"github.com/miekg/dns"
)

type fakeBlocklistIngester struct {
	content string
	result  internalblocklist.Result
	err     error
}

func (f fakeBlocklistIngester) Refresh(_ context.Context, _ internalblocklist.Source, destination, _ string, validate internalblocklist.Validator) (internalblocklist.Result, error) {
	if f.err != nil {
		return internalblocklist.Result{}, f.err
	}
	temporary, err := os.CreateTemp(filepath.Dir(destination), ".fake-blocklist-*")
	if err != nil {
		return internalblocklist.Result{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err := temporary.WriteString(f.content); err != nil {
		_ = temporary.Close()
		return internalblocklist.Result{}, err
	}
	if err := temporary.Close(); err != nil {
		return internalblocklist.Result{}, err
	}
	entries, err := validate(context.Background(), temporaryPath, internalblocklist.DefaultLimits())
	if err != nil {
		return internalblocklist.Result{}, &internalblocklist.Error{Kind: internalblocklist.KindInvalid, Stage: "validate candidate", Err: err}
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return internalblocklist.Result{}, err
	}
	result := f.result
	result.Changed = true
	result.Entries = entries
	return result, nil
}

func TestBlackListStatusCountsRawBlockfileEntries(t *testing.T) {
	blockfile := filepath.Join(t.TempDir(), "blacklist.txt")
	content := "# comment\n0.0.0.0 ads.example.com\ninvalid-line\n127.0.0.1 tracker.example.net\n"
	if err := os.WriteFile(blockfile, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	bp := &BlackList{
		Enabled:     true,
		HoleFile:    blockfile,
		WhiteList:   []string{"allowed.example"},
		runtimeList: []string{"runtime.example"},
	}

	status, err := bp.Status()
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}

	if status.BlockfileTotalEntries != 2 {
		t.Fatalf("expected 2 raw entries, got %d", status.BlockfileTotalEntries)
	}
	if len(status.Excludes) != 1 || status.Excludes[0] != "allowed.example" {
		t.Fatalf("unexpected excludes: %#v", status.Excludes)
	}
	if len(status.RuntimeWhitelist) != 1 || status.RuntimeWhitelist[0] != "runtime.example" {
		t.Fatalf("unexpected runtime whitelist: %#v", status.RuntimeWhitelist)
	}
}

func TestBlackListDownloadAtomicallyUpdatesLiveTree(t *testing.T) {
	dir := t.TempDir()
	blockfile := filepath.Join(dir, "blacklist.txt")
	if err := os.WriteFile(blockfile, []byte("0.0.0.0 old.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := internalblocklist.Source{Owner: "acme", Repo: "lists", Branch: "main", File: "hosts"}
	bp := &BlackList{
		HoleFile: blockfile,
		source:   &source,
		ingester: fakeBlocklistIngester{
			content: "0.0.0.0 new.example alias.example # comment\n",
			result:  internalblocklist.Result{Revision: testBlacklistRevision},
		},
	}
	bp.Hole = radix.New()
	bp.Hole.Insert("old.example", None{})

	if err := bp.Download(); err != nil {
		t.Fatalf("Download error: %v", err)
	}
	if _, ok := bp.Hole.Get("new.example"); !ok {
		t.Fatal("new domain missing from live tree")
	}
	if _, ok := bp.Hole.Get("alias.example"); !ok {
		t.Fatal("additional hosts entry missing from live tree")
	}
	if _, ok := bp.Hole.Get("old.example"); ok {
		t.Fatal("old domain retained in live tree")
	}
	body, err := os.ReadFile(blockfile)
	if err != nil || string(body) != "0.0.0.0 new.example alias.example # comment\n" {
		t.Fatalf("active file = %q, error %v", body, err)
	}
	if got := internalblocklist.ReadRevision(blockfile + ".state"); got != testBlacklistRevision {
		t.Fatalf("revision state = %q", got)
	}
}

func TestBlackListDownloadRejectsMalformedCandidateWithoutChangingActiveData(t *testing.T) {
	blockfile := filepath.Join(t.TempDir(), "blacklist.txt")
	oldContent := "0.0.0.0 old.example\n"
	if err := os.WriteFile(blockfile, []byte(oldContent), 0o600); err != nil {
		t.Fatal(err)
	}
	source := internalblocklist.Source{Owner: "acme", Repo: "lists", Branch: "main", File: "hosts"}
	bp := &BlackList{
		HoleFile: blockfile,
		source:   &source,
		ingester: fakeBlocklistIngester{content: "malformed\n"},
		Hole:     radix.New(),
	}
	bp.Hole.Insert("old.example", None{})

	err := bp.Download()
	if err == nil || internalblocklist.KindOf(err) != internalblocklist.KindInvalid {
		t.Fatalf("Download error = %v, kind %q", err, internalblocklist.KindOf(err))
	}
	body, readErr := os.ReadFile(blockfile)
	if readErr != nil || string(body) != oldContent {
		t.Fatalf("active file = %q, error %v", body, readErr)
	}
	if _, ok := bp.Hole.Get("old.example"); !ok {
		t.Fatal("live tree changed after validation failure")
	}
}

func TestBuildHoleEnforcesCandidateEntryLimit(t *testing.T) {
	blockfile := filepath.Join(t.TempDir(), "candidate")
	if err := os.WriteFile(blockfile, []byte("0.0.0.0 one.example\n0.0.0.0 two.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	limits := internalblocklist.DefaultLimits()
	limits.MaxEntries = 1
	bp := &BlackList{}
	if _, _, err := bp.buildHole(context.Background(), blockfile, true, limits); err == nil {
		t.Fatal("expected candidate entry limit error")
	}
}

func TestCurrentBlocklistRevisionRequiresNonEmptyRegularFile(t *testing.T) {
	dir := t.TempDir()
	blockfile := filepath.Join(dir, "blacklist.txt")
	if err := internalblocklist.WriteRevision(blockfile+".state", testBlacklistRevision); err != nil {
		t.Fatal(err)
	}
	if got := currentBlocklistRevision(blockfile); got != "" {
		t.Fatalf("revision for missing file = %q", got)
	}
	if err := os.WriteFile(blockfile, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := currentBlocklistRevision(blockfile); got != "" {
		t.Fatalf("revision for empty file = %q", got)
	}
	if err := os.WriteFile(blockfile, []byte("0.0.0.0 ads.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := currentBlocklistRevision(blockfile); got != testBlacklistRevision {
		t.Fatalf("revision for valid file = %q", got)
	}
}

func TestBlackListDownloadPreservesLiveDataOnRemoteFailure(t *testing.T) {
	blockfile := filepath.Join(t.TempDir(), "blacklist.txt")
	if err := os.WriteFile(blockfile, []byte("0.0.0.0 old.example\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	source := internalblocklist.Source{Owner: "acme", Repo: "lists", Branch: "main", File: "hosts"}
	bp := &BlackList{
		HoleFile: blockfile,
		source:   &source,
		ingester: fakeBlocklistIngester{err: &internalblocklist.Error{Kind: internalblocklist.KindTimeout, Stage: "request", Err: errors.New("timed out")}},
		Hole:     radix.New(),
	}
	bp.Hole.Insert("old.example", None{})
	if err := bp.Download(); err == nil || internalblocklist.KindOf(err) != internalblocklist.KindTimeout {
		t.Fatalf("Download error = %v", err)
	}
	if _, ok := bp.Hole.Get("old.example"); !ok {
		t.Fatal("live tree changed after remote failure")
	}
}

const testBlacklistRevision = "0123456789abcdef0123456789abcdef01234567"

func TestBlackListRunBypassesRuntimeWhitelistSuffixes(t *testing.T) {
	bp := &BlackList{
		Enabled: true,
		Hole:    radix.New(),
	}
	bp.Hole.Insert("ads.example.com", None{})

	if err := bp.AddRuntimeWhitelist([]string{"EXAMPLE.COM."}); err != nil {
		t.Fatalf("AddRuntimeWhitelist error: %v", err)
	}

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("ads.example.com"), dns.TypeA)

	request := &Message{}
	request.SetMsg(msg)
	request.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		Values: map[string]string{},
	}))

	response, err := bp.Run(request)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if response.IsResolved() {
		t.Fatal("expected runtime-whitelisted domain not to be blocked")
	}
	if value, ok := response.GetValue("blocked"); ok || value != "" {
		t.Fatalf("expected blocked context value to be absent, got %q", value)
	}
}

func TestBlackListRunBypassesLabelWhitelist(t *testing.T) {
	bp := &BlackList{
		Enabled:    true,
		Hole:       radix.New(),
		WhiteList:  []string{"label:trusted"},
		whiteRules: parseSelectors([]string{"label:trusted"}),
	}
	bp.Hole.Insert("ads.example.com", None{})

	msg := new(dns.Msg)
	msg.SetQuestion(dns.Fqdn("ads.example.com"), dns.TypeA)

	request := &Message{}
	request.SetMsg(msg)
	request.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		Labels: []string{"trusted"},
		Values: map[string]string{},
	}))

	response, err := bp.Run(request)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if response.IsResolved() {
		t.Fatal("expected label-whitelisted domain not to be blocked")
	}
	if value, ok := response.GetValue("blocked"); ok || value != "" {
		t.Fatalf("expected blocked context value to be absent, got %q", value)
	}
}
