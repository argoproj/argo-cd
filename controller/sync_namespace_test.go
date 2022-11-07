package controller

import (
	"errors"
	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

type fakeResourceTracking struct {
}

func (f fakeResourceTracking) GetAppName(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) string {
	panic("implement me")
}

func (f fakeResourceTracking) GetAppInstance(un *unstructured.Unstructured, key string, trackingMethod v1alpha1.TrackingMethod) *argo.AppInstanceValue {
	return nil
}

func (f fakeResourceTracking) SetAppInstance(un *unstructured.Unstructured, key, val, namespace string, trackingMethod v1alpha1.TrackingMethod) error {
	return errors.New("some error")
}

func (f fakeResourceTracking) BuildAppInstanceValue(value argo.AppInstanceValue) string {
	panic("implement me")
}

func (f fakeResourceTracking) ParseAppInstanceValue(value string) (*argo.AppInstanceValue, error) {
	panic("implement me")
}

func (f fakeResourceTracking) Normalize(config, live *unstructured.Unstructured, labelKey, trackingMethod string) error {
	panic("implement me")
}

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
		un                  *unstructured.Unstructured
		expected            bool
		expectedLabels      map[string]string
		expectedAnnotations map[string]string
	}{
		{
			name:       "un is nil and syncPolicy is nil",
			expected:   false,
			un:         nil,
			syncPolicy: nil,
		},
		{
			name:     "un is nil and syncPolicy is not nil",
			expected: false,
			un:       nil,
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: nil,
			},
		},
		{
			name:                "un is nil and syncPolicy has labels and annotations",
			expected:            false,
			un:                  nil,
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
			name:           "namespace does not yet exist and managedNamespaceMetadata nil",
			expected:       true,
			expectedLabels: map[string]string{},
			//expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace"},
			expectedAnnotations: map[string]string{},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: nil,
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata not nil",
			expected:            true,
			expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has empty labels map",
			expected:            true,
			expectedLabels:      map[string]string{},
			expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels: map[string]string{},
				},
			},
		},
		{
			name:                "namespace does not yet exist and managedNamespaceMetadata has empty annotations map",
			expected:            true,
			expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("", "", map[string]string{}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("something", "1", map[string]string{}, map[string]string{}),
			syncPolicy: &v1alpha1.SyncPolicy{
				ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
					Labels: map[string]string{"my-cool-label": "some-value"},
				},
			},
		},
		{
			name:                "namespace exists with no labels or annotations and managedNamespaceMetadata has annotations",
			expected:            true,
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("something", "1", map[string]string{}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("something", "1", map[string]string{}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			expectedLabels:      map[string]string{"my-cool-label": "some-value", "my-other-label": "some-other-value"},
			un:                  createFakeNamespace("something", "1", map[string]string{"my-cool-label": "some-value"}, map[string]string{}),
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
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("something", "1", map[string]string{}, map[string]string{"my-cool-annotation": "some-value", "my-other-annotation": "some-other-value"}),
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
			expectedAnnotations: map[string]string{"my-cool-annotation": "some-value", "my-other-annotation": "some-other-value", "argocd.argoproj.io/tracking-id": "some-app:/Namespace:/some-namespace", "argocd.argoproj.io/sync-options": "ServerSideApply=true"},
			un:                  createFakeNamespace("something", "1", map[string]string{"my-cool-label": "some-value"}, map[string]string{"my-cool-annotation": "some-value"}),
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
			actual, err := syncNamespace(argo.NewResourceTracking(), common.LabelKeyAppInstance, argo.TrackingMethodAnnotation, "some-app", tt.syncPolicy)(tt.un)
			assert.NoError(t, err)

			if tt.un != nil {
				assert.Equal(t, tt.expectedLabels, tt.un.GetLabels())
				assert.Equal(t, tt.expectedAnnotations, tt.un.GetAnnotations())
			}

			assert.Equalf(t, tt.expected, actual, "syncNamespace(%v)", tt.syncPolicy)
		})
	}
}

func Test_shouldNamespaceSync_Failure(t *testing.T) {
	fake := fakeResourceTracking{}
	_, err := syncNamespace(fake, common.LabelKeyAppInstance, argo.TrackingMethodAnnotation, "some-app", &v1alpha1.SyncPolicy{
		ManagedNamespaceMetadata: &v1alpha1.ManagedNamespaceMetadata{
			Labels:      map[string]string{"my-cool-label": "some-value"},
			Annotations: map[string]string{"my-cool-annotation": "some-value"},
		},
	})(createFakeNamespace("something", "1", map[string]string{}, map[string]string{}))
	assert.Error(t, err, "Expected error")
	assert.Equal(t, "failed to set app instance tracking on the namespace some-namespace: some error", err.Error())
}
