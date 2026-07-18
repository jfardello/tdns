package resolver

import (
	"crypto/tls"
	"strings"
	"testing"
	"time"

	"github.com/miekg/dns"
)

type FakeClient struct {
	Net       string
	TLSConfig *tls.Config
	Store     map[string]string
	Wait      time.Duration
	Fail      bool
}

func (f *FakeClient) Exchange(m *dns.Msg, _ string) (*dns.Msg, time.Duration, error) {
	time.Sleep(f.Wait)
	response := new(dns.Msg)
	domain := m.Question[0].Name
	var rr dns.RR
	rr, _ = dns.NewRR(domain + " A 127.0.0.5")
	if val, ok := f.Store[domain]; ok {
		rr, _ = dns.NewRR(domain + " A " + val)
	}

	if f.Fail {
		response.Rcode = dns.RcodeServerFailure
		return response, 100, nil

	}
	response.Answer = append(response.Answer, rr)
	return response, 100, nil

}

type FStore map[string]string

func GetFakeClient(store FStore, w time.Duration, fail bool) Exchanger {
	f := &FakeClient{Wait: w, Fail: fail}
	f.Store = store
	return f
}

type resolveStruct struct {
	name    string
	query   string
	want    string
	mux     Mux
	wantErr bool
}

func TestClientMux_Resolve(t *testing.T) {

	client1 := GetFakeClient(FStore{"foo.tld.": "127.0.0.2", "bar.tld.": "127.0.0.1"}, 90*time.Millisecond, false)
	client2 := GetFakeClient(FStore{"foo.tld.": "127.0.0.99", "bar.tld.": "127.0.0.3"}, 5, false)
	client3 := GetFakeClient(FStore{"foo.tld.": "127.0.0.99", "bar.tld.": "127.0.0.3"}, 90*time.Millisecond, true)

	mux1 := &Mux{
		Upstreams: []*Upstream{
			{
				NetType: NetTLS,
				Address: "127.0.0.9",
				Client:  client1,
			},
			{
				NetType: NetTLS,
				Address: "127.0.0.10",
				Client:  client2,
			},
		},
		globalTimeout: 120 * time.Millisecond,
	}

	mux2 := &Mux{
		Upstreams: []*Upstream{
			{
				NetType: NetTLS,
				Address: "127.0.0.10",
				Client:  client3,
			},
		},
		globalTimeout: 120 * time.Millisecond,
	}

	tests := []resolveStruct{
		{name: "test1", query: "foo.tld.", mux: *mux1, want: "127.0.0.2"},
		{name: "test2", query: "bar.tld.", mux: *mux1, want: "127.0.0.1"},
		{name: "test3", query: "bar.tld.", mux: *mux2, want: "127.0.0.3", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			m := new(dns.Msg)
			m.SetQuestion(tt.query, dns.TypeA)

			gotR, _, err := tt.mux.Resolve(m)
			if tt.wantErr && err == nil {
				t.Errorf("ClientMux.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				return
			}

			if !strings.HasSuffix(gotR.Answer[0].String(), tt.want) {
				t.Errorf("ClientMux.Resolve() query=%s gotR = %v, want %v", tt.query, gotR.Answer[0].String(), tt.want)
			}

		})
	}
}

func TestNewClientMux(t *testing.T) {

	var testOpts = []UpstreamMuxOption{WithGlobalTimeout(90 * time.Millisecond), WithMuxUpstreamOptions(WithTimeout(40 * time.Millisecond))}
	type args struct {
		servers []string
		opts    []UpstreamMuxOption
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{name: "testnew_01", args: args{servers: []string{"tcp://localhost:99953", "tcp://localhost:99954"}, opts: testOpts}, want: 40 * time.Millisecond},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewClientMux(tt.args.servers, tt.args.opts...)
			if got.Upstreams[0].Timeout != tt.want {
				t.Errorf("ClientMux.upstreams[0].Timeout = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClientMuxPanic(t *testing.T) {

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	var testOpts = []UpstreamMuxOption{WithGlobalTimeout(90 * time.Millisecond), WithMuxUpstreamOptions(WithTimeout(40 * time.Millisecond))}
	type args struct {
		servers []string
		opts    []UpstreamMuxOption
	}
	test := struct {
		name    string
		args    args
		want    time.Duration
		wantErr bool
	}{name: "testnew_02", args: args{servers: []string{"tcp://loc alh ost:99953", "tcp://localhost:99954"}, opts: testOpts}, want: 40 * time.Millisecond, wantErr: true}

	NewClientMux(test.args.servers, test.args.opts...)

}

func TestNewUpstream(t *testing.T) {
	type args struct {
		upstreamUrl string
		opts        []UpstreamOption
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{name: "newupstream01", args: args{upstreamUrl: "tls://127.1.1.2:9090#foo.tld"}, want: "127.1.1.2:9090"},
		{name: "newupstream02", args: args{upstreamUrl: "tls 127.1.1 .2:9090#foo.tld"}, wantErr: true},
		{name: "newupstream01", args: args{upstreamUrl: "udp://127.1.1.2:9090"}, want: "127.1.1.2:9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewUpstream(tt.args.upstreamUrl, tt.args.opts...)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewUpstream() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			} else if got.Address != tt.want {
				t.Errorf("NewUpstream().Address = %+v, want %+v", got.Address, tt.want)
			}
		})
	}
}

func TestClientMuxSRVFail(t *testing.T) {
	client1 := GetFakeClient(FStore{"foo.tld.": "127.0.0.2", "bar.tld.": "127.0.0.1"}, 0, true) //fail will jump to the next upstream.
	client2 := GetFakeClient(FStore{"foo.tld.": "127.0.0.99", "bar.tld.": "127.0.0.3"}, 0, false)

	mux1 := &Mux{
		Upstreams: []*Upstream{
			{
				NetType: NetTLS,
				Address: "127.0.0.9",
				Client:  client1,
			},
			{
				NetType: NetTLS,
				Address: "127.0.0.10",
				Client:  client2,
			},
		},
		globalTimeout: 40 * time.Millisecond,
	}
	tests := []struct {
		name    string
		query   string
		want    string
		mux     Mux
		wantErr bool
	}{
		{name: "test1", query: "foo.tld.", mux: *mux1, want: "127.0.0.99"},
		{name: "test2", query: "bar.tld.", mux: *mux1, want: "127.0.0.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			m := new(dns.Msg)
			m.SetQuestion(tt.query, dns.TypeA)

			gotR, _, err := tt.mux.Resolve(m)
			if tt.wantErr && err == nil {
				t.Errorf("ClientMux.Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				return
			}

			if !strings.HasSuffix(gotR.Answer[0].String(), tt.want) {
				t.Errorf("ClientMux.Resolve() query=%s gotR = %v, want %v", tt.query, gotR.Answer[0].String(), tt.want)
			}

		})
	}

}
