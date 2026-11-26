package kube

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"weak"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/kube-openapi/pkg/util/proto"
	"k8s.io/kubectl/pkg/util/openapi"

	"github.com/stretchr/testify/assert"
	"k8s.io/klog/v2/textlogger"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/tracing"
)

var _ Kubectl = &KubectlCmd{}

func TestConvertToVersion(t *testing.T) {
	t.Parallel()
	kubectl := KubectlCmd{
		Log:    textlogger.NewLogger(textlogger.NewConfig()),
		Tracer: tracing.NopTracer{},
	}
	t.Run("AppsDeployment", func(t *testing.T) {
		t.Parallel()
		newObj, err := kubectl.ConvertToVersion(testingutils.UnstructuredFromFile("testdata/appsdeployment.yaml"), "apps", "v1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "apps", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
		}
	})
	t.Run("CustomResource", func(t *testing.T) {
		t.Parallel()
		_, err := kubectl.ConvertToVersion(testingutils.UnstructuredFromFile("testdata/cr.yaml"), "argoproj.io", "v1")
		assert.Error(t, err)
	})
	t.Run("ExtensionsDeployment", func(t *testing.T) {
		t.Parallel()
		obj := testingutils.UnstructuredFromFile("testdata/nginx.yaml")

		// convert an extensions/v1beta1 object into itself
		newObj, err := kubectl.ConvertToVersion(obj, "extensions", "v1beta1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "extensions", gvk.Group)
			assert.Equal(t, "v1beta1", gvk.Version)
		}

		// convert an extensions/v1beta1 object into an apps/v1
		newObj, err = kubectl.ConvertToVersion(obj, "apps", "v1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "apps", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
		}

		// converting it again should not have any affect
		newObj, err = kubectl.ConvertToVersion(obj, "apps", "v1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "apps", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
		}
	})
	t.Run("newGVKParser gracefully handles duplicate GVKs", func(t *testing.T) {
		t.Parallel()
		client := &fakeOpenAPIClient{}
		doc, err := client.OpenAPISchema()
		require.NoError(t, err)
		models, err := proto.NewOpenAPIData(doc)
		require.NoError(t, err)
		_, err = kubectl.newGVKParser(models)
		require.NoError(t, err)
	})
}

func TestGetServerVersion(t *testing.T) {
	t.Parallel()
	t.Run("returns full semantic version with patch", func(t *testing.T) {
		t.Parallel()
		fakeServer := fakeHTTPServer(version.Info{
			Major:      "1",
			Minor:      "34",
			GitVersion: "v1.34.0",
			GitCommit:  "abc123def456",
			Platform:   "linux/amd64",
		}, nil)
		defer fakeServer.Close()
		config := mockConfig(fakeServer.URL)

		serverVersion, err := kubectlCmd().GetServerVersion(config)
		require.NoError(t, err)
		assert.Equal(t, "v1.34.0", serverVersion, "Should return full semantic serverVersion")
		assert.Regexp(t, `^v\d+\.\d+\.\d+`, serverVersion, "Should match semver pattern with 'v' prefix")
		assert.NotEqual(t, "1.34", serverVersion, "Should not be old Major.Minor format")
	})

	t.Run("do not preserver build metadata", func(t *testing.T) {
		t.Parallel()
		fakeServer := fakeHTTPServer(version.Info{
			Major:      "1",
			Minor:      "30",
			GitVersion: "v1.30.11+IKS",
			GitCommit:  "xyz789",
			Platform:   "linux/amd64",
		}, nil)
		defer fakeServer.Close()
		config := mockConfig(fakeServer.URL)

		serverVersion, err := kubectlCmd().GetServerVersion(config)
		require.NoError(t, err)
		assert.Equal(t, "v1.30.11", serverVersion, "Should not preserve build metadata")
		assert.NotContains(t, serverVersion, "+IKS", "Should not contain provider-specific metadata")
		assert.NotEqual(t, "1.30", serverVersion, "Should not strip to Major.Minor")
	})

	t.Run("handles error from discovery client", func(t *testing.T) {
		t.Parallel()
		fakeServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer fakeServer.Close()
		config := mockConfig(fakeServer.URL)

		_, err := kubectlCmd().GetServerVersion(config)
		assert.Error(t, err, "Should return error when server fails")
		assert.Contains(t, err.Error(), "failed to get server version",
			"Error should indicate version retrieval failure")
	})

	t.Run("handles minor version with plus suffix", func(t *testing.T) {
		t.Parallel()
		fakeServer := fakeHTTPServer(version.Info{
			Major:      "1",
			Minor:      "30+",
			GitVersion: "v1.30.0",
		}, nil)
		defer fakeServer.Close()
		config := mockConfig(fakeServer.URL)
		serverVersion, err := kubectlCmd().GetServerVersion(config)
		require.NoError(t, err)

		assert.Equal(t, "v1.30.0", serverVersion)
		assert.NotContains(t, serverVersion, "+", "Should not contain the '+' from Minor field")
	})
}

func kubectlCmd() *KubectlCmd {
	kubectl := &KubectlCmd{
		Log:    textlogger.NewLogger(textlogger.NewConfig()),
		Tracer: tracing.NopTracer{},
	}
	return kubectl
}

