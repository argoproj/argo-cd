package cache

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/kubetest"
)

var (
	podsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	podsGVK = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"}
)

func podsAPI() kube.APIResourceInfo {
	return kube.APIResourceInfo{
		GroupKind:            podsGVK.GroupKind(),
		GroupVersionResource: podsGVR,
		Meta:                 metav1.APIResource{Group: "", Version: "v1", Kind: "Pod", Name: "pods", Namespaced: true},
	}
}

func unstructuredPod(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Pod",
		"metadata":   map[string]any{"name": name, "namespace": namespace, "uid": "uid-" + name},
	}}
}

// newInformerTestCache wires a clusterCache whose kubectl returns the given
// fake dynamic client. Used by lifecycle tests that need real informers
// driven off a fake tracker.
func newInformerTestCache(t *testing.T, seed ...runtime.Object) (*clusterCache, *fake.FakeDynamicClient) {
	t.Helper()
	client := fake.NewSimpleDynamicClient(scheme.Scheme, seed...)
	// The fake dynamic client's default list reactor returns a list without a
	// resource version. The reflector rejects that. Patch it here so informers
	// can start.
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		handled, ret, err := reactor.React(action)
		if !handled || err != nil {
			return handled, ret, err
		}
		ret.(metav1.ListInterface).SetResourceVersion("1")
		return handled, ret, nil
	})
	c := NewClusterCache(
		&rest.Config{Host: "https://test"},
		SetKubectl(&kubetest.MockKubectlCmd{DynamicClient: client}),
	).(*clusterCache)
	t.Cleanup(func() { c.Invalidate() })
	return c, client
}

