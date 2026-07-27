package webui

import (
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestContentSecurityPolicyAllowsGeneratedInlineScriptsByHash(t *testing.T) {
	const inlineScript = `window.__NUXT__={}`
	assetsFS := fstest.MapFS{
		"index.html": {
			Data: []byte(`<html><script src="/_nuxt/app.js"></script><script>` + inlineScript + `</script></html>`),
		},
	}

	policy, err := contentSecurityPolicy(assetsFS)
	if err != nil {
		t.Fatalf("contentSecurityPolicy: %v", err)
	}

	sum := sha256.Sum256([]byte(inlineScript))
	wantHash := "'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	if !strings.Contains(policy, "script-src 'self' "+wantHash) {
		t.Fatalf("policy does not allow generated inline script: %q", policy)
	}
	if strings.Contains(policy, "'unsafe-eval'") || strings.Contains(policy, "script-src 'self' 'unsafe-inline'") {
		t.Fatalf("policy permits unsafe script execution: %q", policy)
	}
}

func TestContentSecurityPolicyHeaderIsApplied(t *testing.T) {
	assetsFS := fstest.MapFS{
		"index.html": {Data: []byte("<html>home</html>")},
	}
	static := http.FileServer(http.FS(assetsFS))
	policy, err := contentSecurityPolicy(assetsFS)
	if err != nil {
		t.Fatalf("contentSecurityPolicy: %v", err)
	}
	handler := withContentSecurityPolicy(spaHandler(assetsFS, static, ""), policy)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := response.Header().Get("Content-Security-Policy"); got != policy {
		t.Fatalf("Content-Security-Policy = %q, want %q", got, policy)
	}
}
