package application

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
)

func TestProcessApplicationListField_SyncOperation(t *testing.T) {
	t.Parallel()
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

func TestProcessApplicationListField_SyncOperationMissing(t *testing.T) {
	t.Parallel()
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

func TestProcessApplicationListField_OperationStateOperationSync(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{
				Operation: v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
					Revision:  "abc",
					Resources: []v1alpha1.SyncOperationResource{{Group: "apps", Kind: "Deployment", Name: "web"}},
					Manifests: []string{"apiVersion: v1\nkind: ConfigMap"},
				}},
				SyncResult: &v1alpha1.SyncOperationResult{Revision: "def"},
			},
		}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.status.operationState.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]any)
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]any)
	require.True(t, ok)
	item := test.ToMap(items[0])

	syncMap, ok, err := unstructured.NestedMap(item, "status", "operationState", "operation", "sync")
	require.NoError(t, err)
	require.True(t, ok)
	// The field is a pure presence marker: it must be an empty object, with none
	// of the operation's payload (revision, resources, manifests, sources).
	require.Empty(t, syncMap)
}

func TestProcessApplicationListField_OperationStateOperationSyncMultiSource(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{
				Operation: v1alpha1.Operation{Sync: &v1alpha1.SyncOperation{
					Revisions: []string{"abc", "def"},
					Sources: v1alpha1.ApplicationSources{
						{RepoURL: "https://example.com/repo1.git"},
						{RepoURL: "https://example.com/repo2.git"},
					},
				}},
			},
		}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.status.operationState.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]any)
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]any)
	require.True(t, ok)
	item := test.ToMap(items[0])

	syncMap, ok, err := unstructured.NestedMap(item, "status", "operationState", "operation", "sync")
	require.NoError(t, err)
	require.True(t, ok)
	// Multi-source operations get the same empty presence marker; their sources
	// and revisions must not leak into the list payload.
	require.Empty(t, syncMap)
}

func TestProcessApplicationListField_OperationStateOperationSyncMissing(t *testing.T) {
	t.Parallel()
	list := v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{{Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{Operation: v1alpha1.Operation{Sync: nil}},
		}}},
	}

	res, err := processApplicationListField(&list, map[string]any{"items.status.operationState.operation.sync": true}, false)
	require.NoError(t, err)
	resMap, ok := res.(map[string]any)
	require.True(t, ok)

	items, ok := resMap["items"].([]map[string]any)
	require.True(t, ok)
	item := test.ToMap(items[0])

	_, ok, err = unstructured.NestedMap(item, "status")
	require.NoError(t, err)
	require.False(t, ok)
}
