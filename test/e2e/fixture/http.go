package fixture

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/argoproj/argo-cd/v3/common"
)

// DoHttpRequest executes a http request against the Argo CD API server
func DoHttpRequest(method string, path string, host string, data ...byte) (*http.Response, error) { //nolint:revive //FIXME(var-naming)
	reqURL, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	reqURL.Scheme = "http"
	if host != "" {
		reqURL.Host = host
	} else {
		reqURL.Host = apiServerAddress
	}
	var body io.Reader
	if data != nil {
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}
	req.AddCookie(&http.Cookie{Name: common.AuthCookieName, Value: token})
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: IsRemote()},
		},
	}

	return httpClient.Do(req)
}

// DoHttpJsonRequest executes a http request against the Argo CD API server and unmarshals the response body as JSON
func DoHttpJsonRequest(method string, path string, result any, data ...byte) error { //nolint:revive //FIXME(var-naming)
	resp, err := DoHttpRequest(method, path, "", data...)
	if err != nil {
		return err
	}
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(responseData, result)
}
