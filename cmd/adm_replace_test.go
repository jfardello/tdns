package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jfardello/tdns/apiclient"
	"github.com/jfardello/tdns/config"
)

type apiRequest struct {
	Method string
	Path   string
}

func newMockAPI(t *testing.T, handler func(*http.Request) apiclient.Response) <-chan apiRequest {
	t.Helper()

	requests := make(chan apiRequest, 16)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests <- apiRequest{Method: r.Method, Path: r.URL.RequestURI()}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(handler(r)); err != nil {
			t.Errorf("encode mock response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	c := &config.Config{}
	c.Client.Server = server.URL
	c.Client.CAcert = "../fixtures/tdns.crt"
	c.Client.Token = "test-token"
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
	requests := newMockAPI(t, func(r *http.Request) apiclient.Response {
		return apiclient.Response{
			Kind:          apiclient.StubResolverResponseKind,
			Message:       apiclient.MESSAGE_OK,
			CurrentStatus: "true",
		}
	})

	if err := handleStubs([]string{"google.es,udp://8.8.8.8", "google.com,udp://8.8.8.8"}); err != nil {
		t.Fatalf("handleStubs error: %v", err)
	}
	expectRequest(t, requests, http.MethodPost, "/api/stub-resolver")
}

func Test_handleZenDomains(t *testing.T) {
	requests := newMockAPI(t, func(r *http.Request) apiclient.Response {
		return apiclient.Response{
			Kind:          apiclient.ZenModeResponseKind,
			Message:       apiclient.MESSAGE_OK,
			CurrentStatus: "enabled",
		}
	})

	if err := handleZenDomains([]string{"example.com"}); err != nil {
		t.Fatalf("handleZenDomains error: %v", err)
	}
	expectRequest(t, requests, http.MethodPost, "/api/zen-mode")
}