/**
Getting the test data here was challenging.

First I needed a Kubernetes cluster with an aggregated API installed which had disagreeing versions of the same
resource. In this case, I used Kubernetes 1.30.1 and metrics-server 0.7.1. They have different versions of
APIResourceList.

So then I got the protobuf representation of the aggregated OpenAPI doc. I used the following command to get it:
curl -k https://localhost:6443/openapi/v2 -H "Authorization: Bearer $token" -H "Accept: application/com.github.proto-openapi.spec.v2@v1.0+protobuf" > pkg/utils/kube/testdata/openapi_v2.proto

Then I unmarshaled the protobuf representation into JSON using the following code:

json.Unmarshal(openAPIDoc, document)
jsondata, _ := json.Marshal(document)
ioutil.WriteFile("pkg/utils/kube/testdata/openapi_v2.json", jsondata, 0644)

This step was necessary because it's not possible to unmarshal the json representation of the OpenAPI doc into the
openapi_v2.Document struct. I don't know why they're different.

Then I used this code to post-process the JSON representation of the OpenAPI doc and remove unnecessary information:
jq '.definitions.additional_properties |= map(select(.name | test(".*APIResource.*"))) | {definitions: {additional_properties: .definitions.additional_properties}}' pkg/utils/kube/testdata/openapi_v2.json > pkg/utils/kube/testdata/openapi_v2.json.better
mv pkg/utils/kube/testdata/openapi_v2.json.better pkg/utils/kube/testdata/openapi_v2.json

Hopefully we'll never need to reload the test data, because it was to demonstrate a very specific bug.
*/

//go:embed testdata/openapi_v2.json
var openAPIDoc []byte

type fakeOpenAPIClient struct{}

func (f *fakeOpenAPIClient) OpenAPISchema() (*openapi_v2.Document, error) {
	document := &openapi_v2.Document{}
	err := json.Unmarshal(openAPIDoc, document)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal OpenAPI document: %w", err)
	}
	return document, nil
}

func mockConfig(host string) *rest.Config {
	return &rest.Config{
		Host: host,
	}
}

func fakeHTTPServer(info version.Info, err error) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/version" {
			versionInfo := info
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(versionInfo)
			return
		}
		http.NotFound(w, r)
	}))
}

// weakOpenAPIClient serves the test OpenAPI document and keeps a weak pointer to the last
// document it returned, so tests can check whether anything still references it.
type weakOpenAPIClient struct {
	doc weak.Pointer[openapi_v2.Document]
}

func (c *weakOpenAPIClient) OpenAPISchema() (*openapi_v2.Document, error) {
	doc, err := (&fakeOpenAPIClient{}).OpenAPISchema()
	if err != nil {
		return nil, err
	}
	c.doc = weak.Make(doc)
	return doc, nil
}

func TestLoadOpenAPISchema(t *testing.T) {
	t.Parallel()

	resources, gvkParser, err := kubectlCmd().loadOpenAPISchema(&fakeOpenAPIClient{})
	require.NoError(t, err)
	require.NotNil(t, resources)
	assert.NotNil(t, gvkParser)

	t.Run("LookupResource matches kubectl's implementation", func(t *testing.T) {
		t.Parallel()
		doc, err := (&fakeOpenAPIClient{}).OpenAPISchema()
		require.NoError(t, err)
		expected, err := openapi.NewOpenAPIData(doc)
		require.NoError(t, err)
		models, err := proto.NewOpenAPIData(doc)
		require.NoError(t, err)

		var gvks []schema.GroupVersionKind
		for _, modelName := range models.ListModels() {
			gvks = append(gvks, parseGroupVersionKind(models.LookupModel(modelName))...)
		}
		require.NotEmpty(t, gvks, "test document should contain models with GVKs")

		for _, gvk := range gvks {
			want := expected.LookupResource(gvk)
			got := resources.LookupResource(gvk)
			require.NotNil(t, got, gvk.String())
			assert.Equal(t, want.GetPath().String(), got.GetPath().String(), gvk.String())
		}
	})

	t.Run("LookupResource returns nil for an unknown GVK", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, resources.LookupResource(schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Missing"}))
	})

	t.Run("GetConsumes returns nil", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, resources.GetConsumes(schema.GroupVersionKind{Version: "v1", Kind: "APIResourceList"}, "PATCH"))
	})
}

// The cluster cache keeps the openapi.Resources and GVK parser for every cluster, so they must not
// keep the decoded OpenAPI document alive.
func TestLoadOpenAPISchema_DoesNotRetainDocument(t *testing.T) {
	t.Parallel()

	t.Run("kubectl's implementation retains the document", func(t *testing.T) {
		t.Parallel()
		// Control case: shows the weak-pointer check detects retention. If this starts failing after a
		// kubectl upgrade, kubectl stopped retaining the document and openAPIResources may be unneeded.
		client := &weakOpenAPIClient{}
		resources, err := openapi.NewOpenAPIParser(client).Parse()
		require.NoError(t, err)
		runtime.GC()
		assert.NotNil(t, client.doc.Value())
		runtime.KeepAlive(resources)
	})

	t.Run("loadOpenAPISchema does not retain the document", func(t *testing.T) {
		t.Parallel()
		client := &weakOpenAPIClient{}
		resources, gvkParser, err := kubectlCmd().loadOpenAPISchema(client)
		require.NoError(t, err)
		runtime.GC()
		assert.Nil(t, client.doc.Value())
		runtime.KeepAlive(resources)
		runtime.KeepAlive(gvkParser)
	})
}
