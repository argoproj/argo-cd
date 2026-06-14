package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	apps "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	appinformer "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions"
)

// TestInformerFilterDoesNotCacheDisallowedNamespaces drives the full
// SharedIndexInformer pipeline (list -> transform -> indexer) and asserts
// that objects from disallowed namespaces never reach the cache, while
// allowed and control-plane namespace objects do. This guards against
// regressions where the transform stops dropping (or starts polluting the
// cache with nil entries).
func TestInformerFilterDoesNotCacheDisallowedNamespaces(t *testing.T) {
	t.Parallel()
	kept := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "kept", Namespace: "team-a"}}
	control := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "ctrl", Namespace: "argocd"}}
	dropped := &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "dropped", Namespace: "team-b"}}

	client := apps.NewSimpleClientset(kept, control, dropped)
	factory := appinformer.NewSharedInformerFactoryWithOptions(client, 0)

	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	require.NoError(t, appInformer.SetTransform(newNamespaceFilterTransform("argocd", []string{"team-a"})))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go appInformer.Run(ctx.Done())

	require.Eventually(t, appInformer.HasSynced, 2*time.Second, 10*time.Millisecond)

	// Wait for the informer to process the list through the FIFO into the
	// indexer. We expect exactly two objects (control-plane + team-a).
	require.NoError(t, wait.PollUntilContextTimeout(ctx, 10*time.Millisecond, 2*time.Second, true, func(_ context.Context) (bool, error) {
		return len(appInformer.GetIndexer().List()) >= 2, nil
	}))

	listed := appInformer.GetIndexer().List()
	gotNS := map[string]bool{}
	for _, obj := range listed {
		app, ok := obj.(*v1alpha1.Application)
		require.True(t, ok, "indexer must only hold non-nil Applications, got %T", obj)
		gotNS[app.Namespace] = true
	}
	assert.True(t, gotNS["argocd"], "control-plane namespace should be cached")
	assert.True(t, gotNS["team-a"], "allowed namespace should be cached")
	assert.False(t, gotNS["team-b"], "disallowed namespace must not be cached")
}
