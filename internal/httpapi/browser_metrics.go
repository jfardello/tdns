package httpapi

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	browserAuthenticationAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tdns_browser_authentication_attempts_total",
		Help: "Browser authentication attempts by method and bounded outcome category.",
	}, []string{"method", "outcome"})
	passwordAuthenticationDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tdns_password_authentication_duration_seconds",
		Help:    "Time spent verifying a local administrator credential and creating its session.",
		Buckets: prometheus.DefBuckets,
	})
)

func recordBrowserAuthentication(method, outcome string) {
	if method != "password" {
		method = "other"
	}
	switch outcome {
	case "unavailable", "ambiguous", "cross_site", "malformed", "oversized", "rate_limited", "invalid", "error", "success":
	default:
		outcome = "other"
	}
	browserAuthenticationAttempts.WithLabelValues(method, outcome).Inc()
}
