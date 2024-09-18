package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	userAgent      = "argocd-applicationset"
	defaultTimeout = 30
)

type Client struct {
	// URL is the URL used for API requests.
	baseURL string

	// UserAgent is the user agent to include in HTTP requests.
	UserAgent string

	// Token is used to make authenticated API calls.
	token string

	// Client is an HTTP client used to communicate with the API.
	client *http.Client
}

type ErrorResponse struct {
	Body     []byte
	Response *http.Response
	Message  string
}

func NewClient(baseURL string, options ...ClientOptionFunc) (*Client, error) {
	client, err := newClient(baseURL, options...)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func newClient(baseURL string, options ...ClientOptionFunc) (*Client, error) {
	c := &Client{baseURL: baseURL, UserAgent: userAgent}

	// Configure the HTTP client.
	c.client = &http.Client{
		Timeout: time.Duration(defaultTimeout) * time.Second,
	}

	// Apply any given client options.
	for _, fn := range options {
		if fn == nil {
			continue
		}
		if err := fn(c); err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *Client) NewRequest(method, path string, body interface{}, options []ClientOptionFunc) (*http.Request, error) {
	// Make sure the given URL end with a slash
	if !strings.HasSuffix(c.baseURL, "/") {
		c.baseURL += "/"
	}

	var buf io.ReadWriter
	if body != nil {
		buf = &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		err := enc.Encode(body)
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest(method, c.baseURL+path, buf)
	if err != nil {
		return nil, err
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if len(c.token) != 0 {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}

	return req, nil
}

func (c *Client) Do(ctx context.Context, req *http.Request, v interface{}) (*http.Response, error) {
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if err := CheckResponse(resp); err != nil {
		return resp, err
	}

	switch v := v.(type) {
	case nil:
	case io.Writer:
		_, err = io.Copy(v, resp.Body)
	default:
		buf := new(bytes.Buffer)
		teeReader := io.TeeReader(resp.Body, buf)
		decErr := json.NewDecoder(teeReader).Decode(v)
		if decErr == io.EOF {
			decErr = nil // ignore EOF errors caused by empty response body
		}
		if decErr != nil {
			err = fmt.Errorf("%s: %s", decErr.Error(), buf.String())
		}
	}
	return resp, err
}

// CheckResponse checks the API response for errors, and returns them if present.
func CheckResponse(resp *http.Response) error {
	if c := resp.StatusCode; 200 <= c && c <= 299 {
		return nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("API error with status code %d: %w", resp.StatusCode, err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("API error with status code %d: %s", resp.StatusCode, string(data))
	}

	message := ""
	if value, ok := raw["message"].(string); ok {
		message = value
	} else if value, ok := raw["error"].(string); ok {
		message = value
	}

	return fmt.Errorf("API error with status code %d: %s", resp.StatusCode, message)
}
