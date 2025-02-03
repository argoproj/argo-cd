package kubectl

import (
	"github.com/stretchr/testify/require"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Test_discernGetRequest is adapted from: https://github.com/argoproj/pkg/blob/f5a0a066030558f089fa645dc6546ddc5917bad5/kubeclientmetrics/metric_test.go
func Test_discernGetRequest(t *testing.T) {
	testData := []struct {
		testName string
		url      string
		expected string
	}{
		{
			testName: "Pod LIST",
			url:      "https://127.0.0.1/api/v1/namespaces/default/pods",
			expected: "List",
		},
		{
			testName: "Pod Cluster LIST",
			url:      "https://127.0.0.1/api/v1/pods",
			expected: "List",
		},
		{
			testName: "Pod GET",
			url:      "https://127.0.0.1/api/v1/namespaces/default/pods/pod-name-123456",
			expected: "Get",
		},
		{
			testName: "Namespace LIST",
			url:      "https://127.0.0.1/api/v1/namespaces",
			expected: "List",
		},
		{
			testName: "Namespace GET",
			url:      "https://127.0.0.1/api/v1/namespaces/default",
			expected: "Get",
		},
		{
			testName: "ReplicaSet LIST",
			url:      "https://127.0.0.1/apis/extensions/v1beta1/namespaces/default/replicasets",
			expected: "List",
		},
		{
			testName: "ReplicaSet Cluster LIST",
			url:      "https://127.0.0.1/apis/apps/v1/replicasets",
			expected: "List",
		},
		{
			testName: "ReplicaSet GET",
			url:      "https://127.0.0.1/apis/extensions/v1beta1/namespaces/default/replicasets/rs-abc123",
			expected: "Get",
		},
		{
			testName: "VirtualService LIST",
			url:      "https://127.0.0.1/apis/networking.istio.io/v1alpha3/namespaces/default/virtualservices",
			expected: "List",
		},
		{
			testName: "VirtualService GET",
			url:      "https://127.0.0.1/apis/networking.istio.io/v1alpha3/namespaces/default/virtualservices/virtual-service",
			expected: "Get",
		},
		{
			testName: "ClusterRole LIST",
			url:      "https://127.0.0.1/apis/rbac.authorization.k8s.io/v1/clusterroles",
			expected: "List",
		},
		{
			testName: "ClusterRole Get",
			url:      "https://127.0.0.1/apis/rbac.authorization.k8s.io/v1/clusterroles/argo-rollouts-clusterrole",
			expected: "Get",
		},
		{
			testName: "CRD List",
			url:      "https://127.0.0.1/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions",
			expected: "List",
		},
		{
			testName: "CRD Get",
			url:      "https://127.0.0.1/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions/dummies.argoproj.io",
			expected: "Get",
		},
		{
			testName: "Resource With Periods Get",
			url:      "https://127.0.0.1/apis/argoproj.io/v1alpha1/namespaces/argocd/applications/my-cluster.cluster.k8s.local",
			expected: "Get",
		},
		{
			testName: "Watch cluster resources",
			url:      "https://127.0.0.1/api/v1/namespaces?resourceVersion=343003&watch=true",
			expected: "Watch",
		},
		{
			testName: "Watch single cluster resource",
			url:      "https://127.0.0.1/api/v1/namespaces?fieldSelector=metadata.name%3Ddefault&resourceVersion=0&watch=true",
			expected: "Watch",
		},
		{
			testName: "Watch namespace resources",
			url:      "https://127.0.0.1/api/v1/namespaces/kube-system/serviceaccounts?resourceVersion=343091&watch=true",
			expected: "Watch",
		},
		{
			testName: "Watch single namespace resource",
			url:      "https://127.0.0.1/api/v1/namespaces/kube-system/serviceaccounts?fieldSelector=metadata.name%3Ddefault&resourceVersion=0&watch=true",
			expected: "Watch",
		},
		// Not yet supported
		// {
		// 	testName: "Non resource request",
		// 	url:      "https://127.0.0.1/apis/apiextensions.k8s.io/v1beta1",
		// 	expected: "Get",
		// },
	}

	for _, td := range testData {
		t.Run(td.testName, func(t *testing.T) {
			u, err := url.Parse(td.url)
			require.NoError(t, err)
			info := discernGetRequest(*u)
			assert.Equal(t, td.expected, info)
		})
	}
}
