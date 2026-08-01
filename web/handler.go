package webui

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	DefaultIndexFile    = "index.html"
	DefaultFallbackFile = "200.html"
)

var inlineScriptPattern = regexp.MustCompile(`(?is)<script(?:\s[^>]*)?>(.*?)</script>`)

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
	policy, err := contentSecurityPolicy(assetsFS)
	if err != nil {
		return nil, err
	}

	return &Handlers{
		FS:     assetsFS,
		Static: static,
		SPA:    withContentSecurityPolicy(spaHandler(assetsFS, static, fallback), policy),
	}, nil
}

func contentSecurityPolicy(assetsFS fs.FS) (string, error) {
	hashes := make(map[string]struct{})
	err := fs.WalkDir(assetsFS, ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".html") {
			return nil
		}

		content, err := fs.ReadFile(assetsFS, name)
		if err != nil {
			return err
		}
		for _, match := range inlineScriptPattern.FindAllSubmatch(content, -1) {
			if len(match[1]) == 0 {
				continue
			}
			sum := sha256.Sum256(match[1])
			hashes["'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'"] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	scriptSources := []string{"'self'"}
	for hash := range hashes {
		scriptSources = append(scriptSources, hash)
	}
	sort.Strings(scriptSources[1:])

	return strings.Join([]string{
		"default-src 'none'",
		"base-uri 'none'",
		"connect-src 'self'",
		"font-src 'self' data:",
		"form-action 'self'",
		"frame-ancestors 'none'",
		"img-src 'self' data:",
		"manifest-src 'self'",
		"object-src 'none'",
		"script-src " + strings.Join(scriptSources, " "),
		"style-src 'self' 'unsafe-inline'",
		"style-src-elem 'self' 'unsafe-inline'",
		"style-src-attr 'unsafe-inline'",
	}, "; "), nil
}

func withContentSecurityPolicy(next http.Handler, policy string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", policy)
		next.ServeHTTP(w, r)
	})
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
