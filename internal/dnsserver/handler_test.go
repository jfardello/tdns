package dnsserver

import (
	"net"
	"testing"

	"github.com/jfardello/tdns/server"
	"github.com/miekg/dns"
)

type fakeResolver struct {
	calls int
	err   error
}

func (r *fakeResolver) Handler(request *dns.Msg, _ net.Addr) (*dns.Msg, error) {
	r.calls++
	if r.err != nil {
		return nil, r.err
	}
	response := new(dns.Msg)
	response.SetReply(request)
	return response, nil
}

type fakeResponseWriter struct {
	remote net.Addr
	writes []*dns.Msg
}

func (w *fakeResponseWriter) LocalAddr() net.Addr       { return &net.UDPAddr{} }
func (w *fakeResponseWriter) RemoteAddr() net.Addr      { return w.remote }
func (w *fakeResponseWriter) WriteMsg(m *dns.Msg) error { w.writes = append(w.writes, m); return nil }
func (w *fakeResponseWriter) Write([]byte) (int, error) { return 0, nil }
func (w *fakeResponseWriter) Close() error              { return nil }
func (w *fakeResponseWriter) TsigStatus() error         { return nil }
func (w *fakeResponseWriter) TsigTimersOnly(bool)       {}
func (w *fakeResponseWriter) Hijack()                   {}

func TestHandlerRejectsUnauthorizedClientBeforeResolver(t *testing.T) {
	policy, err := NewPolicy(testDNSAccessConf())
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	resolver := &fakeResolver{}
	handler := NewHandler(policy, resolver)
	writer := &fakeResponseWriter{remote: &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 53000}}

	handler.ServeDNS(writer, testQuery())

	if resolver.calls != 0 || len(writer.writes) != 0 {
		t.Fatalf("unauthorized request reached resolver or response: calls=%d writes=%d", resolver.calls, len(writer.writes))
	}
}

func TestHandlerDropsClientRateLimitedQuery(t *testing.T) {
	conf := testDNSAccessConf()
	conf.ClientBurst = 1
	policy, err := NewPolicy(conf)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	resolver := &fakeResolver{}
	handler := NewHandler(policy, resolver)
	writer := &fakeResponseWriter{remote: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53000}}

	handler.ServeDNS(writer, testQuery())
	handler.ServeDNS(writer, testQuery())

	if resolver.calls != 1 || len(writer.writes) != 1 {
		t.Fatalf("client limiter admitted too much work: calls=%d writes=%d", resolver.calls, len(writer.writes))
	}
}

func TestHandlerReturnsServerFailureForAuthorizedSaturationWithBudget(t *testing.T) {
	policy, err := NewPolicy(testDNSAccessConf())
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	handler := NewHandler(policy, &fakeResolver{err: server.ErrUpstreamSaturated})
	writer := &fakeResponseWriter{remote: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53000}}

	handler.ServeDNS(writer, testQuery())

	if len(writer.writes) != 1 || writer.writes[0].Rcode != dns.RcodeServerFailure {
		t.Fatalf("saturated authorized response = %#v, want one SERVFAIL", writer.writes)
	}
}

func TestHandlerDropsServerFailureWhenResponseBudgetIsExhausted(t *testing.T) {
	conf := testDNSAccessConf()
	conf.GlobalResponseBurst = 1
	policy, err := NewPolicy(conf)
	if err != nil {
		t.Fatalf("NewPolicy: %v", err)
	}
	if !policy.allowResponse() {
		t.Fatal("failed to consume initial response budget")
	}
	handler := NewHandler(policy, &fakeResolver{err: server.ErrUpstreamSaturated})
	writer := &fakeResponseWriter{remote: &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 53000}}

	handler.ServeDNS(writer, testQuery())

	if len(writer.writes) != 0 {
		t.Fatalf("wrote %d responses after response budget was exhausted", len(writer.writes))
	}
}

func testQuery() *dns.Msg {
	request := new(dns.Msg)
	request.SetQuestion("example.com.", dns.TypeA)
	return request
}
