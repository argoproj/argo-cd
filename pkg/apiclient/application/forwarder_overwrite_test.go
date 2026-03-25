package application

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
)

func TestProcessApplicationListField_SyncOperation(t *testing.T) {
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
			Revision: "abc",
		}}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]any)
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]any)
	require.True(t, ok)
	item := test.ToMap(items[0])

	val, ok, err := unstructured.NestedString(item, "operation", "sync", "revision")
	require.NoError(t, err)
	require.True(t, ok)

	require.Equal(t, "abc", val)
}

func TestBuildLogFilename(t *testing.T) {
	tests := []struct {
		name      string
		urlPath   string
		podName   string
		container string
		expected  string
	}{
		{
			name:      "all params present",
			urlPath:   "/api/v1/applications/my-app/logs",
			podName:   "my-pod",
			container: "nginx",
			expected:  "my-app_my-pod_nginx.log",
		},
		{
			name:      "missing podName",
			urlPath:   "/api/v1/applications/my-app/logs",
			podName:   "",
			container: "nginx",
			expected:  "my-app_nginx.log",
		},
		{
			name:      "missing container",
			urlPath:   "/api/v1/applications/my-app/logs",
			podName:   "my-pod",
			container: "",
			expected:  "my-app_my-pod.log",
		},
		{
			name:      "only app name",
			urlPath:   "/api/v1/applications/my-app/logs",
			podName:   "",
			container: "",
			expected:  "my-app.log",
		},
		{
			name:      "no valid params",
			urlPath:   "/api/v1/other/endpoint",
			podName:   "",
			container: "",
			expected:  "log.log",
		},
		{
			name:      "pods URL pattern",
			urlPath:   "/api/v1/applications/my-app/pods/my-pod/logs",
			podName:   "my-pod",
			container: "sidecar",
			expected:  "my-app_my-pod_sidecar.log",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queryGet := func(key string) string {
				switch key {
				case "podName":
					return tt.podName
				case "container":
					return tt.container
				default:
					return ""
				}
			}
			result := buildLogFilename(tt.urlPath, queryGet)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestProcessApplicationListField_SyncOperationMissing(t *testing.T) {
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: nil}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]any)
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]any)
	require.True(t, ok)
	item := test.ToMap(items[0])

	_, ok, err = unstructured.NestedString(item, "operation")
	require.NoError(t, err)
	require.False(t, ok)
}
