package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func createFakeNamespace(uid string, resourceVersion string, labels map[string]string, annotations map[string]string) *unstructured.Unstructured {
	un := unstructured.Unstructured{}
	un.SetUID(types.UID(uid))
	un.SetResourceVersion(resourceVersion)
	un.SetLabels(labels)
	un.SetAnnotations(annotations)
	un.SetKind("Namespace")
	un.SetName("some-namespace")
	return &un
}

func Test_shouldNamespaceSync(t *testing.T) {
	tests := []struct {
		name                string
		syncPolicy          *v1alpha1.SyncPolicy
		managedNs           *unstructured.Unstructured
		liveNs              *unstructured.Unstructured
		expected            bool
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name:       "liveNs is nil and syncPolicy is nil",
			expected:   false,
			managedNs:  nil,
			liveNs:     nil,
			syncPolicy: nil,
		},
		{
			name:      "liveNs is nil and syncPolicy is not nil",
			expected:  false,
			managedNs: nil,
			liveNs:    nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: nil,
			},
		},
		{
			name:                "liveNs is nil and syncPolicy has labels and annotations",
			expected:            false,
			managedNs:           nil,
			liveNs:              nil,
			expectedLabels:      map[string]string{"my-cool-label": "some-value"},
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value"},
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{"my-cool-label": "some-value"},
					Annotations: map[string]string{"my-cool-annotation": "some-value"},
				},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata nil",
			expected:            true,
			expectedLabels:      map[string]string{},
			expectedAnnotations: map[string]string{},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: nil,
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata not nil",
			expected:            true,
			expectedAnnotations: map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has empty labels map",
			expected:            true,
			expectedLabels:      map[string]string{},
			expectedAnnotations: map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels: map[string]string{},
				},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has empty annotations map",
			expected:            true,
			expectedAnnotations: map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Annotations: map[string]string{},
				},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has empty annotations and labels map",
			expected:            true,
			expectedLabels:      map[string]string{},
			expectedAnnotations: map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{},
					Annotations: map[string]string{},
				},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has labels",
			expected:            true,
			expectedLabels:      map[string]string{"my-cool-label": "some-value"},
			expectedAnnotations: map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{"my-cool-label": "some-value"},
					Annotations: nil,
				},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has annotations",
			expected:            true,
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      nil,
					Annotations: map[string]string{"my-cool-annotation": "some-value"},
				},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has annotations and labels",
			expected:            true,
			expectedLabels:      map[string]string{"my-cool-label": "some-value"},
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{"my-cool-label": "some-value"},
					Annotations: map[string]string{"my-cool-annotation": "some-value"},
				},
			},
		},
		{
			name:                "namespace exists with no labels or annotations and managedNamespaceMetadata has labels",
			expected:            true,
			expectedLabels:      map[string]string{"my-cool-label": "some-value"},
			expectedAnnotations: map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              createFakeNamespace("something", "1", map[string]string{}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels: map[string]string{"my-cool-label": "some-value"},
				},
			},
		},
		{
			name:                "namespace exists with no labels or annotations and managedNamespaceMetadata has annotations",
			expected:            true,
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              createFakeNamespace("something", "1", map[string]string{}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Annotations: map[string]string{"my-cool-annotation": "some-value"},
				},
			},
		},
		{
			name:                "namespace exists with no labels or annotations and managedNamespaceMetadata has annotations and labels",
			expected:            true,
			expectedLabels:      map[string]string{"my-cool-label": "some-value"},
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              createFakeNamespace("something", "1", map[string]string{}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{"my-cool-label": "some-value"},
					Annotations: map[string]string{"my-cool-annotation": "some-value"},
				},
			},
		},
		{
			name:                "namespace exists with labels and managedNamespaceMetadata has mismatching labels",
			expected:            true,
			expectedAnnotations: map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			expectedLabels:      map[string]string{"my-cool-label": "some-value", "my-other-label": "some-other-value"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              createFakeNamespace("something", "1", map[string]string{"my-cool-label": "some-value"}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{"my-cool-label": "some-value", "my-other-label": "some-other-value"},
					Annotations: map[string]string{},
				},
			},
		},
		{
			name:                "namespace exists with annotations and managedNamespaceMetadata has mismatching annotations",
			expected:            true,
			expectedLabels:      map[string]string{},
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              createFakeNamespace("something", "1", map[string]string{}, map[string]string{"my-cool-annotation": "some-value", "my-other-annotation": "some-other-value"}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{},
					Annotations: map[string]string{"my-cool-annotation": "some-value"},
				},
			},
		},
		{
			name:                "namespace exists with annotations and labels managedNamespaceMetadata has mismatching annotations and labels",
			expected:            true,
			expectedLabels:      map[string]string{"my-cool-label": "some-value", "my-other-label": "some-other-value"},
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "my-other-annotation": "some-other-value", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			managedNs:           createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			liveNs:              createFakeNamespace("something", "1", map[string]string{"my-cool-label": "some-value"}, map[string]string{"my-cool-annotation": "some-value"}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels:      map[string]string{"my-cool-label": "some-value", "my-other-label": "some-other-value"},
					Annotations: map[string]string{"my-cool-annotation": "some-value", "my-other-annotation": "some-other-value"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual, err := syncNamespace(tt.syncPolicy)(tt.managedNs, tt.liveNs)
			require.NoError(t, err)

			if tt.managedNs != nil {
				assert.Equal(t, tt.expectedLabels, tt.managedNs.GetLabels())
				assert.Equal(t, tt.expectedAnnotations, tt.managedNs.GetAnnotations())
			}

			assert.Equalf(t, tt.expected, actual, "syncNamespace(%v)", tt.syncPolicy)
		})
	}
}
