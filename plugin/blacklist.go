package plugin

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"strings"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"

	"github.com/armon/go-radix"
	"github.com/miekg/dns"
)

const (
	BlacklistAddr string = "0.0.0.0"
)

type None struct{}

type BlackListPlugin struct {
	Enabled     bool
	Hole        *radix.Tree
	HoleFile    string
	WhiteList   []string
	DefaultAddr string
}

// Info Ptype indicates where to hook the plugin.
func (bp *BlackListPlugin) Info() (string, Ptype) {
	return "blacklist", PreRouting
}

func (bp *BlackListPlugin) Run(mess *Message) (message *Message, err error) {
	logger := log.GetLogger("plugin", "BHole")
	if !bp.Enabled {
		logger.Debug("Blackhole disabled.")
		return mess, nil
	}
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}
	domain := strings.TrimSuffix(m.Question[0].Name, ".")

	_, ok := bp.Hole.Get(domain)
	if ok {
		logger.Debugf("%s found in the list", domain)
		rr, err := dns.NewRR(fmt.Sprintf("%s A 0.0.0.0", domain))
		if err != nil {
			return mess, err
		}
		m.Answer = append(m.Answer, rr)
		mess.SetMsg(m)
		mess.Resolved(true)
		return mess, nil
	}
	logger.Debugf("%s not found in the list", domain)
	return mess, nil

}

func (bp *BlackListPlugin) Config(c config.Config) error {
	bp.Enabled = c.BlackHole.Enabled
	bf := c.BlackHole.File
	if bf != "" {
		bp.HoleFile = bf
		bp.WhiteList = c.BlackHole.Excludes
		return nil
	}
	bp.DefaultAddr = BlacklistAddr
	return errors.New("BlackholeFile is mandatory")
}

func (bp *BlackListPlugin) Init() error {

	readFile, err := os.Open(bp.HoleFile)
	if err != nil {
		return err
	}
	defer func(readFile *os.File) {
		err := readFile.Close()
		if err != nil {
			logger := log.GetLogger("blacklist", "Init/deferred")
			logger.Errorf("Error closing file %s: %s", bp.HoleFile, err.Error())
		}
	}(readFile)
	scanner := bufio.NewScanner(readFile)
OUTER:
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "#") {
			continue
		}

		fields := strings.Fields(s)
		if len(fields) > 1 {
			for _, each := range bp.WhiteList {
				if strings.HasSuffix(fields[1], each) {
					continue OUTER
				}
			}
			bp.Hole.Insert(fields[1], None{})
		}
	}
	return nil

}
