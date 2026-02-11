package kube

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"

	"github.com/stretchr/testify/assert"
	"k8s.io/klog/v2/textlogger"

	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
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
	t.Run("newGVKParser gracefully handles duplicate GVKs", func(t *testing.T) {
		client := &fakeOpenAPIClient{}
		_, err := kubectl.newGVKParser(client)
		require.NoError(t, err)
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

	t.Run("preserves build metadata from IBM Cloud", func(t *testing.T) {
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
		assert.Equal(t, "v1.30.11+IKS", serverVersion, "Should preserve IBM Cloud build metadata")
		assert.Contains(t, serverVersion, "+IKS", "Should contain provider-specific metadata")
		assert.NotEqual(t, "1.30", serverVersion, "Should not strip to Major.Minor")
	})

	t.Run("handles various managed Kubernetes versions", func(t *testing.T) {
		testCases := []struct {
			name            string
			major           string
			minor           string
			gitVersion      string
			expectedVersion string
		}{
			{
				name:            "GKE version",
				major:           "1",
				minor:           "29",
				gitVersion:      "v1.29.3-gke.1234567",
				expectedVersion: "v1.29.3-gke.1234567",
			},
			{
				name:            "EKS version",
				major:           "1",
				minor:           "28",
				gitVersion:      "v1.28.5-eks-a123456",
				expectedVersion: "v1.28.5-eks-a123456",
			},
			{
				name:            "AKS version",
				major:           "1",
				minor:           "27",
				gitVersion:      "v1.27.9-hotfix.20240101",
				expectedVersion: "v1.27.9-hotfix.20240101",
			},
			{
				name:            "Standard Kubernetes",
				major:           "1",
				minor:           "26",
				gitVersion:      "v1.26.15",
				expectedVersion: "v1.26.15",
			},
			{
				name:            "Alpha version",
				major:           "1",
				minor:           "31",
				gitVersion:      "v1.31.0-alpha.1",
				expectedVersion: "v1.31.0-alpha.1",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				fakeServer := fakeHTTPServer(version.Info{
					Major:      tc.major,
					Minor:      tc.minor,
					GitVersion: tc.gitVersion,
				}, nil)
				defer fakeServer.Close()
				config := mockConfig(fakeServer.URL)

				serverVersion, err := kubectlCmd().GetServerVersion(config)
				require.NoError(t, err)
				assert.Equal(t, tc.expectedVersion, serverVersion, "Should return full GitVersion for %s", tc.name)
				assert.Regexp(t, `^v\d+\.\d+\.\d+`, serverVersion, "Should match semver pattern")
			})
		}
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
