package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
)

// newSyncedFakeApp returns an app that needs no refresh, with resourceCount entries
// in status.resources.
func newSyncedFakeApp(resourceCount int) *v1alpha1.Application {
	app := newFakeApp()
	app.Status.ReconciledAt = &metav1.Time{Time: time.Now()}
	app.Status.Sync.Status = v1alpha1.SyncStatusCodeSynced
	app.Status.Sync.ComparedTo.Source = app.Spec.GetSource()
	app.Status.Sync.ComparedTo.Destination = app.Spec.Destination
	app.Status.Sync.ComparedTo.IgnoreDifferences = app.Spec.IgnoreDifferences

	resources := make([]v1alpha1.ResourceStatus, resourceCount)
	for i := range resources {
		resources[i] = v1alpha1.ResourceStatus{
			Version:   "v1",
			Kind:      "ConfigMap",
			Namespace: "default",
			Name:      fmt.Sprintf("cm-%d", i),
			Status:    v1alpha1.SyncStatusCodeSynced,
			Health:    &v1alpha1.HealthStatus{Status: health.HealthStatusHealthy},
		}
	}
	app.Status.Resources = resources
	return app
}

func BenchmarkProcessAppRefreshQueueItem_NoRefresh(b *testing.B) {
	app := newSyncedFakeApp(500)
	proj := defaultProj.DeepCopy()
	ctrl := newFakeController(b.Context(), &fakeData{apps: []runtime.Object{app, proj}}, nil)
	key, err := cache.MetaNamespaceKeyFunc(app)
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		ctrl.appRefreshQueue.Add(key)
		ctrl.processAppRefreshQueueItem()
	}
}

func BenchmarkNormalizeApplication(b *testing.B) {
	app := newSyncedFakeApp(500)
	proj := defaultProj.DeepCopy()
	ctrl := newFakeController(b.Context(), &fakeData{apps: []runtime.Object{app, proj}}, nil)

	patches := 0
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (bool, runtime.Object, error) {
		patches++
		return true, &v1alpha1.Application{}, nil
	})

	b.ReportAllocs()
	for b.Loop() {
		ctrl.normalizeApplication(app)
	}
	b.StopTimer()

	// The spec is already normalized, so the benchmark measures the comparison rather than the patch.
	if patches > 0 {
		b.Fatalf("expected no patches, got %d", patches)
	}
}
