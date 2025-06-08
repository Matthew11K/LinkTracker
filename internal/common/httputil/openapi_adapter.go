package httputil

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"

	"github.com/central-university-dev/go-Matthew11K/internal/config"
	"github.com/go-resty/resty/v2"
)

type OpenAPIHTTPAdapter struct {
	restyClient *resty.Client
}

func NewOpenAPIHTTPAdapter(restyClient *resty.Client) *OpenAPIHTTPAdapter {
	return &OpenAPIHTTPAdapter{
		restyClient: restyClient,
	}
}

func (a *OpenAPIHTTPAdapter) Do(req *http.Request) (*http.Response, error) {
	restyReq := a.restyClient.R()

	for key, values := range req.Header {
		for _, value := range values {
			restyReq.SetHeader(key, value)
		}
	}

	if req.Context() != nil {
		restyReq.SetContext(req.Context())
	}

	if req.Body != nil {
		restyReq.SetBody(req.Body)
	}

	resp, err := restyReq.Execute(req.Method, req.URL.String())
	if err != nil {
		return nil, err
	}

	httpResp := resp.RawResponse
	if httpResp != nil {
		httpResp.Body = io.NopCloser(bytes.NewReader(resp.Body()))
	}

	return httpResp, nil
}

func (a *OpenAPIHTTPAdapter) RoundTrip(req *http.Request) (*http.Response, error) {
	return a.Do(req)
}

func CreateResilientOpenAPIClient(cfg *config.Config, logger *slog.Logger, serviceName string) *OpenAPIHTTPAdapter {
	restyClient := CreateResilientHTTPClient(cfg, logger, serviceName)
	return NewOpenAPIHTTPAdapter(restyClient)
}
