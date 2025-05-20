package application_change_revision_controller

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/grpc"

	appclient "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
)

type ApplicationClient interface {
	GetChangeRevision(ctx context.Context, in *appclient.ChangeRevisionRequest, opts ...grpc.CallOption) (*appclient.ChangeRevisionResponse, error)
}

type httpApplicationClient struct {
	httpClient *http.Client
	baseURL    string
	token      string
	rootpath   string
}

func NewHTTPApplicationClient(token string, address string, rootpath string) ApplicationClient {
	if rootpath != "" && !strings.HasPrefix(rootpath, "/") {
		rootpath = "/" + rootpath
	}

	if !strings.Contains(address, "http") {
		address = "http://" + address
	}

	if rootpath != "" {
		address = address + rootpath
	}

	return &httpApplicationClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				// Support for insecure connections
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		baseURL:  address,
		token:    token,
		rootpath: rootpath,
	}
}

func (c *httpApplicationClient) execute(ctx context.Context, url string, result any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.token)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)

	isStatusOK := res.StatusCode >= 200 && res.StatusCode < 300
	if !isStatusOK {
		return fmt.Errorf("argocd server respond with code %d, msg is: %s", res.StatusCode, string(b))
	}

	err = json.Unmarshal(b, &result)
	if err != nil {
		return err
	}
	return nil
}

func (c *httpApplicationClient) GetChangeRevision(ctx context.Context, in *appclient.ChangeRevisionRequest, _ ...grpc.CallOption) (*appclient.ChangeRevisionResponse, error) {
	params := fmt.Sprintf("?appName=%s&namespace=%s&currentRevision=%s&previousRevision=%s", in.GetAppName(), in.GetNamespace(), in.GetCurrentRevision(), in.GetPreviousRevision())

	url := fmt.Sprintf("%s/api/v1/application/changeRevision%s", c.baseURL, params)

	changeRevisionResponse := &appclient.ChangeRevisionResponse{}
	err := c.execute(ctx, url, changeRevisionResponse)
	if err != nil {
		return nil, err
	}
	return changeRevisionResponse, nil
}
