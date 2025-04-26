package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/jfardello/tdns/plugin"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"
)

const (
	GET                  = "GET"
	POST                 = "POST"
	STUB_RESPONSE_KIND   = "api.tdns/stub/response"
	ZEN_RESPONSE_KIND    = "api.tdns/zen/response"
	BHOLE_RESPONSE_KIND  = "api.tdns/bhole/response"
	DNSLOG_RESPONSE_KIND = "api.tdns/dnslog/response"
)

type Response struct {
	Kind          string              `json:"kind"`
	Message       string              `json:"message"`
	CurrentStatus string              `json:"current_status"`
	Items         []string            `json:"items,omitempty"`
	LogItems      []plugin.LogDetails `json:"log_items,omitempty"`
}

type StubReplaceRequest struct {
	Stubs []string `json:"stubs"`
}

type ZenReplaceRequest struct {
	ZenDomains []string `json:"zen_domains"`
}
type DNSLogAliasRequest struct {
	Name string `json:"name"`
	Addr string `json:"addr"`
}

func Get(ctx context.Context, url string) (*Response, error) {
	return Do(ctx, url, GET, nil)
}

func Post(ctx context.Context, url string, data any) (*Response, error) {
	return Do(ctx, url, POST, data)
}

func Do(ctx context.Context, urlPath string, method string, data any) (*Response, error) {
	logger := log.GetLogger("api", "post")
	conf := config.GetRunningConfig()

	addr := conf.Client.Server
	fullUrl, err := url.JoinPath(addr, urlPath)
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

	r.Header.Add("Authorization", fmt.Sprintf("Bearer: %s", conf.Client.Token))
	r.Header.Add("Content-Type", "application/json")

	//res, err := http.DefaultClient.Do(r)
	client := newClient()
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

func newClient() *http.Client {
	logger := log.GetLogger("api", "client")
	c := config.GetRunningConfig()

	caCert, err := os.ReadFile(c.Client.CAcert)
	if err != nil {
		logger.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	return client
}
