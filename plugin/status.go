package plugin

import (
	"context"
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

func (sc *StatusPlugin) Config(c config.Config) error {
	sc.Enabled = c.Status.Enabled
	if c.Status.Enabled {
		sc.Since = time.Now()
		logger := log.GetLogger("Statuslugin", "config")
		logger.Info("Status plugin starting.")
		return nil
	}
	return nil
}

func (sc *StatusPlugin) Init() error {

	return nil
}

func (sc *StatusPlugin) Run(ctx context.Context, m *dns.Msg) (*dns.RR, bool, error) {
	if !sc.Enabled {
		return nil, false, nil
	}
	domain := m.Question[0].Name
	logger := log.GetLogger("StatusPlugin", "resolve")
	//TODO: configure status options.
	if domain == StatusDefaultDomain && m.Question[0].Qtype == dns.TypeTXT {
		logger.Debugf("Domain: %s query domain: %s", domain, StatusDefaultDomain)
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
