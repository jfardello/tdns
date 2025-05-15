package plugin

import (
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
	"github.com/sirupsen/logrus"
)

const ZENMODE_IP string = "127.0.0.1"

var (
	DefaultZenTimeMinutes int = 20
	DefaultZenDomains         = []string{"wwww.instagram.com", "x.com", "www.facebook.com"}
)

type ZenmodePlugin struct {
	ZenFile  string
	enabled  bool
	initDone bool
	mu       chan struct{}
	c        config.Config
	Hosts    map[string]string
}

// Info ... Ptype indicates where to hook the plugin.
func (z *ZenmodePlugin) Info() (string, Ptype) {
	return "zenmode", PreRouting
}

func (z *ZenmodePlugin) GetDomains() []string {
	d := make([]string, 0, len(z.Hosts))
	for k := range z.Hosts {
		d = append(d, k)
	}
	return d

}

// Run performs the plugin logic, returns a resource record, a cache flag, and an error indicating failiure.
func (z *ZenmodePlugin) Run(mess *Message) (*Message, error) {
	if z.enabled {
		m, err := mess.GetMsg()
		if err != nil {
			return mess, err
		}
		domain := m.Question[0].Name
		//Ideally regexes should be precompiled, but with caching enabled this is negligible.
		for k, v := range z.Hosts {
			match, _ := regexp.MatchString(k+"\\.", domain)
			if match {
				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", domain, v))
				if err != nil {
					return mess, err
				}
				m.Answer = append(m.Answer, rr)
				mess.SetMsg(m)
				mess.Resolved(true)
				err = mess.AddValue("tdsn/zenmode", "true")
				if err != nil {
					return mess, err
				}
				return mess, nil
			}
		}
	}
	return mess, nil
}

func (z *ZenmodePlugin) Config(c config.Config) error {
	z.c = c
	return nil
}

func (z *ZenmodePlugin) Init() error {
	z.mu = make(chan struct{}, 1) //this is the lock
	logger := log.GetLogger("ZenmodePlugin", "init")
	z.enabled = false
	z.initDone = true
	z.Hosts = map[string]string{}
	if z.c.ZenMode.File != "" {
		h, err := ReadHosts(z.c.ZenMode.File)
		if err != nil {
			logger.Error(err)
			return err
		}
		z.Hosts = h
	}
	for _, each := range z.c.ZenMode.Domains {
		z.Hosts[each] = "0.0.0.0"
	}
	return nil
}

func (z *ZenmodePlugin) ReplaceDomains(hosts map[string]string) error {
	if len(hosts) == 0 {
		return errors.New("can't replace with an empty map")
	}
	z.Hosts = hosts
	return nil
}

func (z *ZenmodePlugin) Start() {
	logger := log.GetLogger("ZenmodePlugin", "Timer")
	if z.enabled {
		logger := log.GetLogger("ZenmodePlugin", "Start")
		logger.Info("Zenmode timer in progress")
		return
	}
	select {
	case z.mu <- struct{}{}:
		// lock acquired
		z.enabled = true
		zt := z.c.ZenMode.Time
		logger.WithFields(logrus.Fields{"action": "start"}).Infof("Starting Zen period (%d) minutes", zt)
		go func() {
			<-time.After(time.Duration(zt) * time.Minute)
			z.enabled = false
			<-z.mu // unlock
			logger.WithFields(logrus.Fields{"action": "stop"}).Info("Zen period ended")
		}()
	default:
		logger.Info("Zen period could not acquire a lock, timer already started")
	}
}

func (z *ZenmodePlugin) Status() bool {
	return z.enabled
}
