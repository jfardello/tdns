package apiclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestClientUsesGeneratedPathQueryAndBearerAuth(t *testing.T) {
	request := make(chan *http.Request, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		request <- r.Clone(r.Context())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"api.tdns/dns-log/response","message":"Status OK"}`))
	}))
	defer server.Close()

	client, err := New(server.URL+"/management", "secret-token", server.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	since, status, name, mode := "24h", DNSLogTopStatus("blocked"), "office name", DNSLogTopClientMode("host")
	_, err = client.DNSLogTop(context.Background(), 7, &DNSLogTopParams{
		Since: &since, Status: &status, Client: &name, ClientMode: &mode,
	})
	if err != nil {
		t.Fatalf("DNSLogTop: %v", err)
	}

	got := <-request
	if got.URL.Path != "/management/api/dns-log/top/7" {
		t.Errorf("path = %q", got.URL.Path)
	}
	if got.URL.Query().Get("client") != name || got.URL.Query().Get("since") != since || got.URL.Query().Get("status") != string(status) {
		t.Errorf("query = %q", got.URL.RawQuery)
	}
	if got.Header.Get("Authorization") != "Bearer secret-token" {
		t.Errorf("Authorization = %q", got.Header.Get("Authorization"))
	}
}

func TestClientRejectsMalformedSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`not-json`))
	}))
	defer server.Close()

	client, err := New(server.URL, "", server.Client())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := client.ZenModeStart(context.Background()); err == nil {
		t.Fatal("ZenModeStart returned nil error for malformed JSON")
	}
}

func TestClientReturnsTypedHTTPError(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusInternalServerError} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(status)
				_, _ = w.Write([]byte("diagnostic body"))
			}))
			defer server.Close()

			client, err := New(server.URL, "", server.Client())
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			_, err = client.ZenModeStart(context.Background())
			var responseErr *HTTPError
			if !errors.As(err, &responseErr) {
				t.Fatalf("error = %T %v, want *HTTPError", err, err)
			}
			if responseErr.StatusCode != status || string(responseErr.Body) != "diagnostic body" {
				t.Errorf("HTTPError = %#v", responseErr)
			}
		})
	}
}

func TestNewHTTPClientLoadsConfiguredCA(t *testing.T) {
	client, err := NewHTTPClient("../fixtures/tdns.crt")
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil {
		t.Fatal("configured CA pool is missing from transport")
	}

	invalid := filepath.Join(t.TempDir(), "invalid.pem")
	if err := os.WriteFile(invalid, []byte("not a certificate"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if _, err := NewHTTPClient(invalid); err == nil {
		t.Fatal("NewHTTPClient accepted an invalid certificate")
	}
}
