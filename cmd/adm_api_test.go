package cmd

import (
	"context"
	"net/http"
	"testing"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/plugin"
	"github.com/spf13/cobra"
)

func TestManageCommandsHitExpectedAPIPaths(t *testing.T) {
	tests := []struct {
		name       string
		run        func(*cobra.Command)
		wantMethod string
		wantPath   string
		handler    func(*http.Request) api.Response
	}{
		{
			name: "start zen",
			run: func(cmd *cobra.Command) {
				startZenCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/zen/start?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.ZEN_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "start bhole",
			run: func(cmd *cobra.Command) {
				startBholeCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/bhole/start?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.BHOLE_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "start stubs",
			run: func(cmd *cobra.Command) {
				startStubsCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/stubs/start?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.STUB_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "start static",
			run: func(cmd *cobra.Command) {
				startStaticCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/static/start?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.BHOLE_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "stop stubs",
			run: func(cmd *cobra.Command) {
				stopStubsCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/stubs/stop?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.STUB_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "stop static",
			run: func(cmd *cobra.Command) {
				stoppStaticCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/static/stop?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.BHOLE_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "stop bhole",
			run: func(cmd *cobra.Command) {
				stopBholeCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/bhole/stop?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.BHOLE_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "dnslog alias",
			run: func(cmd *cobra.Command) {
				if err := handleAlias("office", "1.1.1.1"); err != nil {
					t.Fatalf("handleAlias error: %v", err)
				}
			},
			wantMethod: http.MethodPost,
			wantPath:   "https://tdns.example/api/dnslog/alias?",
			handler: func(r *http.Request) api.Response {
				return api.Response{Kind: api.DNSLOG_RESPONSE_KIND, Message: api.MESSAGE_OK}
			},
		},
		{
			name: "dnslog top",
			run: func(cmd *cobra.Command) {
				topLimit = 5
				since = "1w"
				if err := getTop(); err != nil {
					t.Fatalf("getTop error: %v", err)
				}
			},
			wantMethod: http.MethodGet,
			wantPath:   "https://tdns.example/api/dnslog/top/5?since=1w",
			handler: func(r *http.Request) api.Response {
				return api.Response{
					Kind:    api.DNSLOG_RESPONSE_KIND,
					Message: api.MESSAGE_OK,
					LogItems: []plugin.LogDetails{
						{Domain: "example.com.", Counter: 10, Host: "office"},
					},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requests := newMockAPI(t, tt.handler)

			cmd := &cobra.Command{}
			cmd.SetContext(context.Background())
			tt.run(cmd)

			expectRequest(t, requests, tt.wantMethod, tt.wantPath)
		})
	}
}
