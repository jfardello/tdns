package webui

import (
	"bytes"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	DefaultIndexFile    = "index.html"
	DefaultFallbackFile = "200.html"
)

type Handlers struct {
	FS     fs.FS
	Static http.Handler
	SPA    http.Handler
}

// NewHandlers returns the embedded asset filesystem, a direct static file
// handler, and a SPA-aware handler that falls back to 200.html or index.html.
func NewHandlers(fallback string) (*Handlers, error) {
	assetsFS, err := AssetsFS()
	if err != nil {
		return nil, err
	}

	static := http.FileServer(http.FS(assetsFS))

	return &Handlers{
		FS:     assetsFS,
		Static: static,
		SPA:    spaHandler(assetsFS, static, fallback),
	}, nil
}

func spaHandler(assetsFS fs.FS, static http.Handler, fallback string) http.Handler {
	fallbackPath := strings.TrimPrefix(fallback, "/")
	if fallbackPath == "" {
		if exists(assetsFS, DefaultFallbackFile) {
			fallbackPath = DefaultFallbackFile
		} else {
			fallbackPath = DefaultIndexFile
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if assetPath := resolveAssetPath(assetsFS, r.URL.Path); assetPath != "" {
			serveAsset(assetsFS, static, w, r, assetPath)
			return
		}

		serveEmbeddedFile(assetsFS, w, r, fallbackPath)
	})
}

func serveAsset(assetsFS fs.FS, static http.Handler, w http.ResponseWriter, r *http.Request, assetPath string) {
	cleanPath := strings.TrimPrefix(assetPath, "/")
	if cleanPath == DefaultIndexFile || cleanPath == DefaultFallbackFile || strings.HasSuffix(cleanPath, "/"+DefaultIndexFile) {
		serveEmbeddedFile(assetsFS, w, r, cleanPath)
		return
	}

	servePath(static, w, r, cleanPath)
}

func servePath(static http.Handler, w http.ResponseWriter, r *http.Request, assetPath string) {
	cloned := r.Clone(r.Context())
	cloned.URL = cloneURL(r.URL)
	cloned.URL.Path = "/" + strings.TrimPrefix(assetPath, "/")
	static.ServeHTTP(w, cloned)
}

func serveEmbeddedFile(assetsFS fs.FS, w http.ResponseWriter, r *http.Request, assetPath string) {
	content, err := fs.ReadFile(assetsFS, strings.TrimPrefix(assetPath, "/"))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	http.ServeContent(w, r, path.Base(assetPath), time.Time{}, bytes.NewReader(content))
}

func cloneURL(parsedURL *url.URL) *url.URL {
	if parsedURL == nil {
		return &url.URL{}
	}

	cloned := *parsedURL
	return &cloned
}

func resolveAssetPath(assetsFS fs.FS, requestPath string) string {
	cleaned := path.Clean("/" + requestPath)
	relative := strings.TrimPrefix(cleaned, "/")
	if relative == "" || relative == "." {
		relative = DefaultIndexFile
	}

	candidates := []string{relative}
	if path.Ext(relative) == "" {
		candidates = append(candidates, path.Join(relative, DefaultIndexFile))
	}

	for _, candidate := range candidates {
		if exists(assetsFS, candidate) {
			return candidate
		}
	}

	return ""
}

func exists(files fs.FS, name string) bool {
	info, err := fs.Stat(files, name)
	return err == nil && !info.IsDir()
}