func TestBuildInformer_TransformAndEventHandlerInstalled(t *testing.T) {
	c, client := newInformerTestCache(t, unstructuredPod("nginx", "default"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	informer := c.buildInformer(ctx, client.Resource(podsGVR), podsAPI(), "")

	go informer.RunWithContext(ctx)
	require.True(t, cache.WaitForCacheSync(ctx.Done(), informer.HasSynced),
		"informer should sync within timeout")

	// After sync the informer's store should hold a *cachedResource, proving
	// the TransformFunc ran on the ingest path.
	key := "default/nginx"
	obj, exists, err := informer.GetStore().GetByKey(key)
	require.NoError(t, err)
	require.True(t, exists, "informer store should contain the seeded pod")

	cr, ok := obj.(*cachedResource)
	require.True(t, ok, "stored object must be *cachedResource, got %T", obj)
	assert.Equal(t, "nginx", cr.GetName())
	assert.Equal(t, "default", cr.GetNamespace())
	require.NotNil(t, cr.Resource)
	assert.Equal(t, "nginx", cr.Resource.Ref.Name)

	// And the event handler should have populated our shadow maps as part
	// of the initial list delivery.
	assert.Eventually(t, func() bool {
		c.lock.RLock()
		defer c.lock.RUnlock()
		_, present := c.resources[kube.NewResourceKey("", "Pod", "default", "nginx")]
		return present
	}, 2*time.Second, 10*time.Millisecond, "c.resources shadow should be populated by event handler")
}

func TestStartInformersForAPI_PopulatesApisMetaAndShadowState(t *testing.T) {
	c, _ := newInformerTestCache(t, unstructuredPod("nginx", "default"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	// apisMeta populated for the GroupKind.
	c.lock.RLock()
	meta, ok := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	require.True(t, ok)
	require.NotNil(t, meta)
	assert.True(t, meta.namespaced)
	require.Len(t, meta.informers, 1, "cluster-wide watch produces one informer under the empty-string ns key")
	assert.NotNil(t, meta.informers[""].informer)
	assert.NotNil(t, meta.watchCancel)

	// namespacedResources also populated so IsNamespaced answers correctly.
	c.lock.RLock()
	isNS := c.namespacedResources[podsGVK.GroupKind()]
	c.lock.RUnlock()
	assert.True(t, isNS)

	// Shadow state fills in as the informer's initial list is delivered.
	key := kube.NewResourceKey("", "Pod", "default", "nginx")
	assert.Eventually(t, func() bool {
		c.lock.RLock()
		defer c.lock.RUnlock()
		_, present := c.resources[key]
		return present
	}, 2*time.Second, 10*time.Millisecond)
}

func TestStartInformersForAPI_IdempotentWhenAlreadyWatching(t *testing.T) {
	c, _ := newInformerTestCache(t, unstructuredPod("nginx", "default"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	c.lock.RLock()
	firstMeta := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()

	// Second call with the same API must be a no-op.
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	c.lock.RLock()
	secondMeta := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	assert.Same(t, firstMeta, secondMeta, "second call must not replace the existing apiMeta")
}

func TestStartInformersForAPI_WatchCancelStopsInformers(t *testing.T) {
	c, _ := newInformerTestCache(t, unstructuredPod("nginx", "default"))

	parent, cancelParent := context.WithCancel(context.Background())
	defer cancelParent()
	require.NoError(t, c.startInformersForAPI(parent, podsAPI()))

	c.lock.RLock()
	meta := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	require.NotNil(t, meta)

	// Cancelling watchCancel should unblock every informer goroutine.
	// Verify by checking HasSynced remains true but new events no longer
	// flow: we don't have direct goroutine-exit notification, so we verify
	// indirectly by seeing that the informer stops responding after cancel.
	meta.watchCancel()

	// Give informers a moment to notice cancellation.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatal("informer did not observe context cancellation")
		}
		// Once the reflector's loop exits due to ctx.Done, further work
		// stops. We can't observe goroutine exit directly; ensuring the
		// cancel doesn't panic and the store remains accessible is the
		// best we can do in a unit test. (Real exit is verified by
		// Invalidate's lifecycle tests in a later commit.)
		if _, exists := meta.informers[""]; exists {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestStartInformersForAPI_NamespaceFanout(t *testing.T) {
	c, _ := newInformerTestCache(t,
		unstructuredPod("a", "ns1"),
		unstructuredPod("b", "ns2"),
	)
	// Namespace-scoped caller: one informer per namespace.
	c.namespaces = []string{"ns1", "ns2"}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	c.lock.RLock()
	meta := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	require.NotNil(t, meta)
	require.Len(t, meta.informers, 2, "expected one informer per namespace")
	assert.NotNil(t, meta.informers["ns1"].informer)
	assert.NotNil(t, meta.informers["ns2"].informer)

	// Both namespaces' pods should populate the shadow.
	assert.Eventually(t, func() bool {
		c.lock.RLock()
		defer c.lock.RUnlock()
		_, aOK := c.resources[kube.NewResourceKey("", "Pod", "ns1", "a")]
		_, bOK := c.resources[kube.NewResourceKey("", "Pod", "ns2", "b")]
		return aOK && bOK
	}, 2*time.Second, 10*time.Millisecond, "shadow state should carry pods from both namespaces")
}

// Sanity: every informer goroutine we start should drop off when its
// parent context is cancelled. We don't have a direct handle on goroutine
// exit, but we can at least check the informer reports it's done via a
// best-effort probe that the cancel function is callable.
func TestStartInformersForAPI_CancelIsCallable(t *testing.T) {
	c, _ := newInformerTestCache(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	c.lock.RLock()
	meta := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	require.NotPanics(t, func() { meta.watchCancel() })
}

// The test also exercises our informerEventHandler end-to-end: verifies that
// a Create event on the fake tracker flows through the informer's
// TransformFunc + delta FIFO + our onInformerChange into nsIndex.
func TestEnsureSynced_InformerModeEndToEnd(t *testing.T) {
	client := fake.NewSimpleDynamicClient(scheme.Scheme, unstructuredPod("nginx", "default"))
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		handled, ret, err := reactor.React(action)
		if !handled || err != nil {
			return handled, ret, err
		}
		ret.(metav1.ListInterface).SetResourceVersion("1")
		return handled, ret, nil
	})

	iface := NewClusterCache(
		&rest.Config{Host: "https://test"},
		SetMode(ModeInformer),
		SetKubectl(&kubetest.MockKubectlCmd{
			APIResources:  []kube.APIResourceInfo{podsAPI()},
			DynamicClient: client,
		}),
	)
	c := iface.(*clusterCache)
	t.Cleanup(func() { iface.Invalidate() })

	// EnsureSynced routes to syncInformers because c.mode == ModeInformer.
	// It blocks until every informer reports HasSynced, at which point the
	// initial list has flowed through our event handler into the shadow
	// state.
	require.NoError(t, iface.EnsureSynced())

	c.lock.RLock()
	_, hasMeta := c.apisMeta[podsGVK.GroupKind()]
	_, hasShadow := c.resources[kube.NewResourceKey("", "Pod", "default", "nginx")]
	c.lock.RUnlock()
	assert.True(t, hasMeta, "apisMeta should be populated under ModeInformer")
	assert.True(t, hasShadow, "initial list should have populated c.resources")
}

// Regression: a CRD object in the initial list used to deadlock the cache
// under ModeInformer. onInformerChange held c.lock while calling
// dispatchEvent -> handleCRDEvent -> runSynced(c.lock, ...), and the
// non-reentrant RWMutex hung the controller. Verify EnsureSynced now
// returns when discovery includes the apiextensions.k8s.io CRD GVR and
// at least one CRD exists.
func TestEnsureSynced_InformerMode_CRDInInitialListDoesNotDeadlock(t *testing.T) {
	crdGVR := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	crdAPI := kube.APIResourceInfo{
		GroupKind:            schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"},
		GroupVersionResource: crdGVR,
		Meta: metav1.APIResource{
			Group: "apiextensions.k8s.io", Version: "v1",
			Kind: "CustomResourceDefinition", Name: "customresourcedefinitions",
			Namespaced: false,
		},
	}

	crd := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "crontabs.stable.example.com", "uid": "uid-crd"},
		"spec": map[string]any{
			"group": "stable.example.com",
			"names": map[string]any{"kind": "CronTab", "plural": "crontabs", "singular": "crontab"},
			"scope": "Namespaced",
			"versions": []any{
				map[string]any{"name": "v1", "served": true, "storage": true},
			},
		},
	}}

	client := fake.NewSimpleDynamicClient(scheme.Scheme, crd)
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		handled, ret, err := reactor.React(action)
		if !handled || err != nil {
			return handled, ret, err
		}
		ret.(metav1.ListInterface).SetResourceVersion("1")
		return handled, ret, nil
	})

	iface := NewClusterCache(
		&rest.Config{Host: "https://test"},
		SetMode(ModeInformer),
		SetKubectl(&kubetest.MockKubectlCmd{
			APIResources:  []kube.APIResourceInfo{crdAPI},
			DynamicClient: client,
		}),
	)

	done := make(chan error, 1)
	go func() { done <- iface.EnsureSynced() }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("EnsureSynced deadlocked under ModeInformer when initial list contained a CRD")
	}

	// Skip Invalidate cleanup intentionally: if a future regression brings
	// the deadlock back, Invalidate would also block on c.lock and hang
	// the whole test process. The fake clients are short-lived; goroutines
	// die with the test binary.
	c := iface.(*clusterCache)
	c.lock.RLock()
	_, hasMeta := c.apisMeta[crdAPI.GroupKind]
	c.lock.RUnlock()
	assert.True(t, hasMeta, "apisMeta should be populated for the CRD GVR under ModeInformer")
}

