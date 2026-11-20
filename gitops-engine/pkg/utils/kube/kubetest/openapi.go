package kubetest

import (
	"errors"
	"sync/atomic"

	"github.com/go-logr/logr"
	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/openapi"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// FakeOpenAPIClient serves fixed OpenAPI documents and counts the requests it receives.
type FakeOpenAPIClient struct {
	// V2 is the OpenAPI v2 document.
	V2 *openapi_v2.Document
	// V3 maps OpenAPI v3 paths (e.g. "apis/apps/v1") to their JSON documents. If nil, the client
	// reports that the server doesn't serve OpenAPI v3.
	V3 map[string][]byte
	// Err, if set, is returned by every request.
	Err error

	V2Requests      atomic.Int32
	V3PathsRequests atomic.Int32
	V3GVRequests    atomic.Int32
}

var _ kube.OpenAPIClient = &FakeOpenAPIClient{}

// NewOpenAPISchemaResolver returns a resolver that fetches schemas from c.
func (c *FakeOpenAPIClient) NewOpenAPISchemaResolver() kube.OpenAPISchemaResolver {
	return kube.NewOpenAPISchemaResolverFromClient(c, logr.Discard())
}

func (c *FakeOpenAPIClient) OpenAPISchema() (*openapi_v2.Document, error) {
	c.V2Requests.Add(1)
	if c.Err != nil {
		return nil, c.Err
	}
	if c.V2 == nil {
		return nil, errors.New("no OpenAPI v2 document")
	}
	return c.V2, nil
}

func (c *FakeOpenAPIClient) OpenAPIV3() openapi.Client {
	return fakeOpenAPIV3Client{c}
}

type fakeOpenAPIV3Client struct {
	c *FakeOpenAPIClient
}

func (f fakeOpenAPIV3Client) Paths() (map[string]openapi.GroupVersion, error) {
	f.c.V3PathsRequests.Add(1)
	if f.c.Err != nil {
		return nil, f.c.Err
	}
	if f.c.V3 == nil {
		return nil, apierrors.NewNotFound(schema.GroupResource{}, "/openapi/v3")
	}
	paths := map[string]openapi.GroupVersion{}
	for path, doc := range f.c.V3 {
		paths[path] = fakeGroupVersion{c: f.c, doc: doc}
	}
	return paths, nil
}

type fakeGroupVersion struct {
	c   *FakeOpenAPIClient
	doc []byte
}

func (g fakeGroupVersion) Schema(string) ([]byte, error) {
	g.c.V3GVRequests.Add(1)
	if g.c.Err != nil {
		return nil, g.c.Err
	}
	return g.doc, nil
}

func (fakeGroupVersion) ServerRelativeURL() string {
	return ""
}
