package middleware

import (
	"fmt"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
)

const StatusDefaultDomain string = "_status.tdns.local."

type Status struct {
	Enabled      bool
	Since        time.Time
	ExposeStats  bool
	ExposeUptime bool
}

func (sc *Status) Config(c config.Config) error {
	sc.Enabled = c.Status.Enabled
	sc.ExposeStats = c.Status.ExposeStats
	sc.ExposeUptime = c.Status.ExposeUptime
	if c.Status.Enabled {
		sc.Since = time.Now()
		logger := log.GetLogger("middleware.Status", "config")
		logger.Info("Status middleware starting.")
		return nil
	}
	return nil
}

func (sc *Status) Init() error {

	return nil
}

func (sc *Status) Run(mess *Message) (*Message, error) {
	if !sc.Enabled {
		return mess, nil
	}
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}
	domain := m.Question[0].Name
	logger := log.GetLogger("middleware.Status", "resolve")
	if domain == StatusDefaultDomain && m.Question[0].Qtype == dns.TypeTXT {
		logger.Debugf("Domain: %s query domain: %s", domain, StatusDefaultDomain)
		rr, err := dns.NewRR(fmt.Sprintf(`%s 3600 IN TXT "%s"`, domain, getStatus(sc.Since, sc)))
		if err != nil {
			return mess, err
		}
		m.Answer = append(m.Answer, rr)
		mess.SetMsg(m)
		mess.Resolved(true)
		return mess, nil
	}

	return mess, nil
}

func (sc *Status) Info() (string, Stage) {
	return "status", PreRouting
}

func getStatus(t time.Time, sp *Status) string {
	s := "Status: online"
	if sp.ExposeUptime {
		s = fmt.Sprintf("%s since %s", s, t)
	}
	if sp.ExposeStats {
		c := GetCache()
		st := c.Status()
		s = fmt.Sprintf("%s Cache Hits: %d Cache Misses: %d", s, st.Hits, st.Misses)
	}
	return s
}
