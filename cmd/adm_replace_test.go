package cmd

import (
	"encoding/json"
	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func tConf() *httptest.Server {
	c := &config.Config{
		Server: config.Server{
			APIAddr: "127.0.0.1:90909",
		},
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := api.Response{
			Kind:          api.STUB_RESPONSE_KIND,
			Message:       api.MESSAGE_OK,
			CurrentStatus: "true",
		}
		encoded, _ := json.Marshal(resp)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(encoded)
	}))

	c.Client.Server = ts.URL
	c.Client.CAcert = "../fixtures/tdns.crt"
	config.SetRunningConfig(c)
	return ts
}

func Test_handleStubs(t *testing.T) {
	ts := tConf()
	defer ts.Close()
	type args struct {
		stubs []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
		{
			name:    "test",
			args:    args{stubs: []string{"google.es,udp://8.8.8.8", "google.com,udp://8.8.8.8"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := handleStubs(tt.args.stubs); (err != nil) != tt.wantErr {
				t.Errorf("handleStubs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
