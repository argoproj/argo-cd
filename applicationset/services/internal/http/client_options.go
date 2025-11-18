package http

import "time"

// ClientOptionFunc can be used to customize a new Restful API client.
type ClientOptionFunc func(*Client) error

// WithToken is an option for NewClient to set token
func WithToken(token string) ClientOptionFunc {
	return func(c *Client) error {
		c.token = token
		return nil
	}
}

// WithTimeout can be used to configure a custom timeout for requests.
func WithTimeout(timeout int) ClientOptionFunc {
	return func(c *Client) error {
		c.client.Timeout = time.Duration(timeout) * time.Second
		return nil
	}
}
