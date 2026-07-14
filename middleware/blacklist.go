package middleware

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/jfardello/tdns/sched"

	"strings"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"

	"github.com/armon/go-radix"
	"github.com/miekg/dns"

	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	gitconf "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/sirupsen/logrus"
)

const (
	BLAddr         = "0.0.0.0"
	BLMaxFuzziness = 300
)

type None struct{}

type WriteCounter struct {
	Total    uint64
	ReportMB int
	logger   *logrus.Entry
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	if wc.Total >= uint64(1024*1024*wc.ReportMB) {
		wc.PrintProgress()
		wc.ReportMB++
	}
	return n, nil
}

func (wc *WriteCounter) PrintProgress() {
	wc.logger.Infof("Downloaded %s", humanize.Bytes(wc.Total))
}

type BlackList struct {
	Enabled       bool
	Hole          *radix.Tree
	HoleFile      string
	externalFile  string
	externalRepo  string
	branch        string
	pullPeriod    string
	WhiteList     []string
	persistedList []string
	extraHosts    []string
	whiteRules    selectorSet
	DefaultAddr   string
	ctx           context.Context
	cancel        context.CancelFunc
	runtimeList   []string
	runtimeRules  selectorSet
	mu            sync.RWMutex
}

type BlacklistStatus struct {
	Enabled               bool     `json:"enabled"`
	File                  string   `json:"file"`
	ExternalFile          string   `json:"external_file,omitempty"`
	ExternalRepo          string   `json:"external_repo,omitempty"`
	ExternalRepoBranch    string   `json:"external_repo_branch,omitempty"`
	ExternalPullPeriod    string   `json:"external_pull_period,omitempty"`
	Excludes              []string `json:"excludes,omitempty"`
	PersistedExcludes     []string `json:"persisted_excludes,omitempty"`
	PersistedHosts        []string `json:"persisted_hosts,omitempty"`
	RuntimeWhitelist      []string `json:"runtime_whitelist,omitempty"`
	BlockfileTotalEntries int      `json:"blockfile_total_entries"`
}

func (bp *BlackList) Info() (string, Stage) {
	return "blacklist", PreRouting
}

func (bp *BlackList) Run(mess *Message) (message *Message, err error) {
	logger := log.GetLogger("middleware.BlackList", "Run")
	bp.mu.RLock()
	enabled := bp.Enabled
	hole := bp.Hole
	runtimeRules := bp.runtimeRules
	configRules := bp.whiteRules
	if len(runtimeRules.Domains) == 0 && len(runtimeRules.Labels) == 0 && len(bp.runtimeList) > 0 {
		runtimeRules = parseSelectors(bp.runtimeList)
	}
	if len(configRules.Domains) == 0 && len(configRules.Labels) == 0 && len(bp.WhiteList) > 0 {
		configRules = parseSelectors(bp.WhiteList)
	}
	bp.mu.RUnlock()

	if !enabled {
		logger.Debug("Blacklist disabled.")
		return mess, nil
	}
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}
	domain := normalizeDomainPattern(m.Question[0].Name)

	if matchesSuffix(domain, runtimeRules.Domains) || matchesAnyLabel(mess.Labels(), runtimeRules.Labels) {
		logger.Debugf("%s bypassed by runtime whitelist", domain)
		return mess, nil
	}
	if matchesSuffix(domain, configRules.Domains) || matchesAnyLabel(mess.Labels(), configRules.Labels) {
		logger.Debugf("%s bypassed by config whitelist", domain)
		return mess, nil
	}
	if hole == nil {
		logger.Debug("Blacklist hole tree not initialized yet.")
		return mess, nil
	}

	_, ok := hole.Get(domain)
	if ok {
		logger.Debugf("%s found in the list", domain)
		err := mess.AddValue("blocked", "1")
		rr, err := dns.NewRR(fmt.Sprintf("%s A 0.0.0.0", domain))
		if err != nil {
			return mess, err
		}
		m.Answer = append(m.Answer, rr)
		mess.SetMsg(m)
		mess.Resolved(true)
		return mess, nil
	}
	logger.Debugf("%s not found in the list", domain)
	return mess, nil

}

