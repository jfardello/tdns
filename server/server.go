package server

import (
	"context"
	"github.com/jfardello/tdns/client"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/plugin"
	"github.com/miekg/dns"
	"net"
)

type Server struct {
	Plugins         map[string]plugin.Plugin
	pluginIndex     *PluginIndex
	Config          config.Config
	defaultUpstream client.Mux
}

type PluginIndex struct {
	preRouting  []string
	resolving   []string
	postRouting []string
}

func (s *Server) Handler(requestMsg *dns.Msg, remoteAddr net.Addr) (*dns.Msg, error) {
	v := config.CtxValue{RemoteAddr: remoteAddr}
	ctx := context.WithValue(context.Background(), config.CtxKey, v)

	responseMsg := new(dns.Msg)
	if len(requestMsg.Question) > 0 {
		return s.process(ctx, requestMsg)
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

func (s *Server) process(ctx context.Context, requestMsg *dns.Msg) (*dns.Msg, error) {
	pi := s.getIndexes()
	//Handle Prerouting
	var currentResponse *dns.Msg
	for _, p := range pi.preRouting {
		rr, _, err := s.Plugins[p].Run(ctx, requestMsg)
		if err != nil {
			continue
		}
		if rr != nil {
			currentResponse = answerMsg(rr)
			break
		}
	}
	//If we didn't resolve at preRouting time, then try resolving plugins.
	currentResponse, shouldReturn, returnValue, err1 := s.tryResolve(ctx, currentResponse, pi, requestMsg)
	if shouldReturn {
		return returnValue, err1
	}

	//no response from resolving plugins, just resolve with the default upstream.

	var err error
	if currentResponse == nil {
		currentResponse, err = s.resolve(requestMsg)
		if err != nil {
			logger := log.GetLogger("Server", "process")
			logger.Error(err)
			return nil, err
		}
	}

	for _, p := range pi.postRouting {
		_, _, err := s.Plugins[p].Run(ctx, currentResponse)
		if err != nil {
			logger := log.GetLogger("Server", "post-process")
			name, _ := s.Plugins[p].Info()
			logger.Debugf("Plugin [%s] returning err : %s", name, err.Error())
		}

	}

	return currentResponse, nil
}

// try resolving plugins.
func (s *Server) tryResolve(ctx context.Context, currentResponse *dns.Msg, pi *PluginIndex, requestMsg *dns.Msg) (*dns.Msg, bool, *dns.Msg, error) {
	if currentResponse == nil {
		for _, p := range pi.resolving {
			rr, _, err := s.Plugins[p].Run(ctx, requestMsg)
			if err != nil {
				return nil, true, nil, err
			}
			if rr != nil {
				currentResponse = answerMsg(rr)
				break
			}
		}
	}
	return currentResponse, false, nil, nil
}

func (s *Server) StubsToggle(state bool) bool {
	logger := log.GetLogger("server", "StubsToggle")
	c := config.GetRunningConfig()
	config.Lock()
	c.StubResolver.Enabled = state
	config.Unlock()
	err := s.Plugins["stubresolver"].Config(*c)
	if err != nil {
		logger.Fatal(err)
	}
	return c.StubResolver.Enabled
}

func (s *Server) BholeToggle(state bool) bool {
	logger := log.GetLogger("server", "BholeToggle")
	c := config.GetRunningConfig()
	c.BlackHole.Enabled = state
	config.SetRunningConfig(c)
	err := s.Plugins["blacklist"].Config(*c)
	if err != nil {
		logger.Fatal(err)
	}

	return c.BlackHole.Enabled
}

func (s *Server) ClearCache() error {
	c := plugin.GetCache()
	return c.Clear()
}

func (s *Server) resolve(m *dns.Msg) (*dns.Msg, error) {
	logger := log.GetLogger("Server", "resolve")
	logger.Debugf("Asking uppstream %s for %s", s.defaultUpstream.Upstreams[0].Address, m.Question[0].Name)

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
