package blocklist

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const userAgent = "tdns-blocklist"

var errResponseTooLarge = errors.New("response exceeds limit")

type Kind string

const (
	KindInvalid          Kind = "invalid"
	KindTimeout          Kind = "timeout"
	KindTooLarge         Kind = "too_large"
	KindRedirectRejected Kind = "redirect_rejected"
	KindRemote           Kind = "remote_error"
	KindIO               Kind = "io_error"
)

type Error struct {
	Kind  Kind
	Stage string
	Err   error
}

func (e *Error) Error() string {
	return fmt.Sprintf("blocklist %s failed: %v", e.Stage, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

func newError(kind Kind, stage string, err error) error {
	return &Error{Kind: kind, Stage: stage, Err: err}
}

func KindOf(err error) Kind {
	var ingestErr *Error
	if errors.As(err, &ingestErr) {
		return ingestErr.Kind
	}
	return KindRemote
}

type Limits struct {
	MetadataBytes     int64
	CompressedBytes   int64
	UncompressedBytes int64
	MaxLineBytes      int
	MaxEntries        int
	MaxRedirects      int
	TotalTimeout      time.Duration
}

func DefaultLimits() Limits {
	return Limits{
		MetadataBytes:     1 << 20,
		CompressedBytes:   64 << 20,
		UncompressedBytes: 128 << 20,
		MaxLineBytes:      64 << 10,
		MaxEntries:        2_000_000,
		MaxRedirects:      3,
		TotalTimeout:      2 * time.Minute,
	}
}

type Result struct {
	Changed           bool
	Revision          string
	CompressedBytes   int64
	UncompressedBytes int64
	Entries           int
}

type Validator func(context.Context, string, Limits) (int, error)

type Client struct {
	httpClient *http.Client
	apiBase    *url.URL
	token      string
	limits     Limits
}

func NewClient(token string) *Client {
	limits := DefaultLimits()
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		DisableCompression:    true,
	}
	apiBase := &url.URL{Scheme: "https", Host: "api.github.com"}
	return &Client{
		httpClient: &http.Client{
			Transport:     transport,
			Timeout:       limits.TotalTimeout,
			CheckRedirect: redirectPolicy(limits.MaxRedirects, false),
		},
		apiBase: apiBase,
		token:   strings.TrimSpace(token),
		limits:  limits,
	}
}

func newTestClient(httpClient *http.Client, apiBase *url.URL, limits Limits) *Client {
	return &Client{httpClient: httpClient, apiBase: apiBase, limits: limits}
}

func redirectPolicy(maxRedirects int, allowHTTP bool) func(*http.Request, []*http.Request) error {
	allowedHosts := map[string]struct{}{
		"api.github.com":                {},
		"raw.githubusercontent.com":     {},
		"objects.githubusercontent.com": {},
	}
	return func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirects {
			return newError(KindRedirectRejected, "redirect", fmt.Errorf("too many redirects"))
		}
		if req.URL.User != nil || (!allowHTTP && req.URL.Scheme != "https") {
			return newError(KindRedirectRejected, "redirect", fmt.Errorf("redirect target is not acceptable"))
		}
		if !allowHTTP {
			if _, ok := allowedHosts[strings.ToLower(req.URL.Hostname())]; !ok {
				return newError(KindRedirectRejected, "redirect", fmt.Errorf("redirect host is not allowed"))
			}
		}
		if len(via) > 0 && !strings.EqualFold(req.URL.Hostname(), via[0].URL.Hostname()) {
			req.Header.Del("Authorization")
		}
		return nil
	}
}

func (c *Client) Refresh(ctx context.Context, source Source, destination, previousRevision string, validate Validator) (Result, error) {
	ctx, cancel := context.WithTimeout(ctx, c.limits.TotalTimeout)
	defer cancel()

	revision, err := c.resolveRevision(ctx, source)
	if err != nil {
		return Result{}, err
	}
	if revision == previousRevision {
		return Result{Revision: revision}, nil
	}

	dir := filepath.Dir(destination)
	base := filepath.Base(destination)
	if base == "." || base == string(filepath.Separator) {
		return Result{}, newError(KindIO, "temporary file", fmt.Errorf("invalid destination path"))
	}
	temporary, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return Result{}, newError(KindIO, "temporary file", err)
	}
	temporaryPath := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return Result{}, newError(KindIO, "temporary file", err)
	}

	compressed, uncompressed, err := c.download(ctx, source, revision, temporary)
	if err != nil {
		return Result{}, err
	}
	if err := temporary.Sync(); err != nil {
		return Result{}, newError(KindIO, "sync candidate", err)
	}
	if err := temporary.Close(); err != nil {
		return Result{}, newError(KindIO, "close candidate", err)
	}

	entries, err := validate(ctx, temporaryPath, c.limits)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return Result{}, newError(KindTimeout, "validate candidate", fmt.Errorf("validation deadline exceeded"))
		}
		return Result{}, newError(KindInvalid, "validate candidate", err)
	}
	if err := os.Rename(temporaryPath, destination); err != nil {
		return Result{}, newError(KindIO, "install candidate", err)
	}
	committed = true
	return Result{
		Changed:           true,
		Revision:          revision,
		CompressedBytes:   compressed,
		UncompressedBytes: uncompressed,
		Entries:           entries,
	}, nil
}

