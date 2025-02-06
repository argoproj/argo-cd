package kube

import (
	_ "embed"
	"encoding/json"
	"testing"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/stretchr/testify/require"

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
		return nil, err
	}
	return document, nil
}
