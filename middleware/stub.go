package middleware

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jfardello/tdns/client"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
)

// StubResolver sends DNS requests to domain-specific upstreams.
type StubResolver struct {
	mu          sync.Mutex
	EnableStubs bool
	Stubs       map[string]*client.Mux
	runtime     []string
	config      config.Config
}

type StubResolverStatus struct {
	Enabled         bool     `json:"enabled"`
	ConfiguredStubs []string `json:"configured_stubs,omitempty"`
	RuntimeStubs    []string `json:"runtime_stubs,omitempty"`
}

func (sr *StubResolver) Info() (string, Stage) {
	return "stub-resolver", Resolving
}

func (sr *StubResolver) Run(mess *Message) (*Message, error) {
	logger := log.GetLogger("middleware.StubResolver", "Run")
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}
	domain := m.Question[0].Name

	if sr.EnableStubs {
		logger.Debug("Stubs are enabled")
		sr.mu.Lock()
		defer sr.mu.Unlock()
		for k, mux := range sr.Stubs {
			match, _ := regexp.MatchString(`^.*\Q`+k+`\E\.`, domain)
			if match {
				logger.Debugf("Resove %s via server %s", domain, mux.Upstreams[0].Address)
				response, _, err := mux.Resolve(m)
				if err != nil {
					return mess, err
				}
				//TODO: check response
				if response.Rcode != dns.RcodeSuccess || len(response.Answer) == 0 {
					return mess, fmt.Errorf("stub resolver returned no answers, rcode: %s", dns.RcodeToString[response.Rcode])
				}
				mess.SetMsg(response)
				err = mess.AddValue("tdns/stub", "true")
				if err != nil {
					return mess, err
				}
				mess.Resolved(true)
				return mess, nil
			}
		}

	}
	return mess, nil
}

func (sr *StubResolver) Config(c config.Config) error {
	sr.config = c
	return nil
}

func (sr *StubResolver) Replace(m map[string]*client.Mux) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.Stubs = make(map[string]*client.Mux, len(m))
	for each := range m {
		sr.Stubs[each] = m[each]
	}
}

func (sr *StubResolver) Init() error {
	stubs, err := ParseStubList(sr.config.StubResolver.Stubs, sr.config.Timeout, sr.config.UpstreamTimeout)
	if err != nil {
		return err
	}
	sr.Replace(stubs)
	sr.mu.Lock()
	sr.runtime = append([]string(nil), sr.config.StubResolver.Stubs...)
	sr.mu.Unlock()
	return nil
}

func (sr *StubResolver) SetEnabled(state bool) {
	sr.mu.Lock()
	sr.EnableStubs = state
	sr.mu.Unlock()
}

func (sr *StubResolver) IsEnabled() bool {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.EnableStubs
}

func (sr *StubResolver) ReplaceRuntimeEntries(entries []string, globalTimeout int, upstreamTimeout int) error {
	stubs, err := ParseStubList(entries, globalTimeout, upstreamTimeout)
	if err != nil {
		return err
	}

	sr.mu.Lock()
	sr.Stubs = make(map[string]*client.Mux, len(stubs))
	for each := range stubs {
		sr.Stubs[each] = stubs[each]
	}
	sr.runtime = append([]string(nil), entries...)
	sr.mu.Unlock()
	return nil
}

func (sr *StubResolver) Status() StubResolverStatus {
	sr.mu.Lock()
	defer sr.mu.Unlock()

	configured := append([]string(nil), sr.config.StubResolver.Stubs...)
	runtime := append([]string(nil), sr.runtime...)
	sort.Strings(configured)
	sort.Strings(runtime)

	return StubResolverStatus{
		Enabled:         sr.EnableStubs,
		ConfiguredStubs: configured,
		RuntimeStubs:    runtime,
	}
}

func ParseStubList(s []string, globalTimeOut int, upstreamTimeOut int) (map[string]*client.Mux, error) {
	stubs := map[string]*client.Mux{}
	for _, each := range s {
		splitted := strings.Split(each, ",")
		servers := splitted[1:]
		mux := client.NewClientMux(servers,
			client.WithGlobalTimeout(time.Duration(globalTimeOut)*time.Millisecond),
			client.WithMuxUpstreamOptions(client.WithTimeout(time.Duration(upstreamTimeOut)*time.Millisecond)))
		stubs[splitted[0]] = mux
	}
	return stubs, nil

}
