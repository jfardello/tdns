package plugin

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/jfardello/tdns/client"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
)

// StubresolverPlugin sends DNS requests to stub zone dns, eg. VPN or DMZ internal dns.
type StubresolverPlugin struct {
	mu          sync.Mutex
	EnableStubs bool
	Stubs       map[string]*client.Mux
	config      config.Config
}

func (sr *StubresolverPlugin) Info() (string, Ptype) {
	return "stubresolver", Resolving
}

func (sr *StubresolverPlugin) Run(ctx context.Context, m *dns.Msg) (*dns.RR, bool, error) {
	logger := log.GetLogger("plugin", "StubResolver")
	domain := m.Question[0].Name

	//there should be a channel for toogling enableStubs
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
					logger.Error(err)
					return nil, false, nil
				}
				return &response.Answer[0], false, nil
			}
		}

	}
	return nil, false, nil
}

func (sr *StubresolverPlugin) Config(c config.Config) error {
	sr.config = c
	return nil
}

func (sr *StubresolverPlugin) Replace(m map[string]*client.Mux) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	for each := range m {
		sr.Stubs[each] = m[each]
	}
}

func (sr *StubresolverPlugin) Init() error {
	stubs, err := ParseStubList(sr.config.StubResolver.Stubs, sr.config.Timeout, sr.config.UpstreamTimeout)
	if err != nil {
		return err
	}
	sr.Replace(stubs)
	return nil
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
