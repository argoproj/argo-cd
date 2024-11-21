package admin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
)

func TestGetAdditionalNamespaces(t *testing.T) {
	createArgoCDCmdCMWithKeys := func(data map[string]interface{}) *unstructured.Unstructured {
		return &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]interface{}{
					"name":      "argocd-cmd-params-cm",
					"namespace": "argocd",
				},
				"data": data,
			},
		}
	}

	testCases := []struct {
		CmdParamsKeys map[string]interface{}
		expected      argocdAdditonalNamespaces
		description   string
	}{
		{
			description:   "empty configmap should return no additional namespaces",
			CmdParamsKeys: map[string]interface{}{},
			expected:      argocdAdditonalNamespaces{applicationNamespaces: []string{}, applicationsetNamespaces: []string{}},
		},
		{
			description:   "empty strings in respective keys in cm shoud return empty namespace list",
			CmdParamsKeys: map[string]interface{}{applicationsetNamespacesCmdParamsKey: "", applicationNamespacesCmdParamsKey: ""},
			expected:      argocdAdditonalNamespaces{applicationNamespaces: []string{}, applicationsetNamespaces: []string{}},
		},
		{
			description:   "when only one of the keys in the cm is set only correct respective list of namespaces should be returned",
			CmdParamsKeys: map[string]interface{}{applicationNamespacesCmdParamsKey: "foo, bar*"},
			expected:      argocdAdditonalNamespaces{applicationsetNamespaces: []string{}, applicationNamespaces: []string{"foo", "bar*"}},
		},
		{
			description:   "when only one of the keys in the cm is set only correct respective list of namespaces should be returned",
			CmdParamsKeys: map[string]interface{}{applicationsetNamespacesCmdParamsKey: "foo, bar*"},
			expected:      argocdAdditonalNamespaces{applicationNamespaces: []string{}, applicationsetNamespaces: []string{"foo", "bar*"}},
		},
		{
			description:   "whitespaces are removed for both multiple and single namespace",
			CmdParamsKeys: map[string]interface{}{applicationNamespacesCmdParamsKey: "  bar    ", applicationsetNamespacesCmdParamsKey: " foo , bar*  "},
			expected:      argocdAdditonalNamespaces{applicationNamespaces: []string{"bar"}, applicationsetNamespaces: []string{"foo", "bar*"}},
		},
	}

	for _, c := range testCases {
		fakeDynClient := dynfake.NewSimpleDynamicClient(runtime.NewScheme(), createArgoCDCmdCMWithKeys(c.CmdParamsKeys))

		argoCDClientsets := &argoCDClientsets{
			configMaps:      fakeDynClient.Resource(configMapResource).Namespace("argocd"),
			applications:    fakeDynClient.Resource(schema.GroupVersionResource{}),
			applicationSets: fakeDynClient.Resource(schema.GroupVersionResource{}),
			secrets:         fakeDynClient.Resource(schema.GroupVersionResource{}),
			projects:        fakeDynClient.Resource(schema.GroupVersionResource{}),
		}

		result := getAdditionalNamespaces(context.TODO(), argoCDClientsets)
		assert.Equal(t, c.expected, *result)
	}
}
