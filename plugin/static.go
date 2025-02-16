package plugin

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
)

type StaticResponsePlugin struct {
	hostsFile string
	Hosts     map[string]string
	Enabled   bool
}

func (sr *StaticResponsePlugin) Config(c config.Config) error {
	sr.Enabled = c.StaticResponse
	if c.StaticReposnsefile != "" {
		logger := log.GetLogger("StaticResponsePlugin", "config")
		logger.Infof("Using file %s", c.StaticReposnsefile)
		sr.hostsFile = c.StaticReposnsefile
		return nil
	}
	return errors.New("StaticResponseFile is mandatory")
}

func (sc *StaticResponsePlugin) Init() error {
	logger := log.GetLogger("StaticResponsePlugin", "init")
	h, err := ReadHosts(sc.hostsFile)
	if err != nil {
		logger.Error(err)
		return err
	}
	sc.Hosts = h
	return nil
}

func (sc *StaticResponsePlugin) Run(m *dns.Msg) (*dns.RR, bool, error) {
	if !sc.Enabled {
		return nil, false, nil
	}

	domain := m.Question[0].Name
	//Idealy regexes should be precompiled, but with caching enables this is negligible.
	for k, v := range sc.Hosts {
		match, _ := regexp.MatchString(k+"\\.", domain)
		if match {
			rr, err := dns.NewRR(fmt.Sprintf("%s A %s", domain, v))
			if err != nil {
				return nil, false, err
			}
			return &rr, true, nil
		}
	}
	return nil, false, nil
}

func (sc *StaticResponsePlugin) Info() (string, Ptype) {
	return "staticresponse", PreRouting
}

func ReadHosts(filePath string) (map[string]string, error) {
	readFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer readFile.Close()
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
