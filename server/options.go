package server

import (
	"github.com/jfardello/tdns/config"
	"time"

	"github.com/armon/go-radix"
	"github.com/jfardello/tdns/client"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/plugin"
)

func WithStaticResponse() func(*Server) {

	c := config.GetRunningConfig()
	hostFile := c.Static.File
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		if hostFile != "" {
			st := &plugin.StaticResponsePlugin{}
			err := st.Config(s.Config)
			if err != nil {
				logger.Fatal(err)
			}
			err = st.Init()
			if err != nil {
				logger.Fatal(err)
			}
			n, _ := st.Info()
			s.Plugins[n] = st

			logger.Infof("Loaded %d hosts", len(st.Hosts))
		}
	}
}

func WithStatus() func(*Server) {
	return func(s *Server) {
		sp := &plugin.StatusPlugin{}
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
		s.Plugins[n] = sp

		logger.Infof("Loaded Status plugin")
	}
}

func WithZenPlugin() func(*Server) {

	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		z := &plugin.ZenmodePlugin{}
		err := z.Config(s.Config)
		if err != nil {
			logger.Fatal(err)
		}
		err = z.Init()
		if err != nil {
			logger.Fatal(err)
		}
		n, _ := z.Info()
		s.Plugins[n] = z

		logger.Infof("Loaded %d hosts for zen mode", len(z.Hosts))
	}
}

func WithUpstreams(u []string) func(*Server) {
	return func(s *Server) {
		ds := client.NewClientMux(u,
			client.WithMuxUpstreamOptions(client.WithTimeout(1000*time.Millisecond)),
			client.WithGlobalTimeout(1000*time.Millisecond))
		s.defaultUpstream = *ds
	}
}

func WithDNSLog() func(*Server) {
	return func(s *Server) {
		if s.Config.DNSLog.Enabled {
			logger := log.GetLogger("serve", "config")
			dl := &plugin.DNSLog{}
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
			logger.Debugf("About to add dnslog to plug resitry: %#v", dl)
			s.Plugins[n] = dl
		}
	}
}

func WithStubs(u []string) func(*Server) {
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")

		stubs, err := plugin.ParseStubList(u)
		if err != nil {
			logger.Errorf("Malformed stub strings: %#v", u)
			return
		}
		p := &plugin.StubresolverPlugin{
			EnableStubs: true,
			Stubs:       stubs,
		}
		c := config.GetRunningConfig()
		err = p.Config(*c)
		if err != nil {
			logger.Fatal(err)
		}
		err = p.Init()
		if err != nil {
			logger.Fatal(err)
		}
		n, _ := p.Info()
		s.Plugins[n] = p
		logger.Infof("Loaded %d stubs", len(p.Stubs))
	}
}

func WithBHoleList() func(*Server) {
	c := config.GetRunningConfig()
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		if c.BlackHole.File != "" {
			b := &plugin.BlackListPlugin{
				Hole: radix.New(),
			}
			err := b.Config(s.Config)
			if err != nil {
				logger.Fatal(err)
			}
			err = b.Init()
			if err != nil {
				logger.Fatal(err)
			}
			n, _ := b.Info()
			s.Plugins[n] = b
			logger.Infof("Loaded %d hosts th the black hole list.", b.Hole.Len())
		}
	}
}

func WithCacheGet() func(*Server) {
	return func(s *Server) {
		c := &plugin.CacheGet{}
		err := c.Init()
		if err != nil {
			panic(err)
		}
		n, _ := c.Info()
		s.Plugins[n] = c
	}
}
func WithCacheSet() func(*Server) {
	return func(s *Server) {
		c := &plugin.CacheSet{}
		err := c.Init()
		if err != nil {
			panic(err)
		}
		n, _ := c.Info()
		s.Plugins[n] = c
	}
}
