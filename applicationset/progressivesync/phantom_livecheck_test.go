package progressivesync

import (
	"context"
	"errors"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	crtclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// release-3.4 port of the reverse-deletion live-read tests. Read all three together: they pin the
// boundary the fix has to get right.
//
// performReverseDeletion only lets Reconcile remove the ApplicationSet finalizer when it returns
// (0, nil). A terminating child must therefore be classified correctly:
//
//   - a phantom (still in the informer cache, already gone from the API server) must NOT block
//     forever, or the ApplicationSet stays in Terminating until the finalizer is patched by hand;
//   - a genuinely slow child must NOT be skipped, or the finalizer is released while a child is
//     still terminating and deletionOrder: Reverse silently stops being enforced.

func agedTerminatingApp() v1alpha1.Application {
	deletedAt := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	return v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "appset-stage0",
			Namespace: "argocd",
			// Objects decoded from the API server always carry a UID; the eviction delete is gated
			// on it, so the fixture must have one to be representative.
			UID:               "11111111-2222-3333-4444-555555555555",
			Labels:            map[string]string{"stage": "0"},
			DeletionTimestamp: &deletedAt,
			Finalizers:        []string{v1alpha1.ForegroundPropagationPolicyFinalizer},
		},
	}
}

func singleStepAppSet() v1alpha1.ApplicationSet {
	return v1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{Name: "appset", Namespace: "argocd"},
		Spec: v1alpha1.ApplicationSetSpec{
			Strategy: &v1alpha1.ApplicationSetStrategy{
				Type:          "RollingSync",
				DeletionOrder: ReverseDeletionOrder,
				RollingSync: &v1alpha1.ApplicationSetRolloutStrategy{
					Steps: []v1alpha1.ApplicationSetRolloutStep{
						{MatchExpressions: []v1alpha1.ApplicationMatchExpression{
							{Key: "stage", Operator: "In", Values: []string{"0"}},
						}},
					},
				},
			},
		},
	}
}

func schemeWithApps(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(s))
	return s
}

// evictingCacheClient models the production cache-syncing client for a phantom entry: the object is
// already gone from the API server, so the eviction Delete comes back NotFound, and the entry is then
// removed from the informer store so cache-backed reads stop seeing it. Tests that only simulate the
// Delete without that second half are not representative -- reverse deletion verifies the eviction
// actually happened before it lets a step complete.
//
// onDelete, when set, overrides what the Delete returns; the store is only evicted when it reports
// NotFound, mirroring execAndSyncCache.
func evictingCacheClient(s *runtime.Scheme, app *v1alpha1.Application, onDelete func() error) crtclient.WithWatch {
	evicted := false
	return fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(app).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ crtclient.WithWatch, obj crtclient.Object, _ ...crtclient.DeleteOption) error {
				err := error(nil)
				if onDelete != nil {
					err = onDelete()
				} else {
					err = apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, obj.GetName())
				}
				if apierrors.IsNotFound(err) {
					evicted = true
				}
				return err
			},
			Get: func(ctx context.Context, c crtclient.WithWatch, key crtclient.ObjectKey, obj crtclient.Object, opts ...crtclient.GetOption) error {
				if evicted && key.Name == app.Name && key.Namespace == app.Namespace {
					return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, key.Name)
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
}

func TestPhantomIsNotFatal(t *testing.T) {
	t.Parallel()

	s := schemeWithApps(t)
	phantom := agedTerminatingApp()

	cached := evictingCacheClient(s, &phantom, nil)
	apiServer := fake.NewClientBuilder().WithScheme(s).Build() // object really gone

	m := &Manager{Client: cached, APIReader: apiServer}

	var requeue time.Duration
	var err error
	for i := range 10 {
		requeue, err = m.PerformReverseDeletion(t.Context(), log.NewEntry(log.New()),
			singleStepAppSet(), []v1alpha1.Application{phantom})
		require.NoErrorf(t, err, "pass %d must not error: an error here is permanent, the age it derives from only grows", i)
		if requeue == 0 {
			break
		}
	}

	assert.Zero(t, requeue,
		"a phantom must resolve to (0, nil) so Reconcile can reach RemoveFinalizer; requeue=%s means "+
			"the ApplicationSet stays in Terminating forever", requeue)
}

func TestGenuinelySlowChildIsNotSkipped(t *testing.T) {
	t.Parallel()

	s := schemeWithApps(t)
	slow := agedTerminatingApp()

	cached := fake.NewClientBuilder().WithScheme(s).WithObjects(&slow).Build()
	apiServer := fake.NewClientBuilder().WithScheme(s).WithObjects(&slow).Build() // really still there

	m := &Manager{Client: cached, APIReader: apiServer}

	_, err := m.PerformReverseDeletion(t.Context(), log.NewEntry(log.New()),
		singleStepAppSet(), []v1alpha1.Application{slow})

	require.Error(t, err,
		"a child that still exists on the API server must not be treated as absent; returning (0, nil) "+
			"would release the finalizer mid-teardown and break deletionOrder: Reverse")
	assert.Contains(t, err.Error(), "has not been deleted in over 2 minutes")
}

