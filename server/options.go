package server

import (
	"fmt"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/middleware"
	"github.com/jfardello/tdns/resolver"
)

var DEFAULT_TIMEOUT_MILLIS int = 1000

func WithStaticResponse() func(*Server) {

	c := config.GetRunningConfig()
	hostFile := c.StaticResponse.File
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		if hostFile != "" {
			st := &middleware.StaticResponse{}
			err := st.Config(s.Config)
			if err != nil {
				logger.Fatal(err)
			}
			err = st.Init()
			if err != nil {
				logger.Fatal(err)
			}
			n, _ := st.Info()
			s.Middlewares[n] = st

			logger.Infof("Loaded %d hosts", len(st.Hosts))
		}
	}
}

func WithStatus() func(*Server) {
	return func(s *Server) {
		sp := &middleware.Status{}
		err := sp.Config(s.Config)
		logger := log.GetLogger("serve", "config")
		if err != nil {
			logger.Fatal(err)
		}
		err = sp.Init()
		if err != nil {
			logger.Fatal(err)
		}
		n, _ := sp.Info()
		s.Middlewares[n] = sp

		logger.Infof("Loaded status middleware")
	}
}

func WithZenMode() func(*Server) {

	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		z := &middleware.ZenMode{}
		err := z.Config(s.Config)
		if err != nil {
			logger.Fatal(err)
		}
		err = z.Init()
		if err != nil {
			logger.Fatal(err)
		}
		n, _ := z.Info()
		s.Middlewares[n] = z

		logger.Infof("Loaded %d hosts for zen mode", len(z.Hosts))
	}
}

func WithUpstreams(u []string, globalTimeOut int, upstreamTimeOut int) func(*Server) {

	return func(s *Server) {
		ds := resolver.NewClientMux(u,
			resolver.WithMuxUpstreamOptions(resolver.WithTimeout(time.Duration(upstreamTimeOut)*time.Millisecond)),
			resolver.WithGlobalTimeout(time.Duration(globalTimeOut)*time.Millisecond))
		s.defaultUpstream = *ds
	}
}

func WithDNSLog() func(*Server) {
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		if err := configureDNSLog(s); err != nil {
			logger.Fatal(err)
		}
	}
}

func configureDNSLog(s *Server) error {
	dl := &middleware.DNSLog{}
	if err := dl.Config(s.Config); err != nil {
		return fmt.Errorf("initialize DNS-log middleware: %w", err)
	}
	if err := dl.Init(); err != nil {
		dl.Stop()
		return fmt.Errorf("initialize DNS-log middleware: %w", err)
	}
	n, _ := dl.Info()
	s.Middlewares[n] = dl
	return nil
}

func WithTagger() func(*Server) {
	return func(s *Server) {
		if s.Config.Tagger.Enabled {
			logger := log.GetLogger("serve", "config")
			dl := &middleware.Tagger{}
			err := dl.Config(s.Config)
			if err != nil {
				logger.Error(err)
				return
			}
			err = dl.Init()
			if err != nil {
				logger.Error(err)
				return
			}
			n, _ := dl.Info()
			s.Middlewares[n] = dl
		}
	}
}

func WithStubs(u []string, globalTimeOut int, upstreamTimeout int) func(*Server) {
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		c := config.GetRunningConfig()

		stubs, err := middleware.ParseStubList(u, globalTimeOut, upstreamTimeout)
		if err != nil {
			logger.Errorf("Malformed stub strings: %#v", u)
			return
		}
		p := &middleware.StubResolver{
			EnableStubs: c.StubResolver.Enabled,
			Stubs:       stubs,
		}
		err = p.Config(*c)
		if err != nil {
			logger.Fatal(err)
		}
		err = p.Init()
		if err != nil {
			logger.Fatal(err)
		}
		n, _ := p.Info()
		s.Middlewares[n] = p
		logger.Infof("Loaded %d stubs", len(p.Stubs))
	}
}

func WithBlacklist() func(*Server) {
	c := config.GetRunningConfig()
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		if c.Blacklist.File != "" {
			b := &middleware.BlackList{}
			err := b.Config(s.Config)
			if err != nil {
				logger.Fatal(err)
			}
			err = b.Init()
			if err != nil {
				logger.Fatal(err)
			}
			n, _ := b.Info()
			s.Middlewares[n] = b
			logger.Infof("Loaded %d hosts from the black hole list.", b.Hole.Len())
		}
	}
}

func WithCacheGet() func(*Server) {
	return func(s *Server) {
		c := &middleware.CacheGet{}
		err := c.Init()
		if err != nil {
			panic(err)
		}
		n, _ := c.Info()
		s.Middlewares[n] = c
	}
}
func WithCacheSet() func(*Server) {
	return func(s *Server) {
		c := &middleware.CacheSet{}
		err := c.Init()
		if err != nil {
			panic(err)
		}
		n, _ := c.Info()
		s.Middlewares[n] = c
	}
}
