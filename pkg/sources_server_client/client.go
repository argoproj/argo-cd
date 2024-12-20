package sources_server_client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	netUrl "net/url"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type VersionPayload struct {
	App      v1alpha1.Application `json:"app"`
	Revision string               `json:"revision"`
}

type DependenciesMap struct {
	Lock         string `json:"helm/Chart.lock"`
	Deps         string `json:"helm/dependencies"`
	Requirements string `json:"helm/requirements.yaml"`
}

type AppVersionResult struct {
	AppVersion   string          `json:"appVersion"`
	Dependencies DependenciesMap `json:"dependencies"`
}

type SourcesServerConfig struct {
	BaseURL string
}

type sourceServerClient struct {
	clientConfig *SourcesServerConfig
}

type SourceServerClientInteface interface {
	GetAppVersion(app *v1alpha1.Application, revision *string) *AppVersionResult
}

func (c *sourceServerClient) sendRequest(method, url string, payload interface{}) ([]byte, error) {
	var requestBody []byte
	var err error
	if payload != nil {
		requestBody, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("error marshalling payload: %w", err)
		}
	}

	fullURL, err := netUrl.JoinPath(c.clientConfig.BaseURL, url)
	if err != nil {
		return nil, fmt.Errorf("error joining path: %w", err)
	}

	req, err := http.NewRequest(method, fullURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("server responded with status %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	return body, nil
}

func (c *sourceServerClient) GetAppVersion(app *v1alpha1.Application, revision *string) *AppVersionResult {
	log.Infof("cfGetAppVersion. Sending request to sources-server for %s", app.Name)
	appVersionResult, err := c.sendRequest("POST", "/getAppVersion", VersionPayload{App: *app, Revision: *revision})
	if err != nil {
		log.Errorf("error getting app version: %v", err)
		return nil
	}

	var versionStruct AppVersionResult
	err = json.Unmarshal(appVersionResult, &versionStruct)
	if err != nil {
		log.Errorf("error unmarshaling app version: %v", err)
		return nil
	}

	return &versionStruct
}

func NewSourceServerClient(clientConfig *SourcesServerConfig) SourceServerClientInteface {
	return &sourceServerClient{
		clientConfig: clientConfig,
	}
}