func TestMissingAPIReaderSurfacesError(t *testing.T) {
	t.Parallel()

	s := schemeWithApps(t)
	app := agedTerminatingApp()
	cached := fake.NewClientBuilder().WithScheme(s).WithObjects(&app).Build()

	m := &Manager{Client: cached} // APIReader deliberately nil

	_, err := m.PerformReverseDeletion(t.Context(), log.NewEntry(log.New()),
		singleStepAppSet(), []v1alpha1.Application{app})

	require.Error(t, err, "an unverifiable child must not be silently skipped or silently retried")
	assert.Contains(t, err.Error(), "could not be verified against the API server")
}

// A stale entry that the API server has confirmed absent must also be evicted from the informer
// store, not merely skipped for this pass.
//
// getCurrentApplications indexes children by owner *name*, so an ApplicationSet recreated under the
// same name inherits any leftover entry and counts it as an existing child. Under a create-only
// policy createInCluster then filters that child out of the create set and never recreates it — and
// because no write is attempted, nothing triggers eviction either, so the child stays missing for as
// long as the entry survives.
//
// Eviction itself belongs to the cache-syncing client and is covered by
// utils.TestDeleteEvictsStaleCacheOnNotFound: a Delete whose API call returns NotFound removes the
// stale entry and swallows the error. What this test pins is the other half of that contract — that
// reverse deletion actually issues the Delete once it has confirmed the object is gone. Without it
// the two halves never meet.
func TestConfirmedPhantomTriggersCacheEviction(t *testing.T) {
	t.Parallel()

	s := schemeWithApps(t)
	phantom := agedTerminatingApp()

	deletes := 0
	// Stand in for the real API server, which reports the object as already gone. That NotFound is
	// what drives eviction in the cache-syncing client, which the helper then models.
	cached := evictingCacheClient(s, &phantom, func() error {
		deletes++
		return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, phantom.Name)
	})
	apiServer := fake.NewClientBuilder().WithScheme(s).Build() // live read: object really gone

	m := &Manager{Client: cached, APIReader: apiServer}

	requeue, err := m.PerformReverseDeletion(t.Context(), log.NewEntry(log.New()),
		singleStepAppSet(), []v1alpha1.Application{phantom})
	require.NoError(t, err)
	require.Zero(t, requeue, "a confirmed-absent child must not hold up reverse deletion")

	assert.Positive(t, deletes,
		"reverse deletion must issue a Delete for a child the API server has confirmed gone, so the "+
			"cache-syncing client evicts the stale entry; otherwise an ApplicationSet recreated under "+
			"this name treats it as an existing child and never recreates it")
}

// The eviction Delete must not remove a different object that has appeared at the same name.
//
// Between the live read that confirms absence and the Delete issued to trigger cache eviction, an
// Application can be created at the same name/namespace. An unconditional delete by name would
// remove it. The delete is therefore gated on the stale entry's UID, so the API server rejects it as
// a conflict and the new object survives.
func TestEvictionDeleteIsGatedOnUID(t *testing.T) {
	t.Parallel()

	s := schemeWithApps(t)
	phantom := agedTerminatingApp()

	var deletedUID types.UID
	var preconditionUID types.UID
	evicted := false
	cached := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(&phantom).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ crtclient.WithWatch, obj crtclient.Object, opts ...crtclient.DeleteOption) error {
				deletedUID = obj.GetUID()
				do := &crtclient.DeleteOptions{}
				for _, o := range opts {
					o.ApplyToDelete(do)
				}
				if do.Preconditions != nil && do.Preconditions.UID != nil {
					preconditionUID = *do.Preconditions.UID
				}
				evicted = true
				return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, obj.GetName())
			},
			// The cache-syncing client evicts the entry on that NotFound, so cached reads stop
			// seeing it. Reverse deletion checks for exactly that before completing the step.
			Get: func(ctx context.Context, c crtclient.WithWatch, key crtclient.ObjectKey, obj crtclient.Object, opts ...crtclient.GetOption) error {
				if evicted && key.Name == phantom.Name {
					return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, key.Name)
				}
				return c.Get(ctx, key, obj, opts...)
			},
		}).
		Build()
	apiServer := fake.NewClientBuilder().WithScheme(s).Build() // object really gone

	m := &Manager{Client: cached, APIReader: apiServer}

	requeue, err := m.PerformReverseDeletion(t.Context(), log.NewEntry(log.New()),
		singleStepAppSet(), []v1alpha1.Application{phantom})
	require.NoError(t, err)
	require.Zero(t, requeue)

	assert.NotEmpty(t, preconditionUID,
		"the eviction Delete must carry a UID precondition, otherwise an Application created at this "+
			"name between the live read and the delete would be removed instead")
	assert.Equal(t, deletedUID, preconditionUID,
		"the precondition must pin the UID of the entry we confirmed absent, not some other object")
}

