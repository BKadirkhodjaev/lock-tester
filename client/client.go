package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

type RequestClient struct {
	Logger       *slog.Logger
	URI          string
	Tenant       string
	RetryMax     int
	RetryWaitMax time.Duration
}

func (r RequestClient) CreateEndpoint(path string) string {
	parsedUrl, err := url.Parse(r.URI)
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}

	parsedUrl.Path, err = url.JoinPath(parsedUrl.Path, path)
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}

	return parsedUrl.String()
}

func (r RequestClient) CreateHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "*/*",
		"x-okapi-tenant": r.Tenant,
	}
}

func (r RequestClient) CreateHeadersWithToken(token string) map[string]string {
	headers := r.CreateHeaders()
	headers["x-okapi-token"] = token

	return headers
}

func (r RequestClient) DoPostReturnMapStringInteface(url string, bodyBytes []byte, headers map[string]string) map[string]any {
	var respMap map[string]any
	dumpHttpBody(bodyBytes)

	req, err := retryablehttp.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}

	addRequestHeaders(req.Request, headers)
	dumpHttpRequest(r.Logger, req.Request)

	resp, err := r.createRetryableClient().Do(req)
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}
	defer func() {
		checkStatusCodes(r.Logger, resp)
		if err := resp.Body.Close(); err != nil {
			r.Logger.Error(err.Error())
		}
	}()

	dumpHttpResponse(r.Logger, resp)

	if resp.ContentLength == 0 {
		return map[string]any{}
	}

	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}
		}
		r.Logger.Error(err.Error())
		panic(err)
	}

	return respMap
}

func (r RequestClient) DoGetReturnMapStringInterface(url string, headers map[string]string) map[string]any {
	var respMap map[string]any

	req, err := retryablehttp.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}

	addRequestHeaders(req.Request, headers)
	dumpHttpRequest(r.Logger, req.Request)

	resp, err := r.createRetryableClient().Do(req)
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}
	defer func() {
		checkStatusCodes(r.Logger, resp)
		if err := resp.Body.Close(); err != nil {
			r.Logger.Error(err.Error())
		}
	}()

	dumpHttpResponse(r.Logger, resp)

	if resp.ContentLength == 0 {
		return map[string]any{}
	}

	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}
		}
		r.Logger.Error(err.Error())
		panic(err)
	}

	return respMap
}

func (r RequestClient) DoPutReturnNoContent(url string, bodyBytes []byte, headers map[string]string) {
	dumpHttpBody(bodyBytes)

	req, err := retryablehttp.NewRequest(http.MethodPut, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}

	addRequestHeaders(req.Request, headers)
	dumpHttpRequest(r.Logger, req.Request)

	resp, err := r.createRetryableClient().Do(req)
	if err != nil {
		r.Logger.Error(err.Error())
		panic(err)
	}
	defer func() {
		checkStatusCodes(r.Logger, resp)
		if err := resp.Body.Close(); err != nil {
			r.Logger.Error(err.Error())
		}
	}()

	dumpHttpResponse(r.Logger, resp)
}

func addRequestHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Add(key, value)
	}
}

func (r RequestClient) createRetryableClient() *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.RetryMax = r.RetryMax
	client.RetryWaitMax = r.RetryWaitMax
	client.Logger = r.Logger

	return client
}

func checkStatusCodes(logger *slog.Logger, resp *http.Response) {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return
	}

	dumpHttpResponse(logger, resp)

	msg := fmt.Sprintf("unacceptable request status %d for URL: %s", resp.StatusCode, resp.Request.URL.String())
	logger.Error(msg)
	panic(errors.New(msg))
}
