package middleware

import (
	"context"
	"time"

	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
	"net"
	"testing"
)

type stubTestExchanger struct {
	rcode  int
	answer string
}

func (s stubTestExchanger) Exchange(m *dns.Msg, _ string) (*dns.Msg, time.Duration, error) {
	resp := new(dns.Msg)
	resp.SetReply(m)
	resp.Rcode = s.rcode
	if s.answer != "" {
		rr, err := dns.NewRR(s.answer)
		if err != nil {
			return nil, 0, err
		}
		resp.Answer = append(resp.Answer, rr)
	}
	return resp, 0, nil
}

func TestStubResolver_Run(t *testing.T) {
	tests := []struct {
		Name     string
		StubSpec string
		Domain   string
		Client   stubTestExchanger
		WantErr  bool
	}{
		{
			Name:     "ggl",
			StubSpec: "google.es,udp://127.0.0.1:5300",
			Domain:   "www.google.es.",
			Client: stubTestExchanger{
				rcode:  dns.RcodeSuccess,
				answer: "www.google.es. 60 IN A 127.0.0.5",
			},
			WantErr: false,
		},
		{
			Name:     "nxdomain",
			StubSpec: "google.es,udp://127.0.0.1:5300",
			Domain:   "zzzzzzzzz.google.es.",
			Client: stubTestExchanger{
				rcode: dns.RcodeNameError,
			},
			WantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			stubs, err := ParseStubList([]string{tt.StubSpec}, 1000, 300)
			if err != nil {
				t.Fatalf("Malformed stub strings: %#v", err)
			}
			stubs["google.es"].Upstreams[0].Client = tt.Client
			p := &StubResolver{
				EnableStubs: true,
				Stubs:       stubs,
			}
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