func (c *Client) resolveRevision(ctx context.Context, source Source) (string, error) {
	u := c.endpoint("repos", source.Owner, source.Repo, "git", "ref", "heads", source.Branch)
	resp, err := c.do(ctx, u, "application/vnd.github+json", false)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", newError(KindRemote, "resolve revision", fmt.Errorf("GitHub API returned status %d", resp.StatusCode))
	}
	body, err := readLimited(resp.Body, c.limits.MetadataBytes)
	if err != nil {
		kind := KindRemote
		if errors.Is(err, errResponseTooLarge) {
			kind = KindTooLarge
		} else if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			kind = KindTimeout
		}
		return "", newError(kind, "resolve revision", fmt.Errorf("GitHub API metadata could not be read"))
	}
	var payload struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", newError(KindRemote, "resolve revision", fmt.Errorf("invalid GitHub API response"))
	}
	if !validRevision(payload.Object.SHA) {
		return "", newError(KindRemote, "resolve revision", fmt.Errorf("GitHub API returned an invalid revision"))
	}
	return strings.ToLower(payload.Object.SHA), nil
}

func (c *Client) download(ctx context.Context, source Source, revision string, destination io.Writer) (int64, int64, error) {
	parts := []string{"repos", source.Owner, source.Repo, "contents"}
	parts = append(parts, strings.Split(source.File, "/")...)
	u := c.endpoint(parts...)
	query := u.Query()
	query.Set("ref", revision)
	u.RawQuery = query.Encode()
	resp, err := c.do(ctx, u, "application/vnd.github.raw+json", true)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, 0, newError(KindRemote, "download", fmt.Errorf("GitHub API returned status %d", resp.StatusCode))
	}
	if resp.ContentLength > c.limits.CompressedBytes {
		return 0, 0, newError(KindTooLarge, "download", fmt.Errorf("compressed response exceeds limit"))
	}

	wire := &countingReader{reader: io.LimitReader(resp.Body, c.limits.CompressedBytes+1)}
	var content io.Reader = wire
	encoding := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Encoding")))
	if encoding == "gzip" {
		gzipReader, err := gzip.NewReader(wire)
		if err != nil {
			return wire.total, 0, newError(KindInvalid, "decompress", fmt.Errorf("invalid gzip response"))
		}
		defer gzipReader.Close()
		content = gzipReader
	} else if encoding != "" && encoding != "identity" {
		return 0, 0, newError(KindInvalid, "decompress", fmt.Errorf("unsupported content encoding"))
	} else if resp.ContentLength > c.limits.UncompressedBytes {
		return 0, 0, newError(KindTooLarge, "download", fmt.Errorf("uncompressed response exceeds limit"))
	}

	uncompressed, copyErr := io.Copy(destination, io.LimitReader(content, c.limits.UncompressedBytes+1))
	if uncompressed > c.limits.UncompressedBytes || wire.total > c.limits.CompressedBytes {
		return wire.total, uncompressed, newError(KindTooLarge, "download", fmt.Errorf("response exceeds configured size limit"))
	}
	if copyErr != nil {
		kind := KindRemote
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			kind = KindTimeout
		} else if encoding == "gzip" {
			kind = KindInvalid
		}
		return wire.total, uncompressed, newError(kind, "download", fmt.Errorf("response stream failed"))
	}
	return wire.total, uncompressed, nil
}

func (c *Client) do(ctx context.Context, endpoint *url.URL, accept string, compressed bool) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, newError(KindRemote, "request", fmt.Errorf("could not create request"))
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", userAgent)
	if compressed {
		req.Header.Set("Accept-Encoding", "gzip")
	} else {
		req.Header.Set("Accept-Encoding", "identity")
	}
	if c.token != "" && strings.EqualFold(endpoint.Hostname(), "api.github.com") {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		kind := KindRemote
		var ingestErr *Error
		if errors.As(err, &ingestErr) {
			kind = ingestErr.Kind
		} else if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			kind = KindTimeout
		}
		return nil, newError(kind, "request", fmt.Errorf("remote request failed"))
	}
	return resp, nil
}

func (c *Client) endpoint(parts ...string) *url.URL {
	u := *c.apiBase
	base := strings.TrimSuffix(u.Path, "/")
	escaped := make([]string, len(parts))
	for i, part := range parts {
		escaped[i] = url.PathEscape(part)
	}
	u.Path = base + "/" + strings.Join(parts, "/")
	u.RawPath = base + "/" + strings.Join(escaped, "/")
	return &u
}

func validRevision(revision string) bool {
	if len(revision) != 40 && len(revision) != 64 {
		return false
	}
	_, err := strconv.ParseUint(revision[:16], 16, 64)
	if err != nil {
		return false
	}
	for _, r := range revision[16:] {
		if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
			return false
		}
	}
	return true
}

func readLimited(reader io.Reader, limit int64) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errResponseTooLarge
	}
	return body, nil
}

type countingReader struct {
	reader io.Reader
	total  int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.total += int64(n)
	return n, err
}
