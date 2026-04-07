package middleware

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

var (
	DefaultZenTimeMinutes int = 20
	DefaultZenDomains         = []string{"wwww.instagram.com", "x.com", "www.facebook.com"}
)

type ZenMode struct {
	ZenFile            string
	enabled            bool
	initDone           bool
	timerMu            chan struct{}
	stateMu            sync.RWMutex
	c                  config.Config
	Hosts              map[string]string
	configuredHosts    map[string]string
	persistedHosts     map[string]string
	runtimeHosts       map[string]string
	configuredExcludes []string
	persistedExcludes  []string
	excludeRules       selectorSet
	labels             []string
	startedAt          time.Time
	endsAt             time.Time
}

type ZenModeStatus struct {
	Enabled            bool     `json:"enabled"`
	File               string   `json:"file,omitempty"`
	DurationMinutes    int      `json:"duration_minutes"`
	ConfiguredDomains  []string `json:"configured_domains,omitempty"`
	PersistedDomains   []string `json:"persisted_domains,omitempty"`
	ConfiguredExcludes []string `json:"configured_excludes,omitempty"`
	PersistedExcludes  []string `json:"persisted_excludes,omitempty"`
	Labels             []string `json:"labels,omitempty"`
	RuntimeDomains     []string `json:"runtime_domains,omitempty"`
	StartedAt          string   `json:"started_at,omitempty"`
	EndsAt             string   `json:"ends_at,omitempty"`
	RemainingSeconds   int64    `json:"remaining_seconds"`
}

func (z *ZenMode) Info() (string, Stage) {
	return "zen-mode", PreRouting
}

func (z *ZenMode) GetDomains() []string {
	z.stateMu.RLock()
	defer z.stateMu.RUnlock()

	d := make([]string, 0, len(z.Hosts))
	for k := range z.Hosts {
		d = append(d, k)
	}
	sort.Strings(d)
	return d

}

func (z *ZenMode) Run(mess *Message) (*Message, error) {
	z.stateMu.RLock()
	enabled := z.enabled
	hosts := make(map[string]string, len(z.Hosts))
	labels := append([]string(nil), z.labels...)
	excludeRules := z.excludeRules
	for k, v := range z.Hosts {
		hosts[k] = v
	}
	z.stateMu.RUnlock()

	if enabled {
		if !matchesClientScope(mess, labels) {
			return mess, nil
		}
		m, err := mess.GetMsg()
		if err != nil {
			return mess, err
		}
		domain := m.Question[0].Name
		if matchesSuffix(normalizeDomainPattern(domain), excludeRules.Domains) ||
			matchesAnyLabel(mess.Labels(), excludeRules.Labels) {
			return mess, nil
		}
		//Ideally regexes should be precompiled, but with caching enabled this is negligible.
		for k, v := range hosts {
			match, _ := regexp.MatchString(k+"\\.", domain)
			if match {
				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", domain, v))
				if err != nil {
					return mess, err
				}
				m.Answer = append(m.Answer, rr)
				mess.SetMsg(m)
				mess.Resolved(true)
				err = mess.AddValue("tdns/zen-mode", "true")
				if err != nil {
					return mess, err
				}
				return mess, nil
			}
		}
	}
	return mess, nil
}

func (z *ZenMode) Config(c config.Config) error {
	z.c = c
	return nil
}

func (z *ZenMode) Init() error {
	z.timerMu = make(chan struct{}, 1)
	logger := log.GetLogger("middleware.ZenMode", "init")
	z.stateMu.Lock()
	z.enabled = false
	z.initDone = true
	z.Hosts = map[string]string{}
	z.configuredHosts = map[string]string{}
	z.persistedHosts = map[string]string{}
	z.runtimeHosts = map[string]string{}
	z.labels = normalizeLabels(z.c.ZenMode.Labels)
	z.configuredExcludes = normalizeSelectorValues(z.c.ZenMode.Excludes)
	z.persistedExcludes = normalizeSelectorValues(z.c.ZenMode.PersistedExcludes)
	z.excludeRules = parseSelectors(append(append([]string(nil), z.configuredExcludes...), z.persistedExcludes...))
	z.startedAt = time.Time{}
	z.endsAt = time.Time{}
	if z.c.ZenMode.File != "" {
		h, err := ReadHosts(z.c.ZenMode.File)
		if err != nil {
			z.stateMu.Unlock()
			logger.Error(err)
			return err
		}
		z.configuredHosts = cloneHostMap(h)
	}
	for _, each := range z.c.ZenMode.Domains {
		z.configuredHosts[each] = ZenModeIP
	}
	for _, each := range z.c.ZenMode.PersistedDomains {
		z.persistedHosts[each] = ZenModeIP
	}
	z.refreshActiveHostsLocked()
	z.stateMu.Unlock()
	return nil
}

func (z *ZenMode) ReplaceDomains(hosts map[string]string) error {
	if len(hosts) == 0 {
		return errors.New("can't replace with an empty map")
	}
	z.stateMu.Lock()
	defer z.stateMu.Unlock()
	z.runtimeHosts = cloneHostMap(hosts)
	z.Hosts = cloneHostMap(hosts)
	return nil
}

