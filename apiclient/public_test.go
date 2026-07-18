package apiclient_test

import (
	"net/http"
	"testing"

	"github.com/jfardello/tdns/apiclient"
)

func TestPublicClientCanBeConstructed(t *testing.T) {
	client, err := apiclient.New("https://tdns.example", "token", &http.Client{})
	if err != nil {
		t.Fatalf("construct public API client: %v", err)
	}
	if client == nil {
		t.Fatal("construct public API client returned nil")
	}
}
