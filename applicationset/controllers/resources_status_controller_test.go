package controllers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestApplicationStatusChangedPredicate(t *testing.T) {
	p := getApplicationStatusChangedPredicate()
	assert.False(t, p.CreateFunc(event.CreateEvent{}))
	assert.False(t, p.DeleteFunc(event.DeleteEvent{}))
	assert.False(t, p.GenericFunc(event.GenericEvent{}))

	app := func(healthStatus health.HealthStatusCode, syncStatus v1alpha1.SyncStatusCode) *v1alpha1.Application {
		return &v1alpha1.Application{Status: v1alpha1.ApplicationStatus{
			Health: v1alpha1.AppHealthStatus{Status: healthStatus},
			Sync:   v1alpha1.SyncStatus{Status: syncStatus},
		}}
	}
	withRevision := app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced)
	withRevision.Status.Sync.Revision = "abc"
	withSpec := app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced)
	withSpec.Spec.Project = "other"

	for _, tc := range []struct {
		name     string
		oldObj   crtclient.Object
		newObj   crtclient.Object
		expected bool
	}{
		{
			name:     "new application gets its first status",
			oldObj:   app("", ""),
			newObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced),
			expected: true,
		},
		{
			name:     "health changes",
			oldObj:   app(health.HealthStatusProgressing, v1alpha1.SyncStatusCodeSynced),
			newObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced),
			expected: true,
		},
		{
			name:     "sync status changes",
			oldObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced),
			newObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeOutOfSync),
			expected: true,
		},
		{
			name:     "other status fields change",
			oldObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced),
			newObj:   withRevision,
			expected: false,
		},
		{
			name:     "spec changes",
			oldObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced),
			newObj:   withSpec,
			expected: false,
		},
		{
			name:     "old object is not an application",
			oldObj:   &v1alpha1.AppProject{},
			newObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced),
			expected: false,
		},
		{
			name:     "new object is not an application",
			oldObj:   app(health.HealthStatusHealthy, v1alpha1.SyncStatusCodeSynced),
			newObj:   &v1alpha1.AppProject{},
			expected: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, p.UpdateFunc(event.UpdateEvent{ObjectOld: tc.oldObj, ObjectNew: tc.newObj}))
		})
	}
}

func TestReconcileResourcesStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(scheme))

	const namespace = "argocd"
	ownedBy := func(appsetName string) []metav1.OwnerReference {
		return []metav1.OwnerReference{{
			APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Kind:       "ApplicationSet",
			Name:       appsetName,
			UID:        types.UID(appsetName),
			Controller: ptr.To(true),
		}}
	}
	newAppSet := func() *v1alpha1.ApplicationSet {
		return &v1alpha1.ApplicationSet{
			Name: "set", Namespace: namespace, UID: "set",
			Status: v1alpha1.ApplicationSetStatus{
				Resources: []v1alpha1.ResourceStatus{{
					Group:     "argoproj.io",
					Version:   "v1alpha1",
					Kind:      "Application",
					Name:      "app",
					Namespace: namespace,
					Health:    &v1alpha1.HealthStatus{},
				}},
				ResourcesCount: 1,
			},
		}
	}
	healthyApp := func(name, owner string) *v1alpha1.Application {
		return &v1alpha1.Application{
			Kind: "Application", APIVersion: v1alpha1.SchemeGroupVersion.String(),
			Name: name, Namespace: namespace, OwnerReferences: ownedBy(owner),
			Status: v1alpha1.ApplicationStatus{
				Health: v1alpha1.AppHealthStatus{Status: health.HealthStatusHealthy},
				Sync:   v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeSynced},
			},
		}
	}
	newReconciler := func(objs ...crtclient.Object) *ApplicationSetReconciler {
		c := fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(objs...).
			WithStatusSubresource(&v1alpha1.ApplicationSet{}).
			WithIndex(&v1alpha1.Application{}, ".metadata.controller", appControllerIndexer).
			Build()
		return &ApplicationSetReconciler{Client: c, Scheme: scheme}
	}
	req := ctrl.Request{Namespace: namespace, Name: "set"}

	t.Run("copies health and sync status of owned applications", func(t *testing.T) {
		r := newReconciler(newAppSet(), healthyApp("app", "set"), healthyApp("foreign", "other-set"))

		res, err := r.reconcileResourcesStatus(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, res)

		var got v1alpha1.ApplicationSet
		require.NoError(t, r.Get(t.Context(), req.NamespacedName, &got))
		// the fake client drops TypeMeta on list, so group, version and kind are empty here
		require.Len(t, got.Status.Resources, 1)
		assert.Equal(t, "app", got.Status.Resources[0].Name)
		assert.Equal(t, namespace, got.Status.Resources[0].Namespace)
		assert.Equal(t, v1alpha1.SyncStatusCodeSynced, got.Status.Resources[0].Status)
		assert.Equal(t, &v1alpha1.HealthStatus{Status: health.HealthStatusHealthy}, got.Status.Resources[0].Health)
		assert.Equal(t, int64(1), got.Status.ResourcesCount)
	})

	t.Run("ignores an application set that no longer exists", func(t *testing.T) {
		r := newReconciler(healthyApp("app", "set"))

		res, err := r.reconcileResourcesStatus(t.Context(), req)
		require.NoError(t, err)
		assert.Equal(t, ctrl.Result{}, res)
	})

	t.Run("leaves an application set that is being deleted alone", func(t *testing.T) {
		appset := newAppSet()
		appset.DeletionTimestamp = ptr.To(metav1.Now())
		appset.Finalizers = []string{v1alpha1.ResourcesFinalizerName}
		r := newReconciler(appset, healthyApp("app", "set"))

		_, err := r.reconcileResourcesStatus(t.Context(), req)
		require.NoError(t, err)

		var got v1alpha1.ApplicationSet
		require.NoError(t, r.Get(t.Context(), req.NamespacedName, &got))
		assert.Equal(t, newAppSet().Status.Resources, got.Status.Resources)
	})
}
