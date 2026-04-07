package middleware

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
)

var DefaultStaticFile = "/etc/hosts"

type StaticResponse struct {
	hostsFile       string
	Hosts           map[string]string
	configuredHosts map[string]string
	persistedHosts  map[string]string
	runtimeHosts    map[string]string
	Enabled         bool
	labels          []string
	mu              sync.RWMutex
}

type HostEntry struct {
	Domain  string `json:"domain"`
	Address string `json:"address"`
}

type StaticResponseStatus struct {
	Enabled         bool        `json:"enabled"`
	File            string      `json:"file,omitempty"`
	Labels          []string    `json:"labels,omitempty"`
	ConfiguredHosts []HostEntry `json:"configured_hosts,omitempty"`
	PersistedHosts  []HostEntry `json:"persisted_hosts,omitempty"`
	RuntimeHosts    []HostEntry `json:"runtime_hosts,omitempty"`
}

func (sr *StaticResponse) Config(c config.Config) error {
	sr.Enabled = c.StaticResponse.Enabled
	sr.labels = normalizeLabels(c.StaticResponse.Labels)
	if c.StaticResponse.File != "" {
		logger := log.GetLogger("middleware.StaticResponse", "config")
		logger.Infof("Using file %s", c.StaticResponse.File)
		sr.hostsFile = c.StaticResponse.File
		return nil
	}
	return errors.New("StaticResponseFile is mandatory")
}

func (sr *StaticResponse) Init() error {
	logger := log.GetLogger("middleware.StaticResponse", "init")
	h, err := ReadHosts(sr.hostsFile)
	if err != nil {
		logger.Error(err)
		return err
	}
	sr.mu.Lock()
	sr.configuredHosts = cloneHostMap(h)
	sr.persistedHosts = cloneHostMap(sr.activePersistedHosts())
	sr.runtimeHosts = map[string]string{}
	sr.refreshActiveHostsLocked()
	sr.mu.Unlock()
	return nil
}

func (sr *StaticResponse) Run(mess *Message) (*Message, error) {
	sr.mu.RLock()
	enabled := sr.Enabled
	labels := append([]string(nil), sr.labels...)
	hosts := make(map[string]string, len(sr.Hosts))
	for k, v := range sr.Hosts {
		hosts[k] = v
	}
	sr.mu.RUnlock()

	if !enabled {
		return mess, nil
	}
	if !matchesClientScope(mess, labels) {
		return mess, nil
	}
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}

	domain := m.Question[0].Name
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
			return mess, nil
		}
	}
	return mess, nil
}

func (sr *StaticResponse) Info() (string, Stage) {
	return "static-response", PreRouting
}

func (sr *StaticResponse) SetEnabled(state bool) {
	sr.mu.Lock()
	sr.Enabled = state
	sr.mu.Unlock()
}

func (sr *StaticResponse) IsEnabled() bool {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.Enabled
}

func (sr *StaticResponse) ReplaceRuntimeHosts(hosts map[string]string) error {
	if len(hosts) == 0 {
		return errors.New("can't replace with an empty hosts map")
	}
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.runtimeHosts = cloneHostMap(hosts)
	sr.Hosts = cloneHostMap(hosts)
	return nil
}

func (sr *StaticResponse) ReplacePersistedHosts(hosts map[string]string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.persistedHosts = cloneHostMap(hosts)
	if len(sr.runtimeHosts) == 0 {
		sr.refreshActiveHostsLocked()
	}
}

func (sr *StaticResponse) activePersistedHosts() map[string]string {
	conf := config.GetRunningConfig()
	return conf.StaticResponse.ExtraHosts
}

func (sr *StaticResponse) refreshActiveHostsLocked() {
	sr.Hosts = mergeHostMaps(sr.configuredHosts, sr.persistedHosts)
}

func cloneHostMap(hosts map[string]string) map[string]string {
	if len(hosts) == 0 {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(hosts))
	for k, v := range hosts {
		cloned[k] = v
	}
	return cloned
}

func mergeHostMaps(maps ...map[string]string) map[string]string {
	merged := map[string]string{}
	for _, hosts := range maps {
		for domain, address := range hosts {
			merged[domain] = address
		}
	}
	return merged
}

func (sr *StaticResponse) Status() (StaticResponseStatus, error) {
	sr.mu.RLock()
	enabled := sr.Enabled
	labels := append([]string(nil), sr.labels...)
	configuredHosts := cloneHostMap(sr.configuredHosts)
	persistedHosts := cloneHostMap(sr.persistedHosts)
	runtimeHosts := cloneHostMap(sr.runtimeHosts)
	sr.mu.RUnlock()

	return StaticResponseStatus{
		Enabled:         enabled,
		File:            sr.hostsFile,
		Labels:          append([]string(nil), labels...),
		ConfiguredHosts: sortedHostEntries(configuredHosts),
		PersistedHosts:  sortedHostEntries(persistedHosts),
		RuntimeHosts:    sortedHostEntries(runtimeHosts),
	}, nil
}

func ReadHosts(filePath string) (map[string]string, error) {
	readFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func(readFile *os.File) {
		err := readFile.Close()
		if err != nil {

		}
	}(readFile)
	scanner := bufio.NewScanner(readFile)
	var hosts map[string]string = map[string]string{}
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "#") {
			continue
		}

		fields := strings.Fields(s)
		if len(fields) > 1 {
			hosts[fields[1]] = fields[0]
		}
	}
	return hosts, nil
}

func ReadHostsLines(lines []string) (map[string]string, error) {
	hosts := map[string]string{}
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		fields := strings.Fields(s)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid host entry: %q", line)
		}
		hosts[fields[1]] = fields[0]
	}
	return hosts, nil
}

func sortedHostEntries(hosts map[string]string) []HostEntry {
	entries := make([]HostEntry, 0, len(hosts))
	for domain, address := range hosts {
		entries = append(entries, HostEntry{Domain: domain, Address: address})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Domain < entries[j].Domain
	})
	return entries
}

func HostsToEntries(hosts map[string]string) []HostEntry {
	return sortedHostEntries(hosts)
}
