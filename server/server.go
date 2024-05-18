package server

import (
	"github.com/jfardello/tdns/client"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/plugin"
	"github.com/miekg/dns"
)

type Server struct {
	Plugins         map[string]plugin.Plugin
	pluginIndex     *PluginIndex
	Config          config.Config
	defaultUpstream client.ClientMux
}

type PluginIndex struct {
	preRouting  []string
	resolving   []string
	postRouting []string
}

func (s *Server) Handler(requestMsg *dns.Msg) (*dns.Msg, error) {
	responseMsg := new(dns.Msg)
	if len(requestMsg.Question) > 0 {
		return s.process(requestMsg)
	}
	return responseMsg, nil
}
func (s *Server) getIndexes() *PluginIndex {
	if s.pluginIndex == nil {
		pi := &PluginIndex{}
		//Split plugins by type
		for _, p := range s.Plugins {
			name, ptype := p.Info()
			switch ptype {
			case plugin.PreRouting:
				pi.preRouting = append(pi.preRouting, name)
			case plugin.Resolving:
				pi.resolving = append(pi.resolving, name)
			case plugin.PostRouting:
				pi.postRouting = append(pi.postRouting, name)
			}
		}
		s.pluginIndex = pi
	}
	return s.pluginIndex

}

func answerMsg(r *dns.RR) *dns.Msg {
	responseMsg := new(dns.Msg)
	responseMsg.Answer = append(responseMsg.Answer, *r)
	return responseMsg

}

func (s *Server) process(requestMsg *dns.Msg) (*dns.Msg, error) {
	pi := s.getIndexes()

	//Handle Prerouting
	var currentResponse *dns.Msg
	for _, p := range pi.preRouting {
		rr, _, err := s.Plugins[p].Run(requestMsg)
		if err != nil {
			continue
		}
		if rr != nil {
			currentResponse = answerMsg(rr)
			break
		}
	}
	//If we didn't resolve at preRouting time, then try resolving plugins.
	if currentResponse == nil {
		for _, p := range pi.resolving {
			rr, _, err := s.Plugins[p].Run(requestMsg)
			if err != nil {
				return nil, err
			}
			if rr != nil {
				currentResponse = answerMsg(rr)
				break
			}
		}
	}

	//no response from resolving plugins, just resolve with the default upstream.

	var err error
	if currentResponse == nil {
		currentResponse, err = s.resolve(requestMsg)
		if err != nil {
			logger := log.GetLogger("Server", "process")
			logger.Error(err)
		}
	}

	for _, p := range pi.postRouting {
		_, _, err := s.Plugins[p].Run(currentResponse)
		if err != nil {
			return nil, err
		}

	}

	return currentResponse, nil
}

func (s *Server) StubsToogle(state bool) bool {
	c := config.GetRunningConfig()
	config.Lock()
	c.StubResolver = state
	config.Unlock()
	s.Plugins["stubresolver"].Config(*c)
	return c.StubResolver
}

func (s *Server) BholeToogle(state bool) bool {
	c := config.GetRunningConfig()
	c.BlackHole = state
	config.SetRunningConfig(c)
	s.Plugins["blacklist"].Config(*c)
	return c.BlackHole
}

func (s *Server) ClearCache() error {
	c := plugin.GetCache()
	return c.Clear()
}

func (s *Server) resolve(m *dns.Msg) (*dns.Msg, error) {
	logger := log.GetLogger("Server", "resolve")
	logger.Infof("Asking uppstream %s for %s", s.defaultUpstream.Upstreams[0].Address, m.Question[0].Name)

	response, _, err := s.defaultUpstream.Resolve(m)

	if err != nil {
		logger.Error(err)
		return nil, err
	}
	return response, nil
}

func NewServer(options ...func(*Server)) *Server {

	s := &Server{

		Config:  *config.GetRunningConfig(),
		Plugins: make(map[string]plugin.Plugin),
	}
	for _, o := range options {
		o(s)
	}
	return s
}
