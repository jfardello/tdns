package server

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	upstreamInflight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tdns_dns_upstream_inflight",
		Help: "Number of DNS requests currently performing stub or default upstream work.",
	})
	upstreamLimit = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tdns_dns_upstream_limit",
		Help: "Configured maximum concurrent stub or default upstream work.",
	})
	upstreamSaturation = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tdns_dns_upstream_saturation_total",
		Help: "DNS requests rejected because all upstream concurrency slots were occupied.",
	})
)