func (bp *BlackList) Config(c config.Config) error {
	bf := c.Blacklist.File
	ef := c.Blacklist.ExternalFile
	er := c.Blacklist.ExternalRepo
	branch := c.Blacklist.ExternalRepoBranch
	if branch == "" {
		branch = "master"
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	bp.Enabled = c.Blacklist.Enabled
	bp.ctx, bp.cancel = context.WithCancel(context.Background())
	if bf != "" {
		bp.HoleFile = bf
		bp.WhiteList = normalizeSelectorValues(c.Blacklist.Excludes)
		bp.persistedList = normalizeSelectorValues(c.Blacklist.PersistedExcludes)
		bp.extraHosts = normalizeDomainPatterns(c.Blacklist.ExtraHosts)
		mergedWhitelist := append(append([]string(nil), bp.WhiteList...), bp.persistedList...)
		bp.whiteRules = parseSelectors(mergedWhitelist)
		bp.externalFile = ef
		bp.externalRepo = er
		bp.DefaultAddr = BLAddr
		bp.branch = branch
		bp.pullPeriod = c.Blacklist.ExternalPullPeriod
		return nil
	}
	return errors.New("blacklist file is mandatory")
}

type GitLister interface {
	List(o *git.ListOptions) (rfs []*plumbing.Reference, err error)
}

type GitDownloader struct {
	externalFile string
	externalRepo string
	branch       string
	holeFile     string
	remote       GitLister
}

func newGitDownloader(externalFile, externalRepo, branch, holeFile string) *GitDownloader {
	return &GitDownloader{
		externalFile: externalFile,
		externalRepo: externalRepo,
		branch:       branch,
		holeFile:     holeFile,
		remote: git.NewRemote(memory.NewStorage(), &gitconf.RemoteConfig{
			Name: "origin",
			URLs: []string{externalRepo},
		}),
	}
}

func (gd *GitDownloader) RemoteHEAD() (string, error) {
	logger := log.GetLogger("blacklist", "RemoteHEAD")
	logger.Debugf("Checking remote HEAD... %s", gd.externalRepo)
	refs, err := gd.remote.List(&git.ListOptions{
		PeelingOption: git.AppendPeeled,
	})
	if err != nil {
		return "", err
	}
	for _, r := range refs {
		if r.Name().String() == fmt.Sprintf("refs/heads/%s", gd.branch) {
			return r.Hash().String(), nil
		}
	}
	return "", errors.New("could not find remote HEAD")
}

func (gd *GitDownloader) stateFile() string {
	return gd.holeFile + ".state"
}

func (gd *GitDownloader) ReadLastHash() string {
	b, err := os.ReadFile(gd.stateFile())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (gd *GitDownloader) WriteLastHash(h string) error {
	return os.WriteFile(gd.stateFile(), []byte(h+"\n"), 0644)
}

func (gd *GitDownloader) GithubRaw(file *os.File, ref string) (uint64, error) {
	owner, repo, err := parseGitHub(gd.externalRepo)
	if err != nil {
		return 0, err
	}
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		owner, repo, url.PathEscape(gd.externalFile), ref)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github.v3.raw")

	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "token "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, bytes.TrimSpace(slurp))
	}
	counter := &WriteCounter{
		logger:   log.GetLogger("middleware.BlackList", "Download"),
		ReportMB: 1,
	}
	if _, err = io.Copy(file, io.TeeReader(resp.Body, counter)); err != nil {
		return counter.Total, err
	}
	return counter.Total, nil

}