func (z *ZenMode) ReplacePersistedDomains(domains []string) {
	z.stateMu.Lock()
	defer z.stateMu.Unlock()
	z.persistedHosts = map[string]string{}
	for _, each := range domains {
		z.persistedHosts[each] = ZenModeIP
	}
	if len(z.runtimeHosts) == 0 {
		z.refreshActiveHostsLocked()
	}
}

func (z *ZenMode) ReplacePersistedExcludes(values []string) {
	z.stateMu.Lock()
	defer z.stateMu.Unlock()
	z.persistedExcludes = append([]string(nil), values...)
	z.excludeRules = parseSelectors(append(append([]string(nil), z.configuredExcludes...), z.persistedExcludes...))
}

func (z *ZenMode) refreshActiveHostsLocked() {
	z.Hosts = mergeHostMaps(z.configuredHosts, z.persistedHosts)
}

func (z *ZenMode) Start() {
	logger := log.GetLogger("middleware.ZenMode", "Timer")
	z.stateMu.RLock()
	enabled := z.enabled
	durationMinutes := z.c.ZenMode.Time
	z.stateMu.RUnlock()
	if enabled {
		logger := log.GetLogger("middleware.ZenMode", "Start")
		logger.Info("Zen mode timer in progress")
		return
	}
	select {
	case z.timerMu <- struct{}{}:
		// lock acquired
		startedAt := time.Now()
		endsAt := startedAt.Add(time.Duration(durationMinutes) * time.Minute)
		z.stateMu.Lock()
		z.enabled = true
		z.startedAt = startedAt
		z.endsAt = endsAt
		z.stateMu.Unlock()
		logger.WithFields(logrus.Fields{"action": "start"}).Infof("Starting Zen period (%d) minutes", durationMinutes)
		go func() {
			<-time.After(time.Duration(durationMinutes) * time.Minute)
			z.stateMu.Lock()
			z.enabled = false
			z.startedAt = time.Time{}
			z.endsAt = time.Time{}
			z.stateMu.Unlock()
			<-z.timerMu
			logger.WithFields(logrus.Fields{"action": "stop"}).Info("Zen period ended")
		}()
	default:
		logger.Info("Zen period could not acquire a lock, timer already started")
	}
}

func (z *ZenMode) Status() bool {
	z.stateMu.RLock()
	defer z.stateMu.RUnlock()
	return z.enabled
}

func (z *ZenMode) RemainingSeconds() int64 {
	z.stateMu.RLock()
	defer z.stateMu.RUnlock()
	if !z.enabled || z.endsAt.IsZero() {
		return 0
	}
	remaining := time.Until(z.endsAt)
	if remaining < 0 {
		return 0
	}
	return int64(remaining.Seconds())
}

func (z *ZenMode) configuredDomains() ([]string, error) {
	configured := map[string]struct{}{}
	if z.c.ZenMode.File != "" {
		hosts, err := ReadHosts(z.c.ZenMode.File)
		if err != nil {
			return nil, err
		}
		for each := range hosts {
			configured[each] = struct{}{}
		}
	}
	for _, each := range z.c.ZenMode.Domains {
		configured[each] = struct{}{}
	}

	domains := make([]string, 0, len(configured))
	for each := range configured {
		domains = append(domains, each)
	}
	sort.Strings(domains)
	return domains, nil
}

func (z *ZenMode) StatusView() (ZenModeStatus, error) {
	configuredDomains, err := z.configuredDomains()
	if err != nil {
		return ZenModeStatus{}, err
	}

	z.stateMu.RLock()
	defer z.stateMu.RUnlock()

	persistedDomains := make([]string, 0, len(z.persistedHosts))
	for each := range z.persistedHosts {
		persistedDomains = append(persistedDomains, each)
	}
	sort.Strings(persistedDomains)

	runtimeDomains := make([]string, 0, len(z.runtimeHosts))
	for each := range z.runtimeHosts {
		runtimeDomains = append(runtimeDomains, each)
	}
	sort.Strings(runtimeDomains)

	status := ZenModeStatus{
		Enabled:            z.enabled,
		File:               z.c.ZenMode.File,
		DurationMinutes:    z.c.ZenMode.Time,
		ConfiguredDomains:  configuredDomains,
		PersistedDomains:   persistedDomains,
		ConfiguredExcludes: append([]string(nil), z.configuredExcludes...),
		PersistedExcludes:  append([]string(nil), z.persistedExcludes...),
		Labels:             append([]string(nil), z.labels...),
		RuntimeDomains:     runtimeDomains,
		RemainingSeconds:   0,
	}
	if z.enabled && !z.startedAt.IsZero() {
		status.StartedAt = z.startedAt.Format(time.RFC3339)
	}
	if z.enabled && !z.endsAt.IsZero() {
		status.EndsAt = z.endsAt.Format(time.RFC3339)
		remaining := time.Until(z.endsAt)
		if remaining > 0 {
			status.RemainingSeconds = int64(remaining.Seconds())
		}
	}
	return status, nil
}
