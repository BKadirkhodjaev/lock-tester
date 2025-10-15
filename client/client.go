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

func (r *RequestClient) CreateEndpoint(path string) (string, error) {
	parsedUrl, err := url.Parse(r.URI)
	if err != nil {
		return "", err
	}

	parsedUrl.Path, err = url.JoinPath(parsedUrl.Path, path)
	if err != nil {
		return "", err
	}

	return parsedUrl.String(), nil
}

func (r *RequestClient) CreateHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "*/*",
		"x-okapi-tenant": r.Tenant,
	}
}

func (r *RequestClient) CreateHeadersWithToken(token string) map[string]string {
	headers := r.CreateHeaders()
	headers["x-okapi-token"] = token

	return headers
}

func (r *RequestClient) DoPostReturnMapStringAny(url string, bodyBytes []byte, headers map[string]string) (map[string]any, error) {
	var respMap map[string]any
	dumpBody(bodyBytes)

	req, err := retryablehttp.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}

	addRequestHeaders(req.Request, headers)

	err = dumpRequest(req.Request)
	if err != nil {
		return nil, err
	}

	resp, err := r.createRetryableClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = r.checkResponseStatusCodes(resp)
		if err != nil {
			r.Logger.Error(err.Error())
			return
		}

		if err := resp.Body.Close(); err != nil {
			r.Logger.Error(err.Error())
		}
	}()

	err = dumpResponse(resp, false)
	if err != nil {
		return nil, err
	}

	if resp.ContentLength == 0 {
		return map[string]any{}, nil
	}

	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}

		return nil, err
	}

	return respMap, nil
}

func (r *RequestClient) DoGetReturnMapStringAny(url string, headers map[string]string) (map[string]any, error) {
	var respMap map[string]any

	req, err := retryablehttp.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	addRequestHeaders(req.Request, headers)

	err = dumpRequest(req.Request)
	if err != nil {
		return nil, err
	}

	resp, err := r.createRetryableClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = r.checkResponseStatusCodes(resp)
		if err != nil {
			r.Logger.Error(err.Error())
			return
		}

		if err := resp.Body.Close(); err != nil {
			r.Logger.Error(err.Error())
		}
	}()

	err = dumpResponse(resp, false)
	if err != nil {
		return nil, err
	}

	if resp.ContentLength == 0 {
		return map[string]any{}, nil
	}

	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return map[string]any{}, nil
		}

		return nil, err
	}

	return respMap, nil
}

func (r *RequestClient) DoPutReturnNoContent(url string, bodyBytes []byte, headers map[string]string) error {
	dumpBody(bodyBytes)

	req, err := retryablehttp.NewRequest(http.MethodPut, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return err
	}

	addRequestHeaders(req.Request, headers)

	err = dumpRequest(req.Request)
	if err != nil {
		return err
	}

	resp, err := r.createRetryableClient().Do(req)
	if err != nil {
		return err
	}
	defer func() {
		err = r.checkResponseStatusCodes(resp)
		if err != nil {
			r.Logger.Error(err.Error())
			return
		}

		if err := resp.Body.Close(); err != nil {
			r.Logger.Error(err.Error())
		}
	}()

	err = dumpResponse(resp, false)
	if err != nil {
		return err
	}

	return nil
}

func addRequestHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Add(key, value)
	}
}

func (r *RequestClient) createRetryableClient() *retryablehttp.Client {
	client := retryablehttp.NewClient()
	client.RetryMax = r.RetryMax
	client.RetryWaitMax = r.RetryWaitMax
	client.Logger = r.Logger

	return client
}

func (r *RequestClient) checkResponseStatusCodes(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	err := dumpResponse(resp, true)
	if err != nil {
		return err
	}

	r.Logger.Error(fmt.Sprintf("unacceptable request status %d for URL: %s", resp.StatusCode, resp.Request.URL.String()))

	return nil
}
