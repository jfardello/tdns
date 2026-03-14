package middleware

import (
	"fmt"
	"regexp"
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
	config      config.Config
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
