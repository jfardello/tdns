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

type ZenmodePlugin struct {
	ZenFile  string
	enabled  bool
	initDone bool
	mu       chan struct{}
	c        config.Config
	Hosts    map[string]string
}

// Ptype indicates where to hook the plugin.
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
func (z *ZenmodePlugin) Run(m *dns.Msg) (rr *dns.RR, cacheSafe bool, err error) {
	if z.enabled {
		domain := m.Question[0].Name
		//Idealy regexes should be precompiled, but with caching enables this is negligible.
		for k, v := range z.Hosts {
			match, _ := regexp.MatchString(k+"\\.", domain)
			if match {
				rr, err := dns.NewRR(fmt.Sprintf("%s A %s", domain, v))
				if err != nil {
					return nil, false, err
				}
				return &rr, true, nil
			}
		}
	}
	return nil, false, nil
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
	h, err := ReadHosts(z.c.ZenModeFile)
	if err != nil {
		logger.Error(err)
		return err
	}
	z.Hosts = h
	return nil
}

func (z *ZenmodePlugin) ReplaceDomains(hosts map[string]string) error {
	if len(hosts) == 0 {
		return errors.New("Can't replace with an empty map")
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
		logger.WithFields(logrus.Fields{"action": "start"}).Infof("Starting Zen period (%d) minutes", z.c.ZenModeTime)
		go func() {
			<-time.After(time.Duration(z.c.ZenModeTime) * time.Minute)
			z.enabled = false
			<-z.mu // unlock
			logger.WithFields(logrus.Fields{"action": "stop"}).Info("Zen period ended")
		}()
	default:
		logger.Info("Zen period could not aquire a lock, timer already started")
	}
}

func (z *ZenmodePlugin) Status() bool {
	return z.enabled
}
