package syncwindow

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	listers "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
)

func newFakeLister(objects ...*v1alpha1.SyncWindowResource) listers.SyncWindowResourceLister {
	indexer := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
	for _, obj := range objects {
		_ = indexer.Add(obj)
	}
	return listers.NewSyncWindowResourceLister(indexer)
}

func newSyncWindowResource(name, namespace string, labels map[string]string, windows []v1alpha1.SyncWindowDefinition) *v1alpha1.SyncWindowResource {
	return &v1alpha1.SyncWindowResource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "SyncWindow",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: v1alpha1.SyncWindowResourceSpec{
			Windows: windows,
		},
	}
}

func TestResolveProjectRefs_ByName(t *testing.T) {
	sw := newSyncWindowResource("my-window", "argocd", nil, []v1alpha1.SyncWindowDefinition{
		{
			Kind:         "allow",
			Schedule:     "0 0 * * *",
			Duration:     "1h",
			Applications: []string{"*-prod"},
		},
	})

	lister := newFakeLister(sw)
	resolver := NewResolver(lister, "argocd")

	refs := []v1alpha1.SyncWindowProjectRef{
		{
			Ref: v1alpha1.SyncWindowRef{Name: "my-window"},
		},
	}

	windows, err := resolver.ResolveProjectRefs(refs)
	require.NoError(t, err)
	assert.Len(t, windows, 1)
	assert.Equal(t, "allow", windows[0].Kind)
	assert.Equal(t, "0 0 * * *", windows[0].Schedule)
	assert.Equal(t, "1h", windows[0].Duration)
	assert.Equal(t, []string{"*-prod"}, windows[0].Applications)
}

func TestResolveProjectRefs_WithOverrides(t *testing.T) {
	sw := newSyncWindowResource("my-window", "argocd", nil, []v1alpha1.SyncWindowDefinition{
		{
			Kind:         "deny",
			Schedule:     "0 22 * * *",
			Duration:     "2h",
			Applications: []string{"*"},
			Namespaces:   []string{"default"},
		},
	})

	lister := newFakeLister(sw)
	resolver := NewResolver(lister, "argocd")

	refs := []v1alpha1.SyncWindowProjectRef{
		{
			Ref:          v1alpha1.SyncWindowRef{Name: "my-window"},
			Applications: []string{"prod-*"},
			Clusters:     []string{"cluster1"},
		},
	}

	windows, err := resolver.ResolveProjectRefs(refs)
	require.NoError(t, err)
	assert.Len(t, windows, 1)
	assert.Equal(t, "deny", windows[0].Kind)
	assert.Equal(t, []string{"prod-*"}, windows[0].Applications)
	assert.Equal(t, []string{"cluster1"}, windows[0].Clusters)
	// Namespaces not overridden, so keeps the original
	assert.Equal(t, []string{"default"}, windows[0].Namespaces)
}

func TestResolveProjectRefs_BySelector(t *testing.T) {
	sw1 := newSyncWindowResource("window-1", "argocd", map[string]string{"env": "prod"}, []v1alpha1.SyncWindowDefinition{
		{Kind: "allow", Schedule: "0 0 * * *", Duration: "1h"},
	})
	sw2 := newSyncWindowResource("window-2", "argocd", map[string]string{"env": "prod"}, []v1alpha1.SyncWindowDefinition{
		{Kind: "deny", Schedule: "0 22 * * *", Duration: "2h"},
	})
	sw3 := newSyncWindowResource("window-3", "argocd", map[string]string{"env": "dev"}, []v1alpha1.SyncWindowDefinition{
		{Kind: "allow", Schedule: "0 6 * * *", Duration: "12h"},
	})

	lister := newFakeLister(sw1, sw2, sw3)
	resolver := NewResolver(lister, "argocd")

	refs := []v1alpha1.SyncWindowProjectRef{
		{
			Ref: v1alpha1.SyncWindowRef{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"env": "prod"},
				},
			},
		},
	}

	windows, err := resolver.ResolveProjectRefs(refs)
	require.NoError(t, err)
	assert.Len(t, windows, 2)
}

func TestResolveAppRefs_ClearsFilters(t *testing.T) {
	sw := newSyncWindowResource("my-window", "argocd", nil, []v1alpha1.SyncWindowDefinition{
		{
			Kind:         "allow",
			Schedule:     "0 0 * * *",
			Duration:     "1h",
			Applications: []string{"*-prod"},
			Namespaces:   []string{"default"},
			Clusters:     []string{"cluster1"},
		},
	})

	lister := newFakeLister(sw)
	resolver := NewResolver(lister, "argocd")

	refs := []v1alpha1.SyncWindowRef{
		{Name: "my-window"},
	}

	windows, err := resolver.ResolveAppRefs(refs)
	require.NoError(t, err)
	assert.Len(t, windows, 1)
	assert.Equal(t, "allow", windows[0].Kind)
	// Filters should be cleared for app-level refs
	assert.Nil(t, windows[0].Applications)
	assert.Nil(t, windows[0].Namespaces)
	assert.Nil(t, windows[0].Clusters)
}

func TestResolveRef_NotFound(t *testing.T) {
	lister := newFakeLister()
	resolver := NewResolver(lister, "argocd")

	refs := []v1alpha1.SyncWindowProjectRef{
		{Ref: v1alpha1.SyncWindowRef{Name: "nonexistent"}},
	}

	_, err := resolver.ResolveProjectRefs(refs)
	assert.Error(t, err)
}

func TestResolveRef_EmptyRef(t *testing.T) {
	lister := newFakeLister()
	resolver := NewResolver(lister, "argocd")

	refs := []v1alpha1.SyncWindowProjectRef{
		{Ref: v1alpha1.SyncWindowRef{}},
	}

	_, err := resolver.ResolveProjectRefs(refs)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must specify either name or selector")
}
