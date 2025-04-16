package plugin

import (
	"context"
	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
	"net"
	"testing"
)

func TestStubresolverPlugin_Run(t *testing.T) {

	stubs, err := ParseStubList([]string{"google.es,udp://8.8.8.8"})
	if err != nil {
		t.Fatalf("Malformed stub strings: %#v", err)
		return
	}
	p := &StubresolverPlugin{
		EnableStubs: true,
		Stubs:       stubs,
	}

	t.Run("testStub", func(t *testing.T) {
		v := config.CtxValue{RemoteAddr: &net.UDPAddr{IP: net.ParseIP("1.1.1.1")}}
		ctx := context.WithValue(context.Background(), config.CtxKey, v)
		m := new(dns.Msg)
		m.SetQuestion("www.google.es.", dns.TypeANY)
		got, _, err := p.Run(ctx, m)
		if err != nil {
			t.Errorf("Run() error = %v", err)
			return

		}
		t.Logf("%#v", *got)

	})
}
