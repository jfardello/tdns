package diagnostics

import (
	"net/http"
	pprof "net/http/pprof"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewHandler(metricsEnabled, pprofEnabled bool) http.Handler {
	mux := http.NewServeMux()
	if metricsEnabled {
		mux.Handle("GET /metrics", promhttp.Handler())
	}
	if pprofEnabled {
		mux.HandleFunc("GET /debug/pprof/", pprof.Index)
		mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("POST /debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)
	}
	return mux
}
