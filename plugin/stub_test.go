package plugin

import (
	"context"
	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
	"net"
	"testing"
)

func TestStubresolverPlugin_Run(t *testing.T) {

	stubs, err := ParseStubList([]string{"google.es,udp://8.8.8.8"}, 1000, 300)
	if err != nil {
		t.Fatalf("Malformed stub strings: %#v", err)
		return
	}
	p := &StubresolverPlugin{
		EnableStubs: true,
		Stubs:       stubs,
	}
	tests := []struct {
		Name    string
		Domain  string
		WantErr bool
	}{
		{
			Name:    "ggl",
			Domain:  "www.google.es.",
			WantErr: false,
		},
		{
			Name:    "nxdomain",
			Domain:  "zzzzzzzzz.google.es.",
			WantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			v := config.CtxValue{
				RemoteAddr: &net.UDPAddr{IP: net.ParseIP("1.1.1.1")},
				Values:     map[string]string{},
			}
			ctx := context.WithValue(context.Background(), config.CtxKey, v)
			m := new(dns.Msg)
			mess := &Message{}
			m.SetQuestion(tt.Domain, dns.TypeANY)
			mess.SetMsg(m)
			mess.SetCtx(ctx)

			got, err := p.Run(mess)

			if !tt.WantErr && err != nil {
				t.Errorf("Run() error = %v", err)
				return
			}
			t.Logf("%#v", got)
		})
	}
}