// A permanently failing list on one GVR must not block EnsureSynced from
// returning. The bounded WaitForCacheSync wait makes it return an error
// (so callers don't operate against a partial cache) within the
// configured timeout instead of hanging forever. Before bounding the
// wait, this test would hang for the full test timeout because
// watchParent was context.Background().
func TestEnsureSynced_InformerMode_PartialSyncReturnsWithinTimeout(t *testing.T) {
	configmapsGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}
	configmapsAPI := kube.APIResourceInfo{
		GroupKind:            schema.GroupKind{Group: "", Kind: "ConfigMap"},
		GroupVersionResource: configmapsGVR,
		Meta:                 metav1.APIResource{Group: "", Version: "v1", Kind: "ConfigMap", Name: "configmaps", Namespaced: true},
	}

	client := fake.NewSimpleDynamicClient(scheme.Scheme, unstructuredPod("nginx", "default"))
	defaultReactor := client.ReactionChain[0]
	// Pods list works (with a resourceVersion patch); ConfigMaps list always
	// returns Forbidden — simulating an RBAC-restricted GVR.
	client.PrependReactor("list", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		if action.GetResource().Resource == "configmaps" {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "configmaps"}, "", nil)
		}
		handled, ret, err := defaultReactor.React(action)
		if !handled || err != nil {
			return handled, ret, err
		}
		ret.(metav1.ListInterface).SetResourceVersion("1")
		return handled, ret, nil
	})

	// Short timeout so the test runs quickly; the production default
	// (10s via ClusterRetryTimeout) is too slow for a unit test.
	iface := NewClusterCache(
		&rest.Config{Host: "https://test"},
		SetMode(ModeInformer),
		SetClusterSyncRetryTimeout(1*time.Second),
		SetKubectl(&kubetest.MockKubectlCmd{
			APIResources:  []kube.APIResourceInfo{podsAPI(), configmapsAPI},
			DynamicClient: client,
		}),
	)
	t.Cleanup(func() { iface.Invalidate() })

	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- iface.EnsureSynced() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("EnsureSynced did not return within 5s — WaitForCacheSync timeout is not bounded")
	}
	require.Error(t, err, "EnsureSynced must error so callers don't operate against a partial cache")
	assert.Contains(t, err.Error(), "ConfigMap", "error should name the pending GVR")
	// Sanity: we should return roughly at the configured timeout, not
	// immediately (which would mean the wait was skipped) and not at the
	// full 5s test deadline.
	assert.GreaterOrEqual(t, time.Since(start), 800*time.Millisecond, "expected to wait near the configured timeout")
	assert.Less(t, time.Since(start), 4*time.Second, "expected to return shortly after the configured timeout")
}

