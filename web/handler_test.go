package webui

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func newTestSPAHandler(t *testing.T) http.Handler {
	t.Helper()

	assetsFS := fstest.MapFS{
		"index.html":           {Data: []byte("<html>home</html>")},
		"200.html":             {Data: []byte("<html>fallback</html>")},
		"dashboard/index.html": {Data: []byte("<html>dashboard</html>")},
		"_nuxt/app.js":         {Data: []byte("console.log('ok')")},
	}
	return spaHandler(assetsFS, http.FileServer(http.FS(assetsFS)), "")
}

func TestSPAHandlerRootDoesNotRedirect(t *testing.T) {
	handler := newTestSPAHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d want %d", rec.Code, http.StatusOK)
	}
	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("unexpected redirect location %q", location)
	}
}

func TestSPAHandlerRouteIndexDoesNotRedirect(t *testing.T) {
	handler := newTestSPAHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d want %d", rec.Code, http.StatusOK)
	}
	if location := rec.Header().Get("Location"); location != "" {
		t.Fatalf("unexpected redirect location %q", location)
	}
}

func TestSPAHandlerUnknownRouteUsesFallback(t *testing.T) {
	handler := newTestSPAHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "<html>fallback</html>" {
		t.Fatalf("body got %q", body)
	}
}

func TestSPAHandlerStaticAssetStillServesDirectly(t *testing.T) {
	handler := newTestSPAHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/_nuxt/app.js", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status got %d want %d", rec.Code, http.StatusOK)
	}
	if body := rec.Body.String(); body != "console.log('ok')" {
		t.Fatalf("body got %q", body)
	}
}

var _ fs.FS = fstest.MapFS{}
