package middleware

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	dnsLogPurges = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tdns_dns_log_purges_total",
		Help: "DNS-log purge attempts by bounded outcome category.",
	}, []string{"outcome"})
	dnsLogPurgeDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tdns_dns_log_purge_duration_seconds",
		Help:    "Time spent deleting expired DNS-log records and compacting SQLite.",
		Buckets: prometheus.DefBuckets,
	})
	dnsLogPurgedRows = promauto.NewCounter(prometheus.CounterOpts{
		Name: "tdns_dns_log_purged_rows_total",
		Help: "DNS-log records removed by successful purge operations.",
	})
	dnsLogPurgeLastSuccess = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tdns_dns_log_purge_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful DNS-log purge.",
	})
)

func recordDNSLogPurge(started time.Time, deleted int64, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	dnsLogPurges.WithLabelValues(outcome).Inc()
	dnsLogPurgeDuration.Observe(time.Since(started).Seconds())
	if err == nil {
		dnsLogPurgedRows.Add(float64(deleted))
		dnsLogPurgeLastSuccess.SetToCurrentTime()
	}
}
