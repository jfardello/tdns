package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
)

type apiRequest struct {
	Method string
	Path   string
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func newMockAPI(t *testing.T, handler func(*http.Request) api.Response) <-chan apiRequest {
	t.Helper()

	requests := make(chan apiRequest, 16)
	restore := api.SetClientFactoryForTest(func() (*http.Client, error) {
		return &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				requests <- apiRequest{Method: r.Method, Path: r.URL.String()}
				resp := handler(r)
				body, err := json.Marshal(resp)
				if err != nil {
					t.Fatalf("json.Marshal error: %v", err)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Header:     make(http.Header),
					Body:       io.NopCloser(bytes.NewReader(body)),
				}, nil
			}),
		}, nil
	})
	t.Cleanup(restore)

	c := &config.Config{}
	c.Client.Server = "https://tdns.example"
	c.Client.CAcert = "../fixtures/tdns.crt"
	config.SetRunningConfig(c)
	return requests
}

func expectRequest(t *testing.T, requests <-chan apiRequest, wantMethod string, wantPath string) {
	t.Helper()
	select {
	case req := <-requests:
		if req.Method != wantMethod {
			t.Fatalf("request method got %s, want %s", req.Method, wantMethod)
		}
		if req.Path != wantPath {
			t.Fatalf("request path got %s, want %s", req.Path, wantPath)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("expected request %s %s", wantMethod, wantPath)
	}
}

func Test_handleStubs(t *testing.T) {
	requests := newMockAPI(t, func(r *http.Request) api.Response {
		return api.Response{
			Kind:          api.STUB_RESPONSE_KIND,
			Message:       api.MESSAGE_OK,
			CurrentStatus: "true",
		}
	})

	if err := handleStubs([]string{"google.es,udp://8.8.8.8", "google.com,udp://8.8.8.8"}); err != nil {
		t.Fatalf("handleStubs error: %v", err)
	}
	expectRequest(t, requests, http.MethodPost, "https://tdns.example/api/stubs?")
}

func Test_handleZenDomains(t *testing.T) {
	requests := newMockAPI(t, func(r *http.Request) api.Response {
		return api.Response{
			Kind:          api.ZEN_RESPONSE_KIND,
			Message:       api.MESSAGE_OK,
			CurrentStatus: "enabled",
		}
	})

	if err := handleZenDomains([]string{"example.com"}); err != nil {
		t.Fatalf("handleZenDomains error: %v", err)
	}
	expectRequest(t, requests, http.MethodPost, "https://tdns.example/api/zen?")
}
