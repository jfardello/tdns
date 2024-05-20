package server

import (
	"time"

	"github.com/armon/go-radix"
	"github.com/jfardello/tdns/client"
	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/plugin"
)

func WithStaticResponse(hostFile string) func(*Server) {

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

func WithZenFile(zenFile string) func(*Server) {

	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		if zenFile != "" {

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
}

func WithUpstreams(u []string) func(*Server) {
	return func(s *Server) {
		ds := client.NewClientMux(u,
			client.WithMuxUpstreamOptions(client.WithTimeout(1000*time.Millisecond)),
			client.WithGlobalTimeout(1000*time.Millisecond))
		s.defaultUpstream = *ds
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
		err = p.Init()
		if err != nil {
			logger.Fatal(err)
		}
		n, _ := p.Info()
		s.Plugins[n] = p
		logger.Infof("Loaded %d stubs", len(stubs))
	}
}

func WithBHoleList(holeFile string) func(*Server) {
	return func(s *Server) {
		logger := log.GetLogger("serve", "config")
		if holeFile != "" {
			s.Config.BlackHoleFile = holeFile

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
