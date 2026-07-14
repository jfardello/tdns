package apiclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jfardello/tdns/api"
	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
)

var httpClientFactory = newClient

const (
	GET  = "GET"
	POST = "POST"

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
type ZenExcludesRequest = api.ZenExcludesRequest
type BlacklistWhitelistRequest = api.BlacklistWhitelistRequest
type BlacklistHostsRequest = api.BlacklistHostsRequest
type BlacklistExcludesRequest = api.BlacklistExcludesRequest
type StaticReplaceRequest = api.StaticReplaceRequest
type CacheExcludeRequest = api.CacheExcludeRequest
type DNSLogAliasRequest = api.DNSLogAliasRequest

func Get(ctx context.Context, url string) (*Response, error) {
	return Do(ctx, url, GET, nil)
}

func Post(ctx context.Context, url string, data any) (*Response, error) {
	return Do(ctx, url, POST, data)
}

func Do(ctx context.Context, urlPath string, method string, data any) (*Response, error) {
	logger := log.GetLogger("api", "Do")
	conf := config.GetRunningConfig()

	splitted := strings.Split(urlPath, "?")
	urlPath = splitted[0]
	qs := ""
	if len(splitted) > 1 {
		qs = splitted[1]
	}

	addr := conf.Client.Server
	fullUrl, err := url.JoinPath(addr, urlPath)
	fullUrl = fmt.Sprintf("%s?%s", fullUrl, qs)
	logger.Infof("fullUrl: %s", fullUrl)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	var byteReader *bytes.Reader

	if data != nil {
		b, err := toJSON(data)
		if err != nil {
			logger.Errorf("Error creating payload %s", err.Error())
			return nil, err
		}
		byteReader = bytes.NewReader(b)

	} else {
		byteReader = bytes.NewReader([]byte{})
	}

	r, err := http.NewRequestWithContext(ctx, method, fullUrl, byteReader)

	if err != nil {
		logger.Errorf("Error creating request %s", err.Error())
		return nil, err
	}

	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", conf.Client.Token))
	r.Header.Add("Content-Type", "application/json")

	client, err := httpClientFactory()
	if err != nil {
		logger.Errorf("Error creating http client %s", err.Error())
		return nil, err
	}
	res, err := client.Do(r)

	if err != nil {
		logger.Errorf("Error making http request %s", err.Error())
		return nil, err
	}

	body, err := io.ReadAll(res.Body)
	res.Body.Close()

	if err != nil {
		return nil, err
	}

	if res.StatusCode > http.StatusCreated {
		return &Response{Message: string(body)}, nil
	}

	return parseJSON(body)
}

func parseJSON(s []byte) (*Response, error) {

	resp := &Response{}
	if err := json.Unmarshal(s, resp); err != nil {
		return resp, err
	}

	return resp, nil
}

func toJSON(T any) ([]byte, error) {
	return json.Marshal(T)
}

func newClient() (*http.Client, error) {
	c := config.GetRunningConfig()

	caCert, err := os.ReadFile(c.Client.CAcert)
	if err != nil {
		return nil, err
	}
	caCertPool := x509.NewCertPool()
	if ok := caCertPool.AppendCertsFromPEM(caCert); !ok {
		return nil, errors.New("unable to parse client CA certificate")
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	return client, nil
}

func SetClientFactoryForTest(factory func() (*http.Client, error)) func() {
	prev := httpClientFactory
	httpClientFactory = factory
	return func() {
		httpClientFactory = prev
	}
}