func (bp *BlackList) Download() error {
	logger := log.GetLogger("middleware.BlackList", "Download")
	bp.mu.RLock()
	externalFile := bp.externalFile
	externalRepo := bp.externalRepo
	branch := bp.branch
	holeFile := bp.HoleFile
	bp.mu.RUnlock()

	if externalFile != "" && externalRepo != "" {
		downloader := newGitDownloader(externalFile, externalRepo, branch, holeFile)
		head, err := downloader.RemoteHEAD()
		if err != nil {
			return fmt.Errorf("could not query remote: %w", err)
		}
		last := downloader.ReadLastHash()
		if head == last {
			logger.Info("No changes to remote!")
			return nil
		}
		logger.Info("Downloading remote hosts file.")
		f, err := os.OpenFile(holeFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		if err != nil {
			logger.Error(err)
			return err
		}
		defer func() {
			_ = f.Close()
		}()
		total, err := downloader.GithubRaw(f, head)
		if err != nil {
			return err
		}
		logger.Infof("Downloaded %s from remote hosts file.", humanize.Bytes(total))
		if err := downloader.WriteLastHash(head); err != nil {
			return fmt.Errorf("could not persist blacklist state: %w", err)
		}
	}
	return nil
}

func (bp *BlackList) reloadHole() error {
	bp.mu.RLock()
	holeFile := bp.HoleFile
	configWhiteList := append([]string(nil), bp.whiteRules.Domains...)
	extraHosts := append([]string(nil), bp.extraHosts...)
	bp.mu.RUnlock()

	readFile, err := os.OpenFile(holeFile, os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer func(readFile *os.File) {
		err := readFile.Close()
		if err != nil {
			logger := log.GetLogger("blacklist", "Init/deferred")
			logger.Errorf("Error closing file %s: %s", bp.HoleFile, err.Error())
		}
	}(readFile)
	scanner := bufio.NewScanner(readFile)
	hole := radix.New()
OUTER:
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "#") {
			continue
		}

		fields := strings.Fields(s)
		if len(fields) > 1 {
			domain := normalizeDomainPattern(fields[1])
			if domain == "" {
				continue
			}
			if matchesSuffix(domain, configWhiteList) {
				continue OUTER
			}
			hole.Insert(domain, None{})
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	for _, each := range extraHosts {
		if each == "" || matchesSuffix(each, configWhiteList) {
			continue
		}
		hole.Insert(each, None{})
	}

	bp.mu.Lock()
	bp.Hole = hole
	bp.mu.Unlock()
	return nil
}

func (bp *BlackList) Init() error {
	err := bp.Download()
	if err != nil {
		return err
	}
	bp.mu.RLock()
	pullPeriod := bp.pullPeriod
	ctx := bp.ctx
	bp.mu.RUnlock()

	if err := bp.reloadHole(); err != nil {
		return err
	}

	cf := config.GetRunningConfig()
	mf := int64(BLMaxFuzziness * time.Second)
	if pullPeriod != "" {
		name := "BlackListDownloader"
		t := sched.Task{
			Name: name,
			Fn: sched.FuzzyTask(name, ctx, mf, func(context.Context) {
				logger := log.GetLogger("blacklist", name)
				err := bp.Download()
				if err != nil {
					logger.Error(err.Error())
				}
			}),
			Expr: cf.Blacklist.ExternalPullPeriod,
		}
		sched.Add(t)
	}

	return nil

}

func (bp *BlackList) SetEnabled(state bool) {
	bp.mu.Lock()
	bp.Enabled = state
	bp.mu.Unlock()
}

func (bp *BlackList) IsEnabled() bool {
	bp.mu.RLock()
	defer bp.mu.RUnlock()
	return bp.Enabled
}

func (bp *BlackList) AddRuntimeWhitelist(domains []string) error {
	normalized := normalizeSelectorValues(domains)
	if len(normalized) == 0 {
		return errors.New("at least one valid domain is required")
	}

	bp.mu.Lock()
	defer bp.mu.Unlock()

	seen := make(map[string]struct{}, len(bp.runtimeList))
	for _, each := range bp.runtimeList {
		seen[each] = struct{}{}
	}

	added := 0
	for _, domain := range normalized {
		if _, ok := seen[domain]; ok {
			continue
		}
		bp.runtimeList = append(bp.runtimeList, domain)
		seen[domain] = struct{}{}
		added++
	}

	if added == 0 {
		return errors.New("all runtime whitelist values are already present")
	}

	bp.runtimeRules = parseSelectors(bp.runtimeList)

	return nil
}

func (bp *BlackList) CountBlockfileEntries() (int, error) {
	bp.mu.RLock()
	holeFile := bp.HoleFile
	bp.mu.RUnlock()

	readFile, err := os.Open(holeFile)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = readFile.Close()
	}()

	count := 0
	scanner := bufio.NewScanner(readFile)
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "#") {
			continue
		}
		if len(strings.Fields(s)) > 1 {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return count, nil
}

func (bp *BlackList) Status() (BlacklistStatus, error) {
	bp.mu.RLock()
	status := BlacklistStatus{
		Enabled:            bp.Enabled,
		File:               bp.HoleFile,
		ExternalFile:       bp.externalFile,
		ExternalRepo:       bp.externalRepo,
		ExternalRepoBranch: bp.branch,
		ExternalPullPeriod: bp.pullPeriod,
		Excludes:           append([]string(nil), bp.WhiteList...),
		PersistedExcludes:  append([]string(nil), bp.persistedList...),
		PersistedHosts:     append([]string(nil), bp.extraHosts...),
		RuntimeWhitelist:   append([]string(nil), bp.runtimeList...),
	}
	bp.mu.RUnlock()

	totalEntries, err := bp.CountBlockfileEntries()
	if err != nil {
		return BlacklistStatus{}, err
	}
	status.BlockfileTotalEntries = totalEntries

	return status, nil
}

func (bp *BlackList) ReplacePersistedHosts(domains []string) error {
	normalized := normalizeDomainPatterns(domains)
	bp.mu.Lock()
	bp.extraHosts = append([]string(nil), normalized...)
	bp.mu.Unlock()
	return bp.reloadHole()
}

func (bp *BlackList) ReplacePersistedExcludes(values []string) error {
	normalized := normalizeSelectorValues(values)
	bp.mu.Lock()
	bp.persistedList = append([]string(nil), normalized...)
	mergedWhitelist := append(append([]string(nil), bp.WhiteList...), bp.persistedList...)
	bp.whiteRules = parseSelectors(mergedWhitelist)
	bp.mu.Unlock()
	return bp.reloadHole()
}

func parseGitHub(uri string) (owner, repo string, err error) {
	// Accept https://github.com/owner/repo(.git) or git@github.com:owner/repo.git
	if strings.HasPrefix(uri, "http") {
		u, err := url.Parse(uri)
		if err != nil {
			return "", "", err
		}
		parts := strings.Split(strings.TrimSuffix(u.Path, ".git"), "/")
		if len(parts) < 3 {
			return "", "", fmt.Errorf("unexpected URL form")
		}
		owner, repo = parts[1], parts[2]
	} else {
		return "", "", fmt.Errorf("unrecognised GitHub URL")
	}
	return owner, repo, nil
}

func normalizeDomainPattern(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	return strings.TrimSuffix(value, ".")
}

func normalizeDomainPatterns(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		domain := normalizeDomainPattern(value)
		if domain == "" {
			continue
		}
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		normalized = append(normalized, domain)
	}
	return normalized
}

func matchesSuffix(domain string, values []string) bool {
	for _, each := range values {
		if strings.HasSuffix(domain, each) {
			return true
		}
	}
	return false
}
