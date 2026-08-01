package middleware

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jfardello/tdns/config"
	internalblocklist "github.com/jfardello/tdns/internal/blocklist"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/sched"

	"github.com/armon/go-radix"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

const (
	BLAddr         = "0.0.0.0"
	BLMaxFuzziness = 300
)

type None struct{}

type blocklistIngester interface {
	Refresh(context.Context, internalblocklist.Source, string, string, internalblocklist.Validator) (internalblocklist.Result, error)
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
	refreshMu     sync.Mutex
	source        *internalblocklist.Source
	ingester      blocklistIngester
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
	if (ef == "") != (er == "") {
		return errors.New("blacklist external_file and external_repo must be configured together")
	}
	var source *internalblocklist.Source
	if ef != "" {
		parsed, err := internalblocklist.ParseSource(er, branch, ef)
		if err != nil {
			return err
		}
		source = &parsed
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
		bp.source = source
		if source != nil {
			bp.ingester = internalblocklist.NewClient(os.Getenv("GITHUB_TOKEN"))
		} else {
			bp.ingester = nil
		}
		return nil
	}
	return errors.New("blacklist file is mandatory")
}

func (bp *BlackList) Download() error {
	return bp.refresh(context.Background())
}

func (bp *BlackList) refresh(ctx context.Context) error {
	bp.refreshMu.Lock()
	defer bp.refreshMu.Unlock()
	started := time.Now()
	logger := log.GetLogger("middleware.BlackList", "Refresh")
	bp.mu.RLock()
	source := bp.source
	ingester := bp.ingester
	holeFile := bp.HoleFile
	bp.mu.RUnlock()

	if source == nil || ingester == nil {
		return nil
	}

	var prepared *radix.Tree
	result, err := ingester.Refresh(ctx, *source, holeFile, currentBlocklistRevision(holeFile),
		func(validationContext context.Context, candidate string, limits internalblocklist.Limits) (int, error) {
			var parseErr error
			var entries int
			prepared, entries, parseErr = bp.buildHole(validationContext, candidate, true, limits)
			return entries, parseErr
		})
	if err != nil {
		resultName := string(internalblocklist.KindOf(err))
		recordBlacklistRefresh(started, resultName, internalblocklist.Result{}, 0)
		logger.WithFields(logrus.Fields{
			"result":      resultName,
			"duration_ms": time.Since(started).Milliseconds(),
		}).Error(err)
		return err
	}
	if !result.Changed {
		bp.mu.RLock()
		activeEntries := 0
		if bp.Hole != nil {
			activeEntries = bp.Hole.Len()
		}
		bp.mu.RUnlock()
		recordBlacklistRefresh(started, "unchanged", result, activeEntries)
		logger.WithField("result", "unchanged").Info("Remote blocklist is current.")
		return nil
	}

	bp.mu.Lock()
	bp.Hole = prepared
	activeEntries := prepared.Len()
	bp.mu.Unlock()
	recordBlacklistRefresh(started, "success", result, activeEntries)
	logger.WithFields(logrus.Fields{
		"result":             "success",
		"duration_ms":        time.Since(started).Milliseconds(),
		"compressed_bytes":   result.CompressedBytes,
		"uncompressed_bytes": result.UncompressedBytes,
		"entries":            result.Entries,
	}).Info("Remote blocklist refreshed.")
	if err := internalblocklist.WriteRevision(holeFile+".state", result.Revision); err != nil {
		logger.WithField("result", "state_error").Warn("Remote blocklist revision state could not be persisted; the valid blocklist remains active.")
	}
	return nil
}

func (bp *BlackList) reloadHole() error {
	bp.refreshMu.Lock()
	defer bp.refreshMu.Unlock()
	return bp.reloadHoleLocked()
}

