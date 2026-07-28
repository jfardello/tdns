package dnsserver

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dnsRejections = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tdns_dns_rejections_total",
		Help: "DNS requests or responses rejected by a bounded admission control.",
	}, []string{"reason"})
	trackedDNSClients = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tdns_dns_tracked_clients",
		Help: "Number of client addresses currently held by the bounded per-client rate limiter.",
	})
)
