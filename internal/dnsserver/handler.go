package dnsserver

import (
	"errors"
	"net"

	"github.com/jfardello/tdns/log"
	"github.com/jfardello/tdns/server"
	"github.com/miekg/dns"
)

type Resolver interface {
	Handler(request *dns.Msg, remote net.Addr) (*dns.Msg, error)
}

type Handler struct {
	policy   *Policy
	resolver Resolver
}

func NewHandler(policy *Policy, resolver Resolver) *Handler {
	return &Handler{policy: policy, resolver: resolver}
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, request *dns.Msg) {
	if _, _, admitted := h.policy.admit(w.RemoteAddr()); !admitted {
		return
	}
	if request.Opcode != dns.OpcodeQuery {
		return
	}

	response, err := h.resolver.Handler(request, w.RemoteAddr())
	if err != nil {
		if errors.Is(err, server.ErrUpstreamSaturated) {
			dnsRejections.WithLabelValues("upstream_saturated").Inc()
		} else {
			log.GetLogger("dnsserver", "resolve").Error(err)
		}
		response = new(dns.Msg)
		response.SetRcode(request, dns.RcodeServerFailure)
	} else if response == nil {
		response = new(dns.Msg)
		response.SetRcode(request, dns.RcodeServerFailure)
	} else {
		response.SetReply(request)
	}

	if !h.policy.allowResponse() {
		return
	}
	if err := w.WriteMsg(response); err != nil {
		log.GetLogger("dnsserver", "response").Error(err)
	}
}
