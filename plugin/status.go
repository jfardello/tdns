package plugin

import (
	"fmt"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
)

const StatusDefaultDomain string = "_status.tdns.local."

type StatusPlugin struct {
	Enabled bool
	Since   time.Time
}

func (sr *StatusPlugin) Config(c config.Config) error {
	sr.Enabled = c.Status
	if c.Status {
		sr.Since = time.Now()
		logger := log.GetLogger("Statuslugin", "config")
		logger.Info("Status plugin starting.")
		return nil
	}
	return nil
}

func (sc *StatusPlugin) Init() error {

	return nil
}

func (sc *StatusPlugin) Run(m *dns.Msg) (*dns.RR, bool, error) {
	if !sc.Enabled {
		return nil, false, nil
	}

	domain := m.Question[0].Name
	//Idealy regexes should be precompiled, but with caching enables this is negligible.

	logger := log.GetLogger("StatusPlugin", "resolve")
	logger.Debugf("Domain: %s query domain: %s", domain, StatusDefaultDomain)
	if domain == StatusDefaultDomain {
		rr, err := dns.NewRR(fmt.Sprintf(`%s 3600 IN TXT "%s"`, domain, getStatus(sc.Since)))
		if err != nil {
			return nil, false, err
		}
		return &rr, false, nil
	}

	return nil, false, nil
}

func (sc *StatusPlugin) Info() (string, Ptype) {
	return "status", PreRouting
}

func getStatus(t time.Time) string {
	c := GetCache()
	st := c.Status()
	return fmt.Sprintf("Status: online since %s Cache Hits: %d Cache Misses: %d", t, st.Hits, st.Misses)
}