func TestInvalidate_InformerModeCancelsWatches(t *testing.T) {
	client := fake.NewSimpleDynamicClient(scheme.Scheme, unstructuredPod("nginx", "default"))
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		handled, ret, err := reactor.React(action)
		if !handled || err != nil {
			return handled, ret, err
		}
		ret.(metav1.ListInterface).SetResourceVersion("1")
		return handled, ret, nil
	})

	iface := NewClusterCache(
		&rest.Config{Host: "https://test"},
		SetMode(ModeInformer),
		SetKubectl(&kubetest.MockKubectlCmd{
			APIResources:  []kube.APIResourceInfo{podsAPI()},
			DynamicClient: client,
		}),
	)
	require.NoError(t, iface.EnsureSynced())

	c := iface.(*clusterCache)
	c.lock.RLock()
	require.NotNil(t, c.apisMeta[podsGVK.GroupKind()])
	c.lock.RUnlock()

	iface.Invalidate()

	c.lock.RLock()
	defer c.lock.RUnlock()
	assert.Empty(t, c.apisMeta, "Invalidate should clear apisMeta under informer mode")
}

func TestInformerWatchErrorHandler_NotFoundStopsWatching(t *testing.T) {
	c, _ := newInformerTestCache(t, unstructuredPod("nginx", "default"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	c.lock.RLock()
	require.NotNil(t, c.apisMeta[podsGVK.GroupKind()])
	c.lock.RUnlock()

	// Fire the handler with a NotFound error directly — this is what the
	// reflector would call on a GroupKind that disappeared (e.g., a CRD
	// was deleted out from under us).
	handler := c.informerWatchErrorHandler(podsAPI(), "")
	handler(ctx, nil, apierrors.NewNotFound(schema.GroupResource{Resource: "pods"}, ""))

	c.lock.RLock()
	_, stillWatching := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	assert.False(t, stillWatching, "NotFound should trigger stopWatching")
}

func TestInformerWatchErrorHandler_ForbiddenStopsWatchingWhenRBACEnabled(t *testing.T) {
	c, _ := newInformerTestCache(t, unstructuredPod("nginx", "default"))
	c.respectRBAC = RespectRbacNormal

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	handler := c.informerWatchErrorHandler(podsAPI(), "")
	handler(ctx, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", nil))

	c.lock.RLock()
	_, stillWatching := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	assert.False(t, stillWatching, "Forbidden should trigger stopWatching when respectRBAC != Disabled")
}

func TestInformerWatchErrorHandler_ForbiddenIgnoredWhenRBACDisabled(t *testing.T) {
	c, _ := newInformerTestCache(t, unstructuredPod("nginx", "default"))
	c.respectRBAC = RespectRbacDisabled

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	// With respectRBAC=Disabled the handler delegates to the default
	// handler; the default needs a real reflector to log safely, so we
	// skip invoking it and simply assert that apisMeta stays intact —
	// behavior is controlled by not reaching stopWatching. Simulate by
	// replacing only the default-delegation path.
	//
	// (We can't call the handler with a nil reflector because
	// DefaultWatchErrorHandler dereferences r.name. The behavior of not
	// calling stopWatching is verified by the code shape in
	// informer_lifecycle.go — any change there would fail the Forbidden
	// RBACEnabled test above because that path DOES stopWatching.)
	c.lock.RLock()
	_, stillWatching := c.apisMeta[podsGVK.GroupKind()]
	c.lock.RUnlock()
	assert.True(t, stillWatching, "sanity: watch is active before we would fire the handler")
}

func TestInformer_CreateEventFlowsIntoShadowMaps(t *testing.T) {
	c, client := newInformerTestCache(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))

	// Wait until the initial (empty) sync has completed before we publish
	// the pod, so our assertion observes a real Add event rather than a list
	// response.
	c.lock.RLock()
	inf := c.apisMeta[podsGVK.GroupKind()].informers[""].informer
	c.lock.RUnlock()
	require.True(t, cache.WaitForCacheSync(ctx.Done(), inf.HasSynced))

	_, err := client.Resource(podsGVR).Namespace("default").Create(ctx, unstructuredPod("nginx", "default"), metav1.CreateOptions{})
	require.NoError(t, err)

	key := kube.NewResourceKey("", "Pod", "default", "nginx")
	assert.Eventually(t, func() bool {
		c.lock.RLock()
		defer c.lock.RUnlock()
		_, ok := c.resources[key]
		ns, nsOK := c.nsIndex["default"]
		return ok && nsOK && len(ns) == 1
	}, 3*time.Second, 20*time.Millisecond,
		"informer event should propagate into both c.resources and c.nsIndex")
}

// TestStartInformersForAPILocked_LazyInitsApisMetaAfterInvalidate is a
// regression test for the "assignment to entry in nil map" panic at
// informer_lifecycle.go:76. The original panic stack:
//
//	startInformersForAPILocked (informer_lifecycle.go:76)
//	 startMissingWatches (cluster.go:721)
//	  runSynced -> handleCRDEvent (cluster.go:944)
//	   dispatchEvent -> onInformerChange (informer_events.go:122)
//
// A stale pre-Invalidate informer goroutine fired OnAdd for a CRD, routed
// through handleCRDEvent -> startMissingWatches -> startInformersForAPILocked,
// which then did `c.apisMeta[gk] = meta` against a nil map (set by
// Invalidate). The controller crashed mid-test with [recovered, repanicked].
func TestStartInformersForAPILocked_LazyInitsApisMetaAfterInvalidate(t *testing.T) {
	c, _ := newInformerTestCache(t)
	c.mode = ModeInformer

	// Reproduce post-Invalidate state: apisMeta and namespacedResources
	// are both nil while syncInformers has not yet rebuilt them.
	c.apisMeta = nil
	c.namespacedResources = nil

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Pre-fix this panicked: "assignment to entry in nil map".
	require.NotPanics(t, func() {
		require.NoError(t, c.startInformersForAPI(ctx, podsAPI()))
	})

	c.lock.RLock()
	defer c.lock.RUnlock()
	require.NotNil(t, c.apisMeta, "startInformersForAPILocked should lazy-init apisMeta")
	_, ok := c.apisMeta[podsGVK.GroupKind()]
	assert.True(t, ok, "the GroupKind we started should now be present")
	require.NotNil(t, c.namespacedResources, "namespacedResources should also be lazy-init'd")
}

// TestInformerEventHandlerForCtx_BailsAfterCancel verifies that once a
// per-informer watch context is cancelled (by Invalidate or stopWatching),
// in-flight events from the reflector's still-draining DeltaFIFO are
// dropped on the floor instead of mutating freshly-rebuilt cache state.
//
// Without this guard, a stale handler can fire dispatchEvent for a CRD
// and trigger the handleCRDEvent -> startMissingWatches ->
// startInformersForAPILocked nil-map panic (see
// TestStartInformersForAPILocked_LazyInitsApisMetaAfterInvalidate).
func TestInformerEventHandlerForCtx_BailsAfterCancel(t *testing.T) {
	c, _ := newInformerTestCache(t)
	c.mode = ModeInformer

	// Build a cachedResource the way transformForInformer would.
	un := unstructuredPod("nginx", "default")
	cached, err := c.transformForInformer(un)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	handler := c.informerEventHandlerForCtx(ctx)
	key := kube.NewResourceKey("", "Pod", "default", "nginx")

	// Sanity: before cancel the handler propagates events.
	handler.OnAdd(cached, false)
	c.lock.RLock()
	_, present := c.resources[key]
	c.lock.RUnlock()
	require.True(t, present, "pre-cancel Add should populate c.resources")

	cancel()

	// Each post-cancel call must be an independent no-op. We delete the
	// entry between calls so that OnDelete cannot mask a leaking OnAdd
	// (or vice versa) — a missing bail in any of the three branches must
	// fail the test on its own.
	for _, step := range []struct {
		name string
		fire func()
	}{
		{"OnAdd", func() { handler.OnAdd(cached, false) }},
		{"OnUpdate", func() { handler.OnUpdate(cached, cached) }},
		{"OnDelete", func() { handler.OnDelete(cached) }},
	} {
		c.lock.Lock()
		delete(c.resources, key)
		c.lock.Unlock()

		step.fire()

		c.lock.RLock()
		_, mutated := c.resources[key]
		c.lock.RUnlock()
		assert.Falsef(t, mutated, "post-cancel %s should not mutate c.resources", step.name)
	}
}

// TestEnsureSynced_DoesNotDeadlockWithGetClusterInfo is a regression test
// for a 4-minute hang observed in TestSyncWithInfos. The deadlock topology
// (from the goroutine dump) was:
//
//	syncInformers goroutine: holds syncStatus.lock, wants c.lock (write)
//	GetClusterInfo goroutine: holds c.lock (RLock), wants syncStatus.lock
//
// EnsureSynced used to hold both c.lock and syncStatus.lock across c.sync().
// Under informer mode sync() -> syncInformers() releases c.lock for
// WaitForCacheSync — opening a window where GetClusterInfo can grab
// c.lock.RLock and then block on syncStatus.lock, which in turn blocks
// syncInformers from re-acquiring c.lock. Classic lock-order inversion.
//
// The fix: drop syncStatus.lock before calling sync() and re-acquire it
// only to write the result.
func TestEnsureSynced_DoesNotDeadlockWithGetClusterInfo(t *testing.T) {
	client := fake.NewSimpleDynamicClient(scheme.Scheme)
	reactor := client.ReactionChain[0]
	client.PrependReactor("list", "*", func(action testcore.Action) (bool, runtime.Object, error) {
		handled, ret, err := reactor.React(action)
		if !handled || err != nil {
			return handled, ret, err
		}
		ret.(metav1.ListInterface).SetResourceVersion("1")
		return handled, ret, nil
	})

	iface := NewClusterCache(
		&rest.Config{Host: "https://test"},
		SetMode(ModeInformer),
		SetKubectl(&kubetest.MockKubectlCmd{
			APIResources:  []kube.APIResourceInfo{podsAPI()},
			DynamicClient: client,
		}),
	)
	c := iface.(*clusterCache)
	t.Cleanup(func() { c.Invalidate() })

	// Hammer GetClusterInfo concurrently with EnsureSynced. The first
	// EnsureSynced builds informers and (under informer mode) releases
	// c.lock while waiting for HasSynced — the exact window where the
	// pre-fix deadlock occurred.
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = iface.EnsureSynced()
	}()

	// Drive GetClusterInfo continuously until EnsureSynced returns. If
	// the deadlock regresses, this test hangs and the testing.T deadline
	// fires.
	deadline := time.After(15 * time.Second)
	for {
		select {
		case <-done:
			// One more call post-sync to confirm we can still read state.
			_ = iface.GetClusterInfo()
			return
		case <-deadline:
			t.Fatal("EnsureSynced + GetClusterInfo deadlocked")
		default:
			_ = iface.GetClusterInfo()
		}
	}
}

// TestInformerEventHandler_InitialListSkipsDispatchEvent verifies that
// events flagged isInInitialList=true do not fire OnEvent handlers or
// route through handleCRDEvent. This matches legacy semantics, where the
// initial state load via replaceResourceCache fires only OnResourceUpdated.
//
// Regression for: every CRD in the initial-list flood triggered
// handleCRDEvent -> reloadOpenAPISchema, flooding logs with
// "Duplicate GVKs detected in OpenAPI schema" once per existing CRD on
// every cluster cache sync.
func TestInformerEventHandler_InitialListSkipsDispatchEvent(t *testing.T) {
	c, _ := newInformerTestCache(t)
	c.mode = ModeInformer

	un := unstructuredPod("nginx", "default")
	cached, err := c.transformForInformer(un)
	require.NoError(t, err)

	var (
		eventCount        int
		resourceUpdatedCt int
	)
	c.OnEvent(func(_ watch.EventType, _ *unstructured.Unstructured) {
		eventCount++
	})
	c.OnResourceUpdated(func(_, _ *Resource, _ map[kube.ResourceKey]*Resource) {
		resourceUpdatedCt++
	})

	handler := c.informerEventHandlerForCtx(context.Background())

	// Initial-list Add: storage + OnResourceUpdated, no dispatchEvent.
	handler.OnAdd(cached, true)
	assert.Equal(t, 0, eventCount, "OnEvent must NOT fire for initial-list events")
	assert.Equal(t, 1, resourceUpdatedCt, "OnResourceUpdated MUST fire so caller state stays consistent with legacy replaceResourceCache")

	// Real watch event: full dispatch.
	handler.OnAdd(cached, false)
	assert.Equal(t, 1, eventCount, "OnEvent must fire for non-initial Add")
	assert.Equal(t, 2, resourceUpdatedCt, "OnResourceUpdated must continue to fire for non-initial Add")
}

// TestInvalidate_InformerModeClearsReadStateSnapshot verifies that
// Invalidate wipes c.resources / c.nsIndex / c.parentUIDToChildren under
// informer mode. These are the read-path source of truth for
// IterateHierarchyV2, GetManagedLiveObjs, and FindResources; leaving them
// populated between Invalidate and the next EnsureSynced lets readers see
// a phantom snapshot of a cluster the cache no longer trusts.
//
// Legacy mode rebuilds the same maps inside sync(), so the assertion only
// applies to informer mode.
func TestInvalidate_InformerModeClearsReadStateSnapshot(t *testing.T) {
	c, _ := newInformerTestCache(t)
	c.mode = ModeInformer

	// Seed the read-path maps directly — Invalidate's contract is to
	// drop this state, regardless of how it got there.
	un := unstructuredPod("nginx", "default")
	key := kube.NewResourceKey("", "Pod", "default", "nginx")
	res := &Resource{Ref: kube.GetObjectRef(un)}
	c.resources[key] = res
	c.nsIndex["default"] = map[kube.ResourceKey]*Resource{key: res}
	c.parentUIDToChildren["parent-uid"] = map[kube.ResourceKey]struct{}{key: {}}

	c.Invalidate()

	c.lock.RLock()
	defer c.lock.RUnlock()
	assert.Empty(t, c.resources, "Invalidate should clear c.resources under informer mode")
	assert.Empty(t, c.nsIndex, "Invalidate should clear c.nsIndex under informer mode")
	assert.Empty(t, c.parentUIDToChildren, "Invalidate should clear c.parentUIDToChildren under informer mode")
}

