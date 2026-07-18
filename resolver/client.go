package resolver

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/url"
	"time"

	"github.com/jfardello/tdns/log"
	"github.com/miekg/dns"
)

type NetType int

const (
	NetTLS NetType = iota
	NetTCP
	NetUDP
)

type Exchanger interface {
	Exchange(*dns.Msg, string) (*dns.Msg, time.Duration, error)
}

type Upstream struct {
	NetType NetType
	Address string
	TLSName string
	//Exchanger interface helps mocking the client in unittests.
	Client  Exchanger
	Timeout time.Duration
}

func (u *Upstream) BuildClient() {
	logger := log.GetLogger("Upstream", "BuildClient")
	logger.Debugf("Build client for %s, with timeout: %d", u.Address, u.Timeout/time.Millisecond)
	var stype string
	switch u.NetType {
	case NetTCP:
		stype = "tcp"
	case NetTLS:
		stype = "tcp-tls"
	default:
		stype = "udp"
	}
	if u.TLSName != "" {
		u.Client = &dns.Client{Net: stype, Timeout: u.Timeout, TLSConfig: &tls.Config{
			VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
				logger.Debugf("Verifying host name: %s", u.TLSName)
				for _, rawCert := range rawCerts {
					cert, _ := x509.ParseCertificate(rawCert)
					if !cert.IsCA {
						return cert.VerifyHostname(u.TLSName)
					}
				}
				return nil
			},
		}}
	} else {
		u.Client = &dns.Client{Net: stype, Timeout: u.Timeout}
	}

}

type Mux struct {
	Upstreams     []*Upstream
	globalTimeout time.Duration
	clientOptions []UpstreamOption
}

func (c *Mux) Resolve(m *dns.Msg) (r *dns.Msg, rtt time.Duration, err error) {

	logger := log.GetLogger("ClientMux", "resolve")

	select {

	case <-time.After(c.globalTimeout):
		return nil, c.globalTimeout, errors.New("reached global mux time-out")
	default:

		for _, u := range c.Upstreams {

			logger.Debugf("Quering %s about %s", u.Address, m.Question[0].Name)
			r, rtt, err := u.Client.Exchange(m, u.Address)
			if r != nil && r.Rcode != dns.RcodeSuccess {
				logger.Errorf("Can't get an answer for question %s from upstream %s", m.Question[0].Name, u.Address)
				if r.Rcode == dns.RcodeServerFailure {
					continue
				}

			}
			return r, rtt, err
		}
		return nil, 0, errors.New("no response")
	}
}

type UpstreamOption func(*Upstream)

func WithTimeout(t time.Duration) UpstreamOption {
	return func(u *Upstream) {
		u.Timeout = t
	}
}

// NewUpstream returns a new Upstream from a url string valid schemas are, "tls", "tcp", and "udp"  .
// url-part is reserved for tls domain validation.
// ex: tls://1.1.1.1:853/#cloudflare-dns.com
func NewUpstream(upstreamUrl string, opts ...UpstreamOption) (*Upstream, error) {

	u, err := url.Parse(upstreamUrl)
	if err != nil {
		return nil, err
	}

	var schema NetType
	var dp string

	switch u.Scheme {
	case "tls":
		schema = NetTLS
		if u.Port() == "" {
			dp = ":853"
		}
	case "tcp":
		schema = NetTCP
		if u.Port() == "" {
			dp = ":53"
		}

	default:
		schema = NetUDP
		if u.Port() == "" {
			dp = ":53"
		}
	}

	up := &Upstream{
		NetType: schema,
		Address: u.Host + dp,
		TLSName: u.Fragment,
	}
	for _, o := range opts {
		o(up)
	}
	up.BuildClient()
	return up, nil
}

type UpstreamMuxOption func(*Mux)

func WithGlobalTimeout(t time.Duration) UpstreamMuxOption {
	return func(u *Mux) {
		u.globalTimeout = t
	}
}

func WithMuxUpstreamOptions(opts ...UpstreamOption) UpstreamMuxOption {
	return func(m *Mux) {
		m.clientOptions = opts
	}
}

func NewClientMux(servers []string, opts ...UpstreamMuxOption) *Mux {

	m := &Mux{}
	for _, o := range opts {
		o(m)
	}

	for _, server := range servers {
		u, err := NewUpstream(server, m.clientOptions...)

		if err != nil {
			panic(err)
		}
		m.Upstreams = append(m.Upstreams, u)
	}
	return m

}
