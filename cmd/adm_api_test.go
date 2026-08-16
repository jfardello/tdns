package cmd

import (
	"context"
	"net/http"
	"testing"

	"github.com/jfardello/tdns/apiclient"
	"github.com/spf13/cobra"
)

func TestManageCommandsHitExpectedAPIPaths(t *testing.T) {
	tests := []struct {
		name       string
		run        func(*cobra.Command)
		wantMethod string
		wantPath   string
		handler    func(*http.Request) apiclient.Response
	}{
		{
			name: "start zen-mode",
			run: func(cmd *cobra.Command) {
				startZenCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/zen-mode/start",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.ZenModeResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "start blacklist",
			run: func(cmd *cobra.Command) {
				startBlacklistCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/blacklist/start",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.BlacklistResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "start stub-resolver",
			run: func(cmd *cobra.Command) {
				startStubsCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/stub-resolver/start",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.StubResolverResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "start static-response",
			run: func(cmd *cobra.Command) {
				startStaticCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/static-response/start",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.StaticResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "stop stub-resolver",
			run: func(cmd *cobra.Command) {
				stopStubsCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/stub-resolver/stop",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.StubResolverResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "stop static-response",
			run: func(cmd *cobra.Command) {
				stoppStaticCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/static-response/stop",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.StaticResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "stop blacklist",
			run: func(cmd *cobra.Command) {
				stopBlacklistCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/blacklist/stop",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.BlacklistResponseKind, Message: apiclient.MESSAGE_OK}
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
			wantPath:   "/api/dns-log/alias",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.DNSLogResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "dnslog status",
			run: func(cmd *cobra.Command) {
				dnsLogStatusCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodGet,
			wantPath:   "/api/dns-log",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.DNSLogResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "dnslog start",
			run: func(cmd *cobra.Command) {
				dnsLogStartCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/dns-log/start",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.DNSLogResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "dnslog stop",
			run: func(cmd *cobra.Command) {
				dnsLogStopCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodPost,
			wantPath:   "/api/dns-log/stop",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.DNSLogResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "dnslog clear",
			run: func(cmd *cobra.Command) {
				dnsLogClearCmd.Run(cmd, nil)
			},
			wantMethod: http.MethodDelete,
			wantPath:   "/api/dns-log",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{Kind: apiclient.DNSLogResponseKind, Message: apiclient.MESSAGE_OK}
			},
		},
		{
			name: "dnslog top",
			run: func(cmd *cobra.Command) {
				topLimit = 5
				since = "1w"
				topStatus = ""
				topClient = ""
				topClientMode = ""
				if err := getTop(); err != nil {
					t.Fatalf("getTop error: %v", err)
				}
			},
			wantMethod: http.MethodGet,
			wantPath:   "/api/dns-log/top/5?since=1w",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{
					Kind:    apiclient.DNSLogResponseKind,
					Message: apiclient.MESSAGE_OK,
					LogItems: []apiclient.LogDetails{
						{Domain: "example.com.", Counter: 10, Host: "office"},
					},
				}
			},
		},
		{
			name: "dnslog top filtered by status and client",
			run: func(cmd *cobra.Command) {
				topLimit = 5
				since = "24h"
				topStatus = "blocked"
				topClient = "office"
				topClientMode = "host"
				if err := getTop(); err != nil {
					t.Fatalf("getTop error: %v", err)
				}
			},
			wantMethod: http.MethodGet,
			wantPath:   "/api/dns-log/top/5?since=24h&status=blocked&client=office&client_mode=host",
			handler: func(r *http.Request) apiclient.Response {
				return apiclient.Response{
					Kind:    apiclient.DNSLogResponseKind,
					Message: apiclient.MESSAGE_OK,
					LogItems: []apiclient.LogDetails{
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
