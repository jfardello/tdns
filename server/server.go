package server

import (
	"context"
	"errors"
	"github.com/jfardello/tdns/client"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/middleware"
	"github.com/miekg/dns"
	"net"
)

type Server struct {
	Middlewares     map[string]middleware.Middleware
	middlewareIndex *MiddlewareIndex
	Config          config.Config
	defaultUpstream client.Mux
}

type MiddlewareIndex struct {
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

func (s *Server) getIndexes() *MiddlewareIndex {
	if s.middlewareIndex == nil {
		pi := &MiddlewareIndex{}
		for _, p := range s.Middlewares {
			name, ptype := p.Info()
			switch ptype {
			case middleware.PreRouting:
				pi.preRouting = append(pi.preRouting, name)
			case middleware.Resolving:
				pi.resolving = append(pi.resolving, name)
			case middleware.PostRouting:
				pi.postRouting = append(pi.postRouting, name)
			}
		}
		s.middlewareIndex = pi
	}
	return s.middlewareIndex

}

func AnswerMsg(r *dns.RR) *dns.Msg {
	responseMsg := new(dns.Msg)
	responseMsg.Answer = append(responseMsg.Answer, *r)
	return responseMsg

}

func (s *Server) process(ctx context.Context, requestMsg *dns.Msg) (*dns.Msg, error) {
	logger := log.GetLogger("server", "process")
	pi := s.getIndexes()
	//Handle Pre-routing
	req := new(middleware.Message)
	req.SetCtx(ctx)
	req.SetMsg(requestMsg)

	for _, p := range pi.preRouting {
		name, _ := s.Middlewares[p].Info()
		logger.Debug("Calling pre-routing middleware ", name)
		req, err := s.Middlewares[p].Run(req)
		if err != nil {
			continue
		}
		if req.IsResolved() {
			break
		}
	}
	// If we didn't resolve at pre-routing time, then try resolving middlewares.
	req, err := s.tryResolve(req, pi)
	if err != nil {
		return nil, err
	}

	// No response from resolving middlewares, just resolve with the default upstream.
	if !req.IsResolved() {
		_m, err := s.resolve(req.Answer())
		if err != nil {
			logger := log.GetLogger("Server", "process")
			logger.Error(err)
			return nil, err
		}
		req.SetMsg(_m)
		req.Resolved(true)
	}

	for _, p := range pi.postRouting {
		req, err = s.Middlewares[p].Run(req)
		if err != nil {
			logger := log.GetLogger("Server", "post-process")
			name, _ := s.Middlewares[p].Info()
			logger.Debugf("Middleware [%s] returning err : %s", name, err.Error())
		}

	}
	if req != nil {
		return req.Answer(), nil
	} else {
		return nil, errors.New("middleware error")
	}
}

// try resolving middlewares.
func (s *Server) tryResolve(req *middleware.Message, pi *MiddlewareIndex) (*middleware.Message, error) {
	logger := log.GetLogger("Server", "tryResolve")
	if !req.IsResolved() {
		for _, p := range pi.resolving {
			//Todo: change middleware.Run 2nd argument to be a full response and not a bool and use it instead of building one.
			//	also, dont cache.
			name, _ := s.Middlewares[p].Info()
			logger.Debug("Calling resolving middleware ", name)
			req, err := s.Middlewares[p].Run(req)
			if err != nil {
				return nil, err
			}
			if req.IsResolved() {
				break
			}
		}
	}
	return req, nil
}

func (s *Server) StubsToggle(state bool) bool {
	s.Middlewares["stub-resolver"].(*middleware.StubResolver).SetEnabled(state)
	return s.Middlewares["stub-resolver"].(*middleware.StubResolver).IsEnabled()
}

func (s *Server) BlacklistToggle(state bool) bool {
	c := config.GetRunningConfig()
	c.Blacklist.Enabled = state
	config.SetRunningConfig(c)
	s.Middlewares["blacklist"].(*middleware.BlackList).SetEnabled(state)

	return c.Blacklist.Enabled
}

func (s *Server) ClearCache() error {
	c := middleware.GetCache()
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

		Config:      *config.GetRunningConfig(),
		Middlewares: make(map[string]middleware.Middleware),
	}
	for _, o := range options {
		o(s)
	}
	return s
}
