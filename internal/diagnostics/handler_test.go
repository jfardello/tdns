package diagnostics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerSeparatesOptionalDiagnostics(t *testing.T) {
	for _, test := range []struct {
		name           string
		metrics        bool
		pprof          bool
		path           string
		wantStatusCode int
	}{
		{name: "metrics enabled", metrics: true, path: "/metrics", wantStatusCode: http.StatusOK},
		{name: "metrics disabled", path: "/metrics", wantStatusCode: http.StatusNotFound},
		{name: "pprof enabled", pprof: true, path: "/debug/pprof/", wantStatusCode: http.StatusOK},
		{name: "pprof disabled", path: "/debug/pprof/", wantStatusCode: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			NewHandler(test.metrics, test.pprof).ServeHTTP(response, request)
			if response.Code != test.wantStatusCode {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatusCode)
			}
			if test.path == "/metrics" && test.metrics && !strings.Contains(response.Body.String(), "go_gc_duration_seconds") {
				t.Fatal("metrics response does not contain Go runtime metrics")
			}
		})
	}
}
