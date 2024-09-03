package codefresh

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
)

type CodefreshConfig struct {
	BaseURL     string
	AuthToken   string
	TlsInsecure bool
	CaCertPath  string
}

type CodefreshClient struct {
	cfConfig   *CodefreshConfig
	httpClient *http.Client
}

type CodefreshClientInterface interface {
	SendEvent(ctx context.Context, appName string, event *events.Event) error
	SendGraphQL(query GraphQLQuery) (*json.RawMessage, error)
}

// GraphQLQuery structure to form a GraphQL query
type GraphQLQuery struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

func (c *CodefreshClient) SendEvent(ctx context.Context, appName string, event *events.Event) error {
	return WithRetry(&DefaultBackoff, func() error {
		url, err := url.JoinPath(c.cfConfig.BaseURL, "/2.0/api/events")
		if err != nil {
			return fmt.Errorf("failed to join URL: %w", err)
		}

		log.Infof("Sending application event for %s", appName)

		wrappedPayload := map[string]json.RawMessage{
			"data": event.Payload,
		}

		newPayloadBytes, err := json.Marshal(wrappedPayload)
		if err != nil {
			return err
		}

		// Create a buffer to hold the compressed data
		var buf bytes.Buffer

		// Create a gzip writer
		gz := gzip.NewWriter(&buf)
		defer gz.Close()

		// Write the data to the gzip writer
		if _, err := gz.Write(newPayloadBytes); err != nil {
			return err
		}
		if err := gz.Close(); err != nil {
			return err
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, io.NopCloser(&buf))
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Content-Encoding", "gzip")
		req.Header.Set("Authorization", c.cfConfig.AuthToken)

		res, err := c.httpClient.Do(req)
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed reporting to Codefresh, event: %s", string(event.Payload)))
		}
		defer res.Body.Close()

		isStatusOK := res.StatusCode >= 200 && res.StatusCode < 300
		if !isStatusOK {
			b, _ := io.ReadAll(res.Body)
			return errors.Errorf("failed reporting to Codefresh, got response: status code %d and body %s, original request body: %s",
				res.StatusCode, string(b), string(event.Payload))
		}

		log.Infof("Application event for %s successfully sent", appName)
		return nil
	})
}

// sendGraphQLRequest function to send the GraphQL request and handle the response
func (c *CodefreshClient) SendGraphQL(query GraphQLQuery) (*json.RawMessage, error) {
	queryJSON, err := json.Marshal(query)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.cfConfig.BaseURL+"/2.0/api/graphql", bytes.NewBuffer(queryJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", c.cfConfig.AuthToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, errors.New(resp.Status)
	}
	defer resp.Body.Close()

	var responseStruct struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&responseStruct); err != nil {
		return nil, err
	}

	return &responseStruct.Data, nil
}

func NewCodefreshClient(cfConfig *CodefreshConfig) CodefreshClientInterface {
	return &CodefreshClient{
		cfConfig:   cfConfig,
		httpClient: cfConfig.getHttpClient(),
	}
}

func (cfConfig *CodefreshConfig) getHttpClient() *http.Client {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	httpClient.Transport = &http.Transport{
		TLSClientConfig: cfConfig.getTlsConfig(),
	}

	return httpClient
}

func (cfConfig *CodefreshConfig) getTlsConfig() *tls.Config {
	c := &tls.Config{}

	if cfConfig.TlsInsecure {
		return &tls.Config{
			InsecureSkipVerify: true,
			ClientAuth:         0,
		}
	}

	if cfConfig.CaCertPath != "" {
		cert, err := os.ReadFile(cfConfig.CaCertPath)
		if err != nil {
			log.Fatal(err)
		}
		pool := x509.NewCertPool()
		if ok := pool.AppendCertsFromPEM(cert); !ok {
			log.Fatalf("unable to parse codefresh cert from path %s", cfConfig.CaCertPath)
		}
		c.RootCAs = pool
	}

	return c
}
