package application

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func toMap(obj interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	var res map[string]interface{}
	err = json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

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
	item, err := toMap(items[0])
	require.NoError(t, err)

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
	item, err := toMap(items[0])
	require.NoError(t, err)

	_, ok, err = unstructured.NestedString(item, "operation")
	require.NoError(t, err)
	require.False(t, ok)
}
