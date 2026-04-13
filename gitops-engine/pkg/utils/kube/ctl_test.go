package kube

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	openapiclient "k8s.io/client-go/openapi"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2/textlogger"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing"
)

var _ Kubectl = &KubectlCmd{}

func TestConvertToVersion(t *testing.T) {
	kubectl := KubectlCmd{
		Log:    textlogger.NewLogger(textlogger.NewConfig()),
		Tracer: tracing.NopTracer{},
	}
	t.Run("AppsDeployment", func(t *testing.T) {
		newObj, err := kubectl.ConvertToVersion(testingutils.UnstructuredFromFile("testdata/appsdeployment.yaml"), "apps", "v1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "apps", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
		}
	})
	t.Run("CustomResource", func(t *testing.T) {
		_, err := kubectl.ConvertToVersion(testingutils.UnstructuredFromFile("testdata/cr.yaml"), "argoproj.io", "v1")
		assert.Error(t, err)
	})
	t.Run("ExtensionsDeployment", func(t *testing.T) {
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
	t.Run("loadGVKParserV2 gracefully handles duplicate GVKs", func(t *testing.T) {
		client := &fakeOpenAPIClient{}
		_, err := kubectl.loadGVKParserV2(client)
		require.NoError(t, err)
	})
	t.Run("eagerGVKParser reports stats", func(t *testing.T) {
		client := &fakeOpenAPIClient{}
		parser, err := kubectl.loadGVKParserV2(client)
		require.NoError(t, err)

		total, loaded, bytes := parser.Stats()
		assert.Greater(t, total, 0, "should report at least one GV")
		assert.Equal(t, total, loaded, "eager parser loads all GVs")
		assert.Greater(t, bytes, int64(0), "schema bytes should be non-zero")
	})
}

func TestGetServerVersion(t *testing.T) {
	t.Run("returns full semantic version with patch", func(t *testing.T) {
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

// fakeGroupVersion implements openapiclient.GroupVersion for testing.
type fakeGroupVersion struct {
	schemaBytes []byte
	err         error
}

func (f *fakeGroupVersion) Schema(contentType string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.schemaBytes, nil
}

func (f *fakeGroupVersion) ServerRelativeURL() string {
	return ""
}

// validCoreV1Schema is a minimal OpenAPI v3 schema for core/v1 ConfigMap.
var validCoreV1Schema = []byte(`{
  "openapi": "3.0.0",
  "info": {"title": "Kubernetes", "version": "v1.30.0"},
  "paths": {},
  "components": {
    "schemas": {
      "io.k8s.api.core.v1.ConfigMap": {
        "type": "object",
        "x-kubernetes-group-version-kind": [
          {"group": "", "kind": "ConfigMap", "version": "v1"}
        ],
        "properties": {
          "apiVersion": {"type": "string"},
          "kind": {"type": "string"},
          "metadata": {"type": "object"},
          "data": {
            "type": "object",
            "additionalProperties": {"type": "string"}
          }
        }
      }
    }
  }
}`)

// badCRDSchema simulates the Kueue visibility API bug (kubernetes-sigs/kueue#8873):
// a schema with a $ref pointing to a model that doesn't exist in the document.
// In v2, this kind of broken schema poisons the entire cluster's OpenAPI parsing.
var badCRDSchema = []byte(`{
  "openapi": "3.0.0",
  "info": {"title": "Kueue Visibility", "version": "v0.16.0"},
  "paths": {},
  "components": {
    "schemas": {
      "io.x-k8s.sigs.kueue.visibility.v1beta1.PendingWorkloadsSummary": {
        "type": "object",
        "x-kubernetes-group-version-kind": [
          {"group": "visibility.kueue.x-k8s.io", "kind": "PendingWorkloadsSummary", "version": "v1beta1"}
        ],
        "properties": {
          "items": {
            "type": "array",
            "items": {
              "$ref": "#/components/schemas/io.x-k8s.sigs.kueue.visibility.v1beta1.PendingWorkload"
            }
          }
        }
      }
    }
  }
}`)

func TestLazyGVKParser_BadCRDDoesNotAffectOtherResources(t *testing.T) {
	// This test simulates the Kueue visibility API bug (kubernetes-sigs/kueue#8873)
	// where a CRD with an invalid OpenAPI schema (dangling $ref) was installed on
	// the cluster. In the v2 path, this caused the entire OpenAPI schema load to
	// fail, breaking ALL apps — even those with no relation to Kueue.
	//
	// With v3 lazy loading, each GroupVersion is fetched independently on demand.
	// A bad schema in one GV is skipped, and the parser still works for all other GVs.
	paths := map[string]openapiclient.GroupVersion{
		"api/v1": &fakeGroupVersion{schemaBytes: validCoreV1Schema},
		"apis/visibility.kueue.x-k8s.io/v1beta1": &fakeGroupVersion{schemaBytes: badCRDSchema},
	}

	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	// The parser should resolve types from the valid GV (core/v1 ConfigMap)
	configMapType, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	require.NoError(t, err)
	assert.NotNil(t, configMapType, "ConfigMap type should be resolvable from the valid GV")

	// The bad CRD's GV should return an error (dangling ref in schema)
	badType, err := parser.Type(schema.GroupVersionKind{Group: "visibility.kueue.x-k8s.io", Version: "v1beta1", Kind: "PendingWorkloadsSummary"})
	assert.Nil(t, badType, "Bad CRD type should not be resolvable")
	assert.Error(t, err, "Bad CRD should return an error")
}

func TestLazyGVKParser_GarbageBytesInOneGV(t *testing.T) {
	// A GV that returns completely unparseable bytes should be skipped lazily.
	paths := map[string]openapiclient.GroupVersion{
		"api/v1":                    &fakeGroupVersion{schemaBytes: validCoreV1Schema},
		"apis/broken.example.io/v1": &fakeGroupVersion{schemaBytes: []byte(`not valid json at all`)},
	}

	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	configMapType, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	require.NoError(t, err)
	assert.NotNil(t, configMapType, "ConfigMap type should still be resolvable")

	// The broken GV returns an error when accessed
	brokenType, err := parser.Type(schema.GroupVersionKind{Group: "broken.example.io", Version: "v1", Kind: "Foo"})
	assert.Nil(t, brokenType, "Broken GV type should return nil")
	assert.Error(t, err, "Broken GV should return an error")
}

func TestLazyGVKParser_FetchErrorInOneGV(t *testing.T) {
	// A GV that returns a fetch error should be skipped lazily.
	paths := map[string]openapiclient.GroupVersion{
		"api/v1":                      &fakeGroupVersion{schemaBytes: validCoreV1Schema},
		"apis/unavailable.io/v1beta1": &fakeGroupVersion{err: fmt.Errorf("connection refused")},
	}

	parser := newLazyGVKParser(paths, textlogger.NewLogger(textlogger.NewConfig()))

	configMapType, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	require.NoError(t, err)
	assert.NotNil(t, configMapType, "ConfigMap type should still be resolvable")

	// The unavailable GV returns an error when accessed
	unavailableType, err := parser.Type(schema.GroupVersionKind{Group: "unavailable.io", Version: "v1beta1", Kind: "Foo"})
	assert.Nil(t, unavailableType, "Unavailable GV type should return nil")
	assert.Error(t, err, "Unavailable GV should return an error")
}

// validAppsV1Schema is a minimal OpenAPI v3 schema for apps/v1 Deployment.
var validAppsV1Schema = []byte(`{
  "openapi": "3.0.0",
  "info": {"title": "Kubernetes", "version": "v1.30.0"},
  "paths": {},
  "components": {
    "schemas": {
      "io.k8s.api.apps.v1.Deployment": {
        "type": "object",
        "x-kubernetes-group-version-kind": [
          {"group": "apps", "kind": "Deployment", "version": "v1"}
        ],
        "properties": {
          "apiVersion": {"type": "string"},
          "kind": {"type": "string"},
          "metadata": {"type": "object"},
          "spec": {"type": "object"}
        }
      }
    }
  }
}`)

// conversionWebhookErrorSchema simulates what happens when a CRD has multiple
// versions with a conversion webhook that is down or misconfigured. The API
// server may return a valid OpenAPI v3 document for the GV endpoint, but with
// a schema that references types that couldn't be populated because the webhook
// is unavailable. This mirrors the pattern from argoproj/argo-cd#20828.
var conversionWebhookErrorSchema = []byte(`{
  "openapi": "3.0.0",
  "info": {"title": "Example CRD", "version": "v1.0.0"},
  "paths": {},
  "components": {
    "schemas": {
      "io.example.test.v2.Widget": {
        "type": "object",
        "x-kubernetes-group-version-kind": [
          {"group": "test.example.io", "kind": "Widget", "version": "v2"}
        ],
        "properties": {
          "apiVersion": {"type": "string"},
          "kind": {"type": "string"},
          "metadata": {"type": "object"},
          "spec": {
            "$ref": "#/components/schemas/io.example.test.v2.WidgetSpec"
          }
        }
      }
    }
  }
}`)

func TestLazyGVKParser_ConversionWebhookErrorIsolated(t *testing.T) {
	// This test simulates the scenario from argoproj/argo-cd#20828 where a
	// conversion webhook is unavailable. In v2, this error class can cascade
	// and break the entire OpenAPI schema load. With v3 lazy loading, each
	// GroupVersion is fetched independently on demand, so a webhook-related
	// schema failure in one GV does not affect others.
	//
	// Compare with PR #23425 which handles conversion webhook errors at the
	// list/watch (cache sync) layer by tainting GVKs. This test verifies the
	// complementary isolation at the OpenAPI schema parsing layer: even if
	// the schema for a webhook-backed GV is broken or unreachable, the
	// GvkParser still works for all other GVs.
	log := textlogger.NewLogger(textlogger.NewConfig())

	t.Run("webhook down causes schema fetch error", func(t *testing.T) {
		// When the conversion webhook is completely down, the API server's
		// OpenAPI v3 endpoint for that GV may return an HTTP error.
		// In v2, this kind of error from an aggregated API would propagate
		// through the monolithic schema fetch and break everything.
		paths := map[string]openapiclient.GroupVersion{
			"api/v1":                  &fakeGroupVersion{schemaBytes: validCoreV1Schema},
			"apis/apps/v1":            &fakeGroupVersion{schemaBytes: validAppsV1Schema},
			"apis/test.example.io/v2": &fakeGroupVersion{err: fmt.Errorf("Internal error occurred: failed calling webhook \"webhook.test.example.io\": failed to call webhook: Post \"https://webhook-service.default.svc:443/convert\": dial tcp: lookup webhook-service.default.svc: no such host")},
		}

		parser := newLazyGVKParser(paths, log)

		// Both valid GVs should still be fully functional
		configMapType, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
		require.NoError(t, err)
		assert.NotNil(t, configMapType, "ConfigMap should be resolvable")

		deploymentType, err := parser.Type(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		require.NoError(t, err)
		assert.NotNil(t, deploymentType, "Deployment should be resolvable")

		// The webhook-backed GV's type should return an error (it fails lazily)
		widgetType, err := parser.Type(schema.GroupVersionKind{Group: "test.example.io", Version: "v2", Kind: "Widget"})
		assert.Nil(t, widgetType, "Widget type should not be resolvable since its GV failed")
		assert.Error(t, err, "Widget GV should return an error")
	})

	t.Run("webhook returns broken schema with dangling ref", func(t *testing.T) {
		// When the conversion webhook is misconfigured, the API server may
		// serve a schema with dangling $refs (similar to the Kueue bug).
		// The webhook was supposed to populate WidgetSpec but couldn't convert it.
		paths := map[string]openapiclient.GroupVersion{
			"api/v1":                  &fakeGroupVersion{schemaBytes: validCoreV1Schema},
			"apis/apps/v1":            &fakeGroupVersion{schemaBytes: validAppsV1Schema},
			"apis/test.example.io/v2": &fakeGroupVersion{schemaBytes: conversionWebhookErrorSchema},
		}

		parser := newLazyGVKParser(paths, log)

		// Both valid GVs should still be fully functional
		configMapType, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
		require.NoError(t, err)
		assert.NotNil(t, configMapType, "ConfigMap should be resolvable")

		deploymentType, err := parser.Type(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		require.NoError(t, err)
		assert.NotNil(t, deploymentType, "Deployment should be resolvable")
	})

	t.Run("webhook timeout returns 504", func(t *testing.T) {
		// A conversion webhook timeout typically results in an HTTP 504.
		// The schema endpoint returns an error.
		paths := map[string]openapiclient.GroupVersion{
			"api/v1":                  &fakeGroupVersion{schemaBytes: validCoreV1Schema},
			"apis/apps/v1":            &fakeGroupVersion{schemaBytes: validAppsV1Schema},
			"apis/test.example.io/v2": &fakeGroupVersion{err: fmt.Errorf("the server was unable to return a response in the time allotted, but may still be processing the request (get /openapi/v3/apis/test.example.io/v2)")},
		}

		parser := newLazyGVKParser(paths, log)

		configMapType, err := parser.Type(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
		require.NoError(t, err)
		assert.NotNil(t, configMapType, "ConfigMap should be resolvable")

		deploymentType, err := parser.Type(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
		require.NoError(t, err)
		assert.NotNil(t, deploymentType, "Deployment should be resolvable")
	})
}
