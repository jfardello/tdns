package blocklist

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const testRevision = "0123456789abcdef0123456789abcdef01234567"

func TestRefreshValidatesThenAtomicallyReplaces(t *testing.T) {
	var contentRequests atomic.Int32
	server := newGitHubServer(t, func(w http.ResponseWriter, r *http.Request) {
		contentRequests.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "0.0.0.0 ads.example\n")
	})
	defer server.Close()

	destination := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(destination, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	client := testHTTPClient(t, server.URL, DefaultLimits())
	result, err := client.Refresh(context.Background(), testSource(), destination, "", func(_ context.Context, path string, _ Limits) (int, error) {
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return 0, readErr
		}
		if string(body) != "0.0.0.0 ads.example\n" {
			return 0, fmt.Errorf("unexpected candidate")
		}
		current, readErr := os.ReadFile(destination)
		if readErr != nil || string(current) != "old\n" {
			return 0, fmt.Errorf("active file changed before validation")
		}
		return 1, nil
	})
	if err != nil {
		t.Fatalf("Refresh error: %v", err)
	}
	if !result.Changed || result.Revision != testRevision || result.Entries != 1 || contentRequests.Load() != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	body, err := os.ReadFile(destination)
	if err != nil || string(body) != "0.0.0.0 ads.example\n" {
		t.Fatalf("active content = %q, error %v", body, err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("active mode = %o, want 600", info.Mode().Perm())
	}
}

func TestRefreshLeavesActiveFileOnValidationFailure(t *testing.T) {
	server := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "not a hosts file\n")
	})
	defer server.Close()
	dir := t.TempDir()
	destination := filepath.Join(dir, "hosts")
	if err := os.WriteFile(destination, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := testHTTPClient(t, server.URL, DefaultLimits())
	_, err := client.Refresh(context.Background(), testSource(), destination, "", func(context.Context, string, Limits) (int, error) {
		return 0, fmt.Errorf("malformed candidate")
	})
	if err == nil || KindOf(err) != KindInvalid {
		t.Fatalf("Refresh error = %v, kind %q", err, KindOf(err))
	}
	assertFileContent(t, destination, "old\n")
	temporary, err := filepath.Glob(filepath.Join(dir, ".hosts.tmp-*"))
	if err != nil || len(temporary) != 0 {
		t.Fatalf("temporary files after failure: %v, error %v", temporary, err)
	}
}

func TestRefreshBoundsCompressedAndUncompressedBodies(t *testing.T) {
	tests := []struct {
		name     string
		limits   Limits
		response func(http.ResponseWriter)
	}{
		{
			name:   "compressed body",
			limits: limitsWithSizes(32, 128),
			response: func(w http.ResponseWriter) {
				_, _ = w.Write(bytes.Repeat([]byte("x"), 64))
			},
		},
		{
			name:   "gzip expansion",
			limits: limitsWithSizes(1024, 64),
			response: func(w http.ResponseWriter) {
				w.Header().Set("Content-Encoding", "gzip")
				writer := gzip.NewWriter(w)
				_, _ = writer.Write(bytes.Repeat([]byte("x"), 256))
				_ = writer.Close()
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) { test.response(w) })
			defer server.Close()
			destination := filepath.Join(t.TempDir(), "hosts")
			if err := os.WriteFile(destination, []byte("old\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			client := testHTTPClient(t, server.URL, test.limits)
			_, err := client.Refresh(context.Background(), testSource(), destination, "", acceptCandidate)
			if err == nil || KindOf(err) != KindTooLarge {
				t.Fatalf("Refresh error = %v, kind %q", err, KindOf(err))
			}
			assertFileContent(t, destination, "old\n")
		})
	}
}

func TestRefreshBoundsMetadata(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 65))
	}))
	defer server.Close()
	limits := limitsWithSizes(1024, 1024)
	limits.MetadataBytes = 64
	client := testHTTPClient(t, server.URL, limits)
	destination := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(destination, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := client.Refresh(context.Background(), testSource(), destination, "", acceptCandidate)
	if err == nil || KindOf(err) != KindTooLarge {
		t.Fatalf("Refresh error = %v, kind %q", err, KindOf(err))
	}
	assertFileContent(t, destination, "old\n")
}

func TestRefreshTimesOutAndPreservesActiveFile(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/git/ref/") {
			time.Sleep(100 * time.Millisecond)
		}
		_, _ = fmt.Fprintf(w, `{"object":{"sha":%q}}`, testRevision)
	}))
	defer server.Close()
	limits := DefaultLimits()
	limits.TotalTimeout = 20 * time.Millisecond
	client := testHTTPClient(t, server.URL, limits)
	destination := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(destination, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := client.Refresh(context.Background(), testSource(), destination, "", acceptCandidate)
	if err == nil || KindOf(err) != KindTimeout {
		t.Fatalf("Refresh error = %v, kind %q", err, KindOf(err))
	}
	assertFileContent(t, destination, "old\n")
}

