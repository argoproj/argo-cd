package scmmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string, insecure bool, rootCAPath string, caCerts []byte) *Client {
	if token == "" {
		token = os.Getenv("SCMM_TOKEN")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = utils.GetTlsConfig(rootCAPath, insecure, caCerts)
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		token:      token,
		httpClient: &http.Client{Transport: transport, Timeout: 30 * time.Second},
	}
}

func RepositoryPath(namespace, name string) string {
	return "/api/v2/repositories/" + url.PathEscape(namespace) + "/" + url.PathEscape(name)
}

func (c *Client) get(ctx context.Context, path string, query url.Values) (*http.Response, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.httpClient.Do(req)
}

func (c *Client) GetJSON(ctx context.Context, path string, query url.Values, result any) error {
	resp, err := c.get(ctx, path, query)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("SCM-Manager %s: HTTP %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decode SCM-Manager response for %s: %w", path, err)
	}
	return nil
}

func (c *Client) HasContent(ctx context.Context, path string) (bool, error) {
	resp, err := c.get(ctx, path, nil)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return false, fmt.Errorf("SCM-Manager %s: HTTP %d", path, resp.StatusCode)
	}
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return false, fmt.Errorf("read SCM-Manager content for %s: %w", path, err)
	}
	return true, nil
}

// ListPages collects a paged SCM-Manager collection. decode extracts its items
// and pagination metadata from the response specific to the endpoint.
func ListPages[T, Page any](ctx context.Context, c *Client, path string, query url.Values, decode func(Page) ([]T, int, int)) ([]T, error) {
	var items []T
	for page := 0; ; page++ {
		if page > 0 {
			query.Set("page", strconv.Itoa(page))
		}
		var result Page
		if err := c.GetJSON(ctx, path, query, &result); err != nil {
			return nil, err
		}
		entries, current, total := decode(result)
		if current != page {
			return nil, fmt.Errorf("SCM-Manager %s: requested page %d, received page %d", path, page, current)
		}
		items = append(items, entries...)
		if page+1 >= total {
			return items, nil
		}
	}
}
