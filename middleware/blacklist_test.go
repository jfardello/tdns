package middleware

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/armon/go-radix"
	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

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
