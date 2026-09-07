package controller

import (
	"fmt"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
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
	// A zero resync period disables expiry-based refresh, so the app stays on the no-refresh path
	// however long the benchmark runs. The default one minute would expire under -benchtime or a
	// profiling run and silently start measuring a full refresh instead.
	ctrl := newFakeControllerWithResync(b.Context(), &fakeData{apps: []runtime.Object{app, proj}}, 0, nil, nil)
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
