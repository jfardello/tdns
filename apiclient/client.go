package apiclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/apiclient/generated"
	"github.com/jfardello/tdns/config"
)

const (
	MessageOK                = api.MessageOK
	MESSAGE_OK               = MessageOK
	StubResolverResponseKind = api.StubResolverResponseKind
	ZenModeResponseKind      = api.ZenModeResponseKind
	BlacklistResponseKind    = api.BlacklistResponseKind
	StaticResponseKind       = api.StaticResponseKind
	DNSLogResponseKind       = api.DNSLogResponseKind
	TaggerResponseKind       = api.TaggerResponseKind
	CacheResponseKind        = api.CacheResponseKind
)

type Response = api.Response
type LogDetails = api.LogDetails
type StubReplaceRequest = api.StubReplaceRequest
type ZenReplaceRequest = api.ZenReplaceRequest
type DNSLogAliasRequest = api.DNSLogAliasRequest
type DNSLogTopParams = generated.DnsLogTopParams
type DNSLogTopStatus = generated.DnsLogTopParamsStatus
type DNSLogTopClientMode = generated.DnsLogTopParamsClientMode

type HTTPError struct {
	StatusCode int
	Body       []byte
}

func (e *HTTPError) Error() string {
	body := strings.TrimSpace(string(e.Body))
	if body == "" {
		return fmt.Sprintf("management API returned HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("management API returned HTTP %d: %s", e.StatusCode, body)
}

type Client struct {
	generated *generated.Client
}

func New(baseURL, token string, httpClient *http.Client) (*Client, error) {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	generatedClient, err := generated.NewClient(
		baseURL,
		generated.WithHTTPClient(httpClient),
		generated.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			return nil
		}),
	)
	if err != nil {
		return nil, err
	}
	return &Client{generated: generatedClient}, nil
}

func NewFromConfig(conf config.Client) (*Client, error) {
	httpClient, err := NewHTTPClient(conf.CAcert)
	if err != nil {
		return nil, err
	}
	return New(conf.Server, conf.Token, httpClient)
}

func NewHTTPClient(caCertFile string) (*http.Client, error) {
	caCert, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil, errors.New("unable to parse client CA certificate")
	}

	return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{
		RootCAs: caCertPool,
	}}}, nil
}

func (c *Client) ZenModeStart(ctx context.Context) (*Response, error) {
	return decode(c.generated.ZenModeStart(ctx))
}

func (c *Client) BlacklistToggle(ctx context.Context, action string) (*Response, error) {
	return decode(c.generated.BlacklistToggle(ctx, generated.BlacklistToggleParamsAction(action)))
}

func (c *Client) StubResolverToggle(ctx context.Context, action string) (*Response, error) {
	return decode(c.generated.StubResolverToggle(ctx, generated.StubResolverToggleParamsAction(action)))
}

func (c *Client) StaticResponseToggle(ctx context.Context, action string) (*Response, error) {
	return decode(c.generated.StaticResponseToggle(ctx, generated.StaticResponseToggleParamsAction(action)))
}

func (c *Client) StubResolverReplace(ctx context.Context, request StubReplaceRequest) (*Response, error) {
	body := generated.ApiStubReplaceRequest{Stubs: &request.Stubs}
	return decode(c.generated.StubResolverReplace(ctx, body))
}

func (c *Client) ZenModeDomainsReplace(ctx context.Context, request ZenReplaceRequest) (*Response, error) {
	body := generated.ApiZenReplaceRequest{ZenDomains: &request.ZenDomains}
	return decode(c.generated.ZenModeDomainsReplace(ctx, body))
}

func (c *Client) DNSLogAliasSet(ctx context.Context, request DNSLogAliasRequest) (*Response, error) {
	body := generated.ApiDNSLogAliasRequest{Name: &request.Name, Addr: &request.Addr}
	return decode(c.generated.DnsLogAliasSet(ctx, body))
}

func (c *Client) DNSLogTop(ctx context.Context, top int, params *DNSLogTopParams) (*Response, error) {
	return decode(c.generated.DnsLogTop(ctx, top, params))
}

func decode(response *http.Response, requestErr error) (*Response, error) {
	if requestErr != nil {
		return nil, requestErr
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, &HTTPError{StatusCode: response.StatusCode, Body: body}
	}

	result := &Response{}
	if err := json.Unmarshal(body, result); err != nil {
		return nil, err
	}
	return result, nil
}
