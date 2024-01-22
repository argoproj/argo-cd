package application

import (
	"context"
	"encoding/json"
	"fmt"
	appclient "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"google.golang.org/grpc"
	"io"
	"net/http"
	"strings"
	"time"
)

type ApplicationClient interface {
	Get(ctx context.Context, in *appclient.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.Application, error)

	RevisionMetadata(ctx context.Context, in *appclient.RevisionMetadataQuery, opts ...grpc.CallOption) (*v1alpha1.RevisionMetadata, error)

	GetManifests(ctx context.Context, in *appclient.ApplicationManifestQuery, opts ...grpc.CallOption) (*repoapiclient.ManifestResponse, error)

	ResourceTree(ctx context.Context, in *appclient.ResourcesQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationTree, error)

	GetResource(ctx context.Context, in *appclient.ApplicationResourceRequest, opts ...grpc.CallOption) (*appclient.ApplicationResourceResponse, error)

	List(ctx context.Context, in *appclient.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationList, error)
}

type httpApplicationClient struct {
	httpClient *http.Client
	baseUrl    string
	token      string
	rootpath   string
}

func NewHttpApplicationClient(token string, address string, rootpath string) ApplicationClient {
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
		},
		baseUrl:  address,
		token:    token,
		rootpath: rootpath,
	}
}

func (c *httpApplicationClient) execute(ctx context.Context, url string, result interface{}, printBody ...bool) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

func (c *httpApplicationClient) Get(ctx context.Context, in *appclient.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	params := fmt.Sprintf("?appNamespace=%s",
		*in.AppNamespace)
	url := fmt.Sprintf("%s/api/v1/applications/%s%s", c.baseUrl, *in.Name, params)
	application := &v1alpha1.Application{}
	err := c.execute(ctx, url, application)
	if err != nil {
		return nil, err
	}
	return application, nil
}

func (c *httpApplicationClient) RevisionMetadata(ctx context.Context, in *appclient.RevisionMetadataQuery, opts ...grpc.CallOption) (*v1alpha1.RevisionMetadata, error) {
	params := fmt.Sprintf("?appNamespace=%s&project=%s",
		*in.AppNamespace,
		*in.Project)
	url := fmt.Sprintf("%s/api/v1/applications/%s/revisions/%s/metadata%s", c.baseUrl, *in.Name, *in.Revision, params)
	revisionMetadata := &v1alpha1.RevisionMetadata{}
	err := c.execute(ctx, url, revisionMetadata)
	if err != nil {
		return nil, err
	}
	return revisionMetadata, nil
}

func (c *httpApplicationClient) GetManifests(ctx context.Context, in *appclient.ApplicationManifestQuery, opts ...grpc.CallOption) (*repoapiclient.ManifestResponse, error) {
	params := fmt.Sprintf("?appNamespace=%s&project=%s",
		*in.AppNamespace,
		*in.Project)
	url := fmt.Sprintf("%s/api/v1/applications/%s/manifests%s", c.baseUrl, *in.Name, params)

	manifest := &repoapiclient.ManifestResponse{}
	err := c.execute(ctx, url, manifest)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func (c *httpApplicationClient) ResourceTree(ctx context.Context, in *appclient.ResourcesQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationTree, error) {
	params := fmt.Sprintf("?appNamespace=%s&project=%s",
		*in.AppNamespace,
		*in.Project)
	url := fmt.Sprintf("%s/api/v1/applications/%s/resource-tree%s", c.baseUrl, *in.ApplicationName, params)
	tree := &v1alpha1.ApplicationTree{}
	err := c.execute(ctx, url, tree)
	if err != nil {
		return nil, err
	}
	return tree, nil
}

func (c *httpApplicationClient) GetResource(ctx context.Context, in *appclient.ApplicationResourceRequest, opts ...grpc.CallOption) (*appclient.ApplicationResourceResponse, error) {
	params := fmt.Sprintf("?appNamespace=%s&namespace=%s&resourceName=%s&version=%s&group=%s&kind=%s&project=%s",
		*in.AppNamespace,
		*in.Namespace,
		*in.ResourceName,
		*in.Version,
		*in.Group,
		*in.Kind,
		*in.Project)
	url := fmt.Sprintf("%s/api/v1/applications/%s/resource%s", c.baseUrl, *in.Name, params)

	applicationResource := &appclient.ApplicationResourceResponse{}
	err := c.execute(ctx, url, applicationResource, true)
	if err != nil {
		return nil, err
	}
	return applicationResource, nil
}

func (c *httpApplicationClient) List(ctx context.Context, in *appclient.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationList, error) {
	url := fmt.Sprintf("%s/api/v1/applications", c.baseUrl)

	apps := &v1alpha1.ApplicationList{}
	err := c.execute(ctx, url, apps)
	if err != nil {
		return nil, err
	}
	return apps, nil
}
