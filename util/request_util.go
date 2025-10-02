package util

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
)

func CreateEndpoint(commandName string, path string) string {
	parsedUrl, err := url.Parse("http://localhost:8000")
	if err != nil {
		slog.Error(commandName, GetFuncName(), "url.Parse error")
		panic(err)
	}

	parsedUrl.Path, err = url.JoinPath(parsedUrl.Path, path)
	if err != nil {
		slog.Error(commandName, GetFuncName(), "url.JoinPath error")
		panic(err)
	}

	return parsedUrl.String()
}

func CreateHeaders() map[string]string {
	return map[string]string{
		"Content-Type":   "application/json",
		"Accept":         "*/*",
		"x-okapi-tenant": "diku",
	}
}

func CreateHeadersWithToken(token string) map[string]string {
	headers := CreateHeaders()
	headers["x-okapi-token"] = token

	return headers
}

func DoPostReturnMapStringInteface(commandName string, url string, enableDebug bool, bodyBytes []byte, headers map[string]string) map[string]any {
	var respMap map[string]any
	DumpHttpBody(commandName, enableDebug, bodyBytes)

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		slog.Error(commandName, GetFuncName(), "http.NewRequest error")
		panic(err)
	}

	AddRequestHeaders(req, headers)
	DumpHttpRequest(commandName, req, enableDebug)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error(commandName, GetFuncName(), "http.DefaultClient.Do error")
		panic(err)
	}
	defer func() {
		CheckStatusCodes(commandName, resp)
		if err := resp.Body.Close(); err != nil {
			slog.Error(commandName, GetFuncName(), "resp.Body.Close error")
		}
	}()

	DumpHttpResponse(commandName, resp, enableDebug)

	if resp.ContentLength == 0 {
		return map[string]any{}
	}

	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		if err.Error() == "EOF" {
			return map[string]any{}
		}
		slog.Error(commandName, GetFuncName(), "json.NewDecoder error")
		panic(err)
	}

	return respMap
}

func DoGetReturnMapStringInterface(commandName string, url string, enableDebug bool, headers map[string]string) map[string]any {
	var respMap map[string]any

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		slog.Error(commandName, GetFuncName(), "http.NewRequest error")
		panic(err)
	}

	AddRequestHeaders(req, headers)
	DumpHttpRequest(commandName, req, enableDebug)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error(commandName, GetFuncName(), "http.DefaultClient.Do error")
		panic(err)
	}
	defer func() {
		CheckStatusCodes(commandName, resp)
		if err := resp.Body.Close(); err != nil {
			slog.Error(commandName, GetFuncName(), "resp.Body.Close error")
		}
	}()

	DumpHttpResponse(commandName, resp, enableDebug)

	if resp.ContentLength == 0 {
		return map[string]any{}
	}

	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		if err.Error() == "EOF" {
			return map[string]any{}
		}
		slog.Error(commandName, GetFuncName(), "json.NewDecoder error")
		panic(err)
	}

	return respMap
}

func DoPutReturnNoContent(commandName string, url string, enableDebug bool, bodyBytes []byte, headers map[string]string) {
	DumpHttpBody(commandName, enableDebug, bodyBytes)

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		slog.Error(commandName, GetFuncName(), "http.NewRequest error")
		panic(err)
	}

	AddRequestHeaders(req, headers)
	DumpHttpRequest(commandName, req, enableDebug)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		slog.Error(commandName, GetFuncName(), "http.DefaultClient.Do error")
		panic(err)
	}
	defer func() {
		CheckStatusCodes(commandName, resp)
		if err := resp.Body.Close(); err != nil {
			slog.Error(commandName, GetFuncName(), "resp.Body.Close error")
		}
	}()

	DumpHttpResponse(commandName, resp, enableDebug)
}

func AddRequestHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Add(key, value)
	}
}