func TestRefreshRejectsInterruptedBody(t *testing.T) {
	server := newGitHubServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		_, _ = io.WriteString(w, "partial")
	})
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(destination, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	client := testHTTPClient(t, server.URL, DefaultLimits())
	if _, err := client.Refresh(context.Background(), testSource(), destination, "", acceptCandidate); err == nil {
		t.Fatal("expected interrupted body error")
	}
	assertFileContent(t, destination, "old\n")
}

func TestRefreshSkipsContentForUnchangedRevision(t *testing.T) {
	server := newGitHubServer(t, func(http.ResponseWriter, *http.Request) {
		t.Fatal("content endpoint must not be called")
	})
	defer server.Close()
	client := testHTTPClient(t, server.URL, DefaultLimits())
	result, err := client.Refresh(context.Background(), testSource(), filepath.Join(t.TempDir(), "hosts"), testRevision, acceptCandidate)
	if err != nil || result.Changed || result.Revision != testRevision {
		t.Fatalf("result = %#v, error %v", result, err)
	}
}

func TestRedirectPolicyRejectsUnsafeTargetsAndStripsCredentials(t *testing.T) {
	policy := redirectPolicy(3, false)
	previous := httptest.NewRequest(http.MethodGet, "https://api.github.com/start", nil)

	for _, target := range []string{"http://api.github.com/next", "https://example.com/next"} {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		if err := policy(req, []*http.Request{previous}); err == nil || KindOf(err) != KindRedirectRejected {
			t.Fatalf("target %q error = %v", target, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "https://raw.githubusercontent.com/acme/lists/ref/hosts", nil)
	req.Header.Set("Authorization", "Bearer secret")
	if err := policy(req, []*http.Request{previous}); err != nil {
		t.Fatalf("allowed redirect error: %v", err)
	}
	if req.Header.Get("Authorization") != "" {
		t.Fatal("authorization header retained across redirect host")
	}
}

func TestRemoteFailureDoesNotIncludeResponseBody(t *testing.T) {
	server := newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, "secret response body")
	}))
	defer server.Close()
	client := testHTTPClient(t, server.URL, DefaultLimits())
	_, err := client.Refresh(context.Background(), testSource(), filepath.Join(t.TempDir(), "hosts"), "", acceptCandidate)
	if err == nil {
		t.Fatal("expected remote error")
	}
	if strings.Contains(err.Error(), "secret response body") {
		t.Fatalf("error leaked response body: %v", err)
	}
}

func TestRevisionStateIsRestrictedAndAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts.state")
	if err := WriteRevision(path, testRevision); err != nil {
		t.Fatalf("WriteRevision error: %v", err)
	}
	if got := ReadRevision(path); got != testRevision {
		t.Fatalf("ReadRevision = %q", got)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("state mode = %o, want 600", info.Mode().Perm())
	}
}

func newGitHubServer(t *testing.T, content http.HandlerFunc) *httptest.Server {
	t.Helper()
	return newTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" || r.Header.Get("User-Agent") != userAgent {
			t.Errorf("unexpected GitHub request headers: %#v", r.Header)
		}
		if strings.Contains(r.URL.Path, "/git/ref/heads/") {
			if r.Header.Get("Accept") != "application/vnd.github+json" {
				t.Errorf("revision Accept = %q", r.Header.Get("Accept"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"object":{"sha":%q}}`, testRevision)
			return
		}
		if strings.Contains(r.URL.Path, "/contents/") {
			if r.Header.Get("Accept") != "application/vnd.github.raw+json" {
				t.Errorf("content Accept = %q", r.Header.Get("Accept"))
			}
			content(w, r)
			return
		}
		http.NotFound(w, r)
	}))
}

func newTestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	return server
}

func testHTTPClient(t *testing.T, serverURL string, limits Limits) *Client {
	t.Helper()
	base, err := url.Parse(serverURL)
	if err != nil {
		t.Fatal(err)
	}
	httpClient := &http.Client{
		Timeout:       limits.TotalTimeout,
		CheckRedirect: redirectPolicy(limits.MaxRedirects, true),
	}
	return newTestClient(httpClient, base, limits)
}

func limitsWithSizes(compressed, uncompressed int64) Limits {
	limits := DefaultLimits()
	limits.CompressedBytes = compressed
	limits.UncompressedBytes = uncompressed
	return limits
}

func testSource() Source {
	return Source{Owner: "acme", Repo: "lists", Branch: "main", File: "hosts"}
}

func acceptCandidate(context.Context, string, Limits) (int, error) { return 1, nil }

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil || string(body) != expected {
		t.Fatalf("file content = %q, error %v; want %q", body, err, expected)
	}
}
