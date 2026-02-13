package sourcecraft

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
)

// Client represents an HTTP client for interacting with the Sourcecraft API.
// It manages authentication, HTTP connections, and provides thread-safe access
// to API endpoints. All methods are safe for concurrent use.
type Client struct {
	mutex       sync.RWMutex
	url         string
	accessToken string
	client      *http.Client
}

// Response wraps the standard http.Response to provide additional
// functionality for API responses.
type Response struct {
	*http.Response
}

// ClientOption are functions used to init a new client
type ClientOption func(*Client) error

// NewClient initializes and returns a API client.
// Usage of all Client methods is thread safe.
func NewClient(url string, options ...ClientOption) (*Client, error) {
	client := &Client{
		url:    strings.TrimSuffix(url, "/"),
		client: &http.Client{},
	}
	for _, opt := range options {
		if err := opt(client); err != nil {
			return nil, err
		}
	}
	return client, nil
}

// SetHTTPClient replaces the default http.Client with a user-provided one.
// This method is thread-safe and can be called concurrently.
func (c *Client) SetHTTPClient(client *http.Client) {
	c.mutex.Lock()
	c.client = client
	c.mutex.Unlock()
}

// SetToken is an option for NewClient to set token
func SetToken(token string) ClientOption {
	return func(client *Client) error {
		client.mutex.Lock()
		client.accessToken = token
		client.mutex.Unlock()
		return nil
	}
}

// SetHTTPClient returns a ClientOption that replaces the default http.Client
// with a user-provided one during client initialization.
func SetHTTPClient(httpClient *http.Client) ClientOption {
	return func(client *Client) error {
		client.SetHTTPClient(httpClient)
		return nil
	}
}

// WithHTTPClient returns a ClientOption that configures the HTTP client
// with optional TLS verification settings. When insecure is true, it creates
// a client that skips TLS certificate verification (useful for testing but
// not recommended for production).
func WithHTTPClient(insecure bool) ClientOption {
	httpClient := &http.Client{}
	if insecure {
		cookieJar, _ := cookiejar.New(nil)
		tr := http.DefaultTransport.(*http.Transport).Clone()
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		httpClient.Jar = cookieJar
		httpClient.Transport = tr
	}
	return func(client *Client) error {
		client.SetHTTPClient(httpClient)
		return nil
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, header http.Header, body io.Reader) (*Response, error) {
	c.mutex.RLock()
	req, err := http.NewRequestWithContext(ctx, method, c.url+path, body)
	if err != nil {
		c.mutex.RUnlock()
		return nil, err
	}
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}

	client := c.client
	c.mutex.RUnlock()

	maps.Copy(req.Header, header)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return newResponse(resp), nil
}

func (c *Client) getResponse(ctx context.Context, method, path string, header http.Header, body io.Reader) ([]byte, *Response, error) {
	resp, err := c.doRequest(ctx, method, path, header, body)
	if err != nil {
		return nil, resp, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	// check for errors
	data, err := statusCodeToErr(resp)
	if err != nil {
		return data, resp, err
	}

	// success (2XX), read body
	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp, err
	}

	return data, resp, nil
}

func statusCodeToErr(resp *Response) (body []byte, err error) {
	// no error
	if resp.StatusCode/100 == 2 {
		return nil, nil
	}

	// error: body will be read for details
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("body read on HTTP error %d: %w", resp.StatusCode, err)
	}

	errMap := make(map[string]any)
	if err = json.Unmarshal(data, &errMap); err != nil {
		path := resp.Request.URL.Path
		method := resp.Request.Method
		return data, fmt.Errorf("unknown API error: %d\nRequest: '%s' with '%s' method and '%s' body", resp.StatusCode, path, method, string(data))
	}

	if msg, ok := errMap["message"]; ok {
		return data, fmt.Errorf("%v", msg)
	}

	return data, fmt.Errorf("%s: %s", resp.Status, string(data))
}

func newResponse(r *http.Response) *Response {
	response := &Response{Response: r}
	return response
}

func (c *Client) getParsedResponse(
	ctx context.Context,
	method, path string,
	header http.Header,
	body io.Reader,
	obj any,
) (*Response, error) {
	data, resp, err := c.getResponse(ctx, method, path, header, body)
	if err != nil {
		return resp, err
	}
	return resp, json.Unmarshal(data, obj)
}

func escapeValidatePathSegments(seg ...*string) error {
	for i := range seg {
		if seg[i] == nil || *seg[i] == "" {
			return fmt.Errorf("path segment [%d] is empty", i)
		}
		*seg[i] = url.PathEscape(*seg[i])
	}
	return nil
}
