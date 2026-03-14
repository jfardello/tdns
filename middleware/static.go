package middleware

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

var DefaultStaticFile = "/etc/hosts"

type StaticResponse struct {
	hostsFile string
	Hosts     map[string]string
	Enabled   bool
}

func (sr *StaticResponse) Config(c config.Config) error {
	sr.Enabled = c.StaticResponse.Enabled
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
	sr.Hosts = h
	return nil
}

func (sr *StaticResponse) Run(mess *Message) (*Message, error) {
	if !sr.Enabled {
		return mess, nil
	}
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}

	domain := m.Question[0].Name
	//Ideally regexes should be precompiled, but with caching enables this is negligible.
	for k, v := range sr.Hosts {
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