// Past the staleness threshold, a step may only be treated as complete once the Application is both
// confirmed absent and its stale cache entry evicted. Neither failure mode may let reverse deletion
// proceed, because each would release the ApplicationSet's finalizer on an unproven premise:
//
//   - an unexpected Delete error means the cache-syncing client returned before evicting, leaving the
//     phantom in the store -- the recreation failure the eviction exists to prevent;
//   - a conflict means the UID precondition failed, so an Application exists at this name that is not
//     the one confirmed absent. It may be a child of this ApplicationSet still owed an ordered
//     deletion, and nothing at this point can tell.
//
// Both are transient, so erroring lets a later pass classify the situation instead of guessing.
func TestEvictionFailureDoesNotReleaseTheFinalizer(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name      string
		deleteErr error
		wantErr   bool
		reason    string
	}{
		{
			name:      "unexpected failure blocks progress",
			deleteErr: apierrors.NewInternalError(errors.New("etcd unavailable")),
			wantErr:   true,
			reason: "the client returns before evicting on any non-NotFound error, so the phantom is " +
				"still in the store; continuing would release the finalizer and leave it there",
		},
		{
			name:      "conflict blocks progress too",
			deleteErr: apierrors.NewConflict(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, "repro-app", errors.New("uid mismatch")),
			wantErr:   true,
			reason: "the precondition failed because an Application exists at this name that is not " +
				"the one confirmed absent, which invalidates the verdict this step rests on; the " +
				"replacement may be a child still owed an ordered deletion, so the finalizer must " +
				"not be released until a later pass has classified it",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := schemeWithApps(t)
			phantom := agedTerminatingApp()
			cached := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(&phantom).
				WithInterceptorFuncs(interceptor.Funcs{
					Delete: func(_ context.Context, _ crtclient.WithWatch, _ crtclient.Object, _ ...crtclient.DeleteOption) error {
						return tc.deleteErr
					},
				}).
				Build()
			apiServer := fake.NewClientBuilder().WithScheme(s).Build() // object really gone

			m := &Manager{Client: cached, APIReader: apiServer}

			_, err := m.PerformReverseDeletion(t.Context(), log.NewEntry(log.New()),
				singleStepAppSet(), []v1alpha1.Application{phantom})

			if tc.wantErr {
				require.Error(t, err, tc.reason)
			} else {
				require.NoError(t, err, tc.reason)
			}
		})
	}
}

// A nil error from the eviction Delete is not proof the entry left the informer store.
// cacheSyncingClient.execAndSyncCache logs a failure to reach the store, or to delete from it, and
// then returns the original error -- which for a NotFound delete is nil. Reverse deletion must not
// take that as success: releasing the ApplicationSet's finalizer with the phantom still cached is the
// create-only recreation failure the eviction exists to prevent, and unlike the entry's age, a store
// failure can clear on a later attempt.
func TestSilentEvictionFailureDoesNotReleaseTheFinalizer(t *testing.T) {
	t.Parallel()

	s := schemeWithApps(t)
	phantom := agedTerminatingApp()

	// Delete reports the object gone, as the API server would, but the entry is never evicted --
	// the store failure the production client only logs.
	cached := fake.NewClientBuilder().
		WithScheme(s).
		WithObjects(&phantom).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(_ context.Context, _ crtclient.WithWatch, obj crtclient.Object, _ ...crtclient.DeleteOption) error {
				return apierrors.NewNotFound(schema.GroupResource{Group: "argoproj.io", Resource: "applications"}, obj.GetName())
			},
		}).
		Build()
	apiServer := fake.NewClientBuilder().WithScheme(s).Build() // object really gone

	m := &Manager{Client: cached, APIReader: apiServer}

	_, err := m.PerformReverseDeletion(t.Context(), log.NewEntry(log.New()),
		singleStepAppSet(), []v1alpha1.Application{phantom})

	require.Error(t, err,
		"the stale entry is still readable from the cache, so this step is not complete; completing it "+
			"would release the finalizer and leave the phantom behind")
	assert.Contains(t, err.Error(), "still present after eviction",
		"the error should say the eviction could not be confirmed, not something unrelated")
}
