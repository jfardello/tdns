package middleware

import (
	"time"

	internalblocklist "github.com/jfardello/tdns/internal/blocklist"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	blacklistRefreshes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "tdns_blacklist_refresh_total",
		Help: "Remote blocklist refresh attempts by bounded result category.",
	}, []string{"result"})
	blacklistRefreshDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tdns_blacklist_refresh_duration_seconds",
		Help:    "Time spent refreshing the remote blocklist.",
		Buckets: prometheus.DefBuckets,
	})
	blacklistCompressedBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tdns_blacklist_download_compressed_bytes",
		Help:    "Compressed bytes consumed by successful remote blocklist downloads.",
		Buckets: prometheus.ExponentialBuckets(1024, 4, 9),
	})
	blacklistUncompressedBytes = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "tdns_blacklist_download_uncompressed_bytes",
		Help:    "Uncompressed bytes produced by successful remote blocklist downloads.",
		Buckets: prometheus.ExponentialBuckets(1024, 4, 10),
	})
	blacklistActiveEntries = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tdns_blacklist_active_entries",
		Help: "Number of entries in the active in-memory blocklist.",
	})
	blacklistLastSuccess = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "tdns_blacklist_last_success_timestamp_seconds",
		Help: "Unix timestamp of the last successful or unchanged remote blocklist refresh.",
	})
)

func recordBlacklistRefresh(started time.Time, result string, refresh internalblocklist.Result, activeEntries int) {
	switch result {
	case "success", "unchanged", "invalid", "timeout", "too_large", "redirect_rejected", "remote_error", "io_error":
	default:
		result = "other"
	}
	blacklistRefreshes.WithLabelValues(result).Inc()
	blacklistRefreshDuration.Observe(time.Since(started).Seconds())
	if result == "success" {
		blacklistCompressedBytes.Observe(float64(refresh.CompressedBytes))
		blacklistUncompressedBytes.Observe(float64(refresh.UncompressedBytes))
	}
	if result == "success" || result == "unchanged" {
		blacklistActiveEntries.Set(float64(activeEntries))
		blacklistLastSuccess.SetToCurrentTime()
	}
}