func (bp *BlackList) reloadHoleLocked() error {
	bp.mu.RLock()
	holeFile := bp.HoleFile
	bp.mu.RUnlock()

	file, err := os.OpenFile(holeFile, os.O_RDONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	hole, _, err := bp.buildHole(context.Background(), holeFile, false, internalblocklist.DefaultLimits())
	if err != nil {
		return err
	}
	bp.mu.Lock()
	bp.Hole = hole
	activeEntries := hole.Len()
	bp.mu.Unlock()
	blacklistActiveEntries.Set(float64(activeEntries))
	return nil
}

func (bp *BlackList) buildHole(ctx context.Context, filename string, strict bool, limits internalblocklist.Limits) (*radix.Tree, int, error) {
	bp.mu.RLock()
	configWhiteList := append([]string(nil), bp.whiteRules.Domains...)
	extraHosts := append([]string(nil), bp.extraHosts...)
	bp.mu.RUnlock()

	readFile, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}
	defer readFile.Close()
	scanner := bufio.NewScanner(readFile)
	scanner.Buffer(make([]byte, 64*1024), limits.MaxLineBytes)
	hole := radix.New()
	entries := 0
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if lineNumber%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, 0, err
			}
		}
		s := strings.TrimSpace(scanner.Text())
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		fields := strings.Fields(s)
		if len(fields) < 2 {
			if strict {
				return nil, 0, fmt.Errorf("line %d is not a hosts entry", lineNumber)
			}
			continue
		}
		if strict {
			if _, err := netip.ParseAddr(fields[0]); err != nil {
				return nil, 0, fmt.Errorf("line %d has an invalid address", lineNumber)
			}
		} else {
			fields = fields[:2]
		}
		for _, field := range fields[1:] {
			if strings.HasPrefix(field, "#") {
				break
			}
			domain := normalizeDomainPattern(field)
			if domain == "" {
				if strict {
					return nil, 0, fmt.Errorf("line %d has an empty domain", lineNumber)
				}
				continue
			}
			if strict {
				if _, ok := dns.IsDomainName(dns.Fqdn(domain)); !ok {
					return nil, 0, fmt.Errorf("line %d has an invalid domain", lineNumber)
				}
				entries++
				if entries > limits.MaxEntries {
					return nil, 0, fmt.Errorf("candidate exceeds the entry limit")
				}
			}
			if !matchesSuffix(domain, configWhiteList) {
				hole.Insert(domain, None{})
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, 0, err
	}
	if strict && entries == 0 {
		return nil, 0, errors.New("candidate contains no usable hosts entries")
	}
	for _, each := range extraHosts {
		if each == "" || matchesSuffix(each, configWhiteList) {
			continue
		}
		hole.Insert(each, None{})
	}
	return hole, entries, nil
}

func currentBlocklistRevision(holeFile string) string {
	info, err := os.Stat(holeFile)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return ""
	}
	return internalblocklist.ReadRevision(holeFile + ".state")
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

	bp.mu.RLock()
	holeInitialized := bp.Hole != nil
	bp.mu.RUnlock()
	if !holeInitialized {
		if err := bp.reloadHole(); err != nil {
			return err
		}
	}

	cf := config.GetRunningConfig()
	mf := int64(BLMaxFuzziness * time.Second)
	if pullPeriod != "" {
		name := "BlackListDownloader"
		t := sched.Task{
			Name: name,
			Fn: sched.FuzzyTask(name, ctx, mf, func(taskContext context.Context) {
				logger := log.GetLogger("blacklist", name)
				err := bp.refresh(taskContext)
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
	bp.refreshMu.Lock()
	defer bp.refreshMu.Unlock()
	normalized := normalizeDomainPatterns(domains)
	bp.mu.Lock()
	bp.extraHosts = append([]string(nil), normalized...)
	bp.mu.Unlock()
	return bp.reloadHoleLocked()
}

func (bp *BlackList) ReplacePersistedExcludes(values []string) error {
	bp.refreshMu.Lock()
	defer bp.refreshMu.Unlock()
	normalized := normalizeSelectorValues(values)
	bp.mu.Lock()
	bp.persistedList = append([]string(nil), normalized...)
	mergedWhitelist := append(append([]string(nil), bp.WhiteList...), bp.persistedList...)
	bp.whiteRules = parseSelectors(mergedWhitelist)
	bp.mu.Unlock()
	return bp.reloadHoleLocked()
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
