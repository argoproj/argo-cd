package application

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
)

func TestProcessApplicationListField_SyncOperation(t *testing.T) {
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: &v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
			Revision: "abc",
		}}}},
	}

	res, err := processApplicationListField(&list, map[string]interface{}{"items.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]interface{})
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]interface{})
	require.True(t, ok)
	item := test.ToMap(items[0])

	val, ok, err := unstructured.NestedString(item, "operation", "sync", "revision")
	require.NoError(t, err)
	require.True(t, ok)

	require.Equal(t, "abc", val)
}

func TestProcessApplicationListField_SyncOperationMissing(t *testing.T) {
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Operation: nil}},
	}

	res, err := processApplicationListField(&list, map[string]interface{}{"items.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]interface{})
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]interface{})
	require.True(t, ok)
	item := test.ToMap(items[0])

	_, ok, err = unstructured.NestedString(item, "operation")
	require.NoError(t, err)
	require.False(t, ok)
}
