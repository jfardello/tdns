package middleware

import (
	"context"
	"net"
	"testing"

	"github.com/jfardello/tdns/config"
	"github.com/miekg/dns"
)

func TestZenModeRunRespectsClientLabels(t *testing.T) {
	zen := &ZenMode{
		enabled: true,
		Hosts: map[string]string{
			"x.com": "0.0.0.0",
		},
		labels: []string{"kids"},
	}

	msg := new(dns.Msg)
	msg.SetQuestion("www.x.com.", dns.TypeA)

	request := &Message{}
	request.SetMsg(msg)
	request.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("1.1.1.1")},
		Labels:     []string{"work"},
		Values:     map[string]string{},
	}))

	response, err := zen.Run(request)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if response.IsResolved() {
		t.Fatal("expected zen mode to skip unmatched labels")
	}
}

func TestStaticResponseRunRespectsClientLabels(t *testing.T) {
	static := &StaticResponse{
		Enabled: true,
		Hosts: map[string]string{
			"example.org": "127.0.0.1",
		},
		labels: []string{"family"},
	}

	msg := new(dns.Msg)
	msg.SetQuestion("www.example.org.", dns.TypeA)

	request := &Message{}
	request.SetMsg(msg)
	request.SetCtx(context.WithValue(context.Background(), config.CtxKey, config.CtxValue{
		RemoteAddr: &net.UDPAddr{IP: net.ParseIP("1.1.1.1")},
		Labels:     []string{"guest"},
		Values:     map[string]string{},
	}))

	response, err := static.Run(request)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if response.IsResolved() {
		t.Fatal("expected static response to skip unmatched labels")
	}
}
