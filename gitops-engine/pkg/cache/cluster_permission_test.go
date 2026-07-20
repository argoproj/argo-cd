package cache

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	authType1 "k8s.io/client-go/kubernetes/typed/authorization/v1"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
)

// recordingSSAR answers from a verb -> allowed map and records every verb checked.
type recordingSSAR struct {
	allowed map[string]bool
	verbs   []string
}

func (r *recordingSSAR) Create(ctx context.Context, sar *authorizationv1.SelfSubjectAccessReview, opts metav1.CreateOptions) (*authorizationv1.SelfSubjectAccessReview, error) {
	verb := sar.Spec.ResourceAttributes.Verb
	r.verbs = append(r.verbs, verb)
	out := sar.DeepCopy()
	out.Status.Allowed = r.allowed[verb]
	return out, nil
}

// compile-time interface check
var _ authType1.SelfSubjectAccessReviewInterface = (*recordingSSAR)(nil)

func storageClassAPI() kube.APIResourceInfo {
	return kube.APIResourceInfo{
		GroupKind: schema.GroupKind{Group: "storage.k8s.io", Kind: "StorageClass"},
		GroupVersionResource: schema.GroupVersionResource{
			Group: "storage.k8s.io", Version: "v1", Resource: "storageclasses",
		},
		Meta: metav1.APIResource{
			Name:       "storageclasses",
			Namespaced: false,
			Group:      "storage.k8s.io",
			Kind:       "StorageClass",
			Version:    "v1",
		},
	}
}

func TestCheckPermission_RequiresListAndWatch(t *testing.T) {
	// Cluster-wide cache (no namespace restriction).
	cluster := &clusterCache{}

	t.Run("allowed when both list and watch are permitted", func(t *testing.T) {
		ssar := &recordingSSAR{allowed: map[string]bool{"list": true, "watch": true}}
		keep, err := cluster.checkPermission(context.Background(), ssar, storageClassAPI())
		require.NoError(t, err)
		assert.True(t, keep)
		assert.ElementsMatch(t, []string{"list", "watch"}, ssar.verbs)
	})

	t.Run("denied when only list is permitted (OpenShift basic-user case)", func(t *testing.T) {
		ssar := &recordingSSAR{allowed: map[string]bool{"list": true, "watch": false}}
		keep, err := cluster.checkPermission(context.Background(), ssar, storageClassAPI())
		require.NoError(t, err)
		assert.False(t, keep, "list without watch must not keep the resource on the watch list")
		assert.Contains(t, ssar.verbs, "list")
		assert.Contains(t, ssar.verbs, "watch")
	})

	t.Run("denied when list is not permitted", func(t *testing.T) {
		ssar := &recordingSSAR{allowed: map[string]bool{"list": false, "watch": true}}
		keep, err := cluster.checkPermission(context.Background(), ssar, storageClassAPI())
		require.NoError(t, err)
		assert.False(t, keep)
		assert.Contains(t, ssar.verbs, "list")
	})

	t.Run("denied when neither verb is permitted", func(t *testing.T) {
		ssar := &recordingSSAR{allowed: map[string]bool{"list": false, "watch": false}}
		keep, err := cluster.checkPermission(context.Background(), ssar, storageClassAPI())
		require.NoError(t, err)
		assert.False(t, keep)
	})
}

func TestIsAllowed_SetsGroupAndVerb(t *testing.T) {
	cluster := &clusterCache{}
	var got *authorizationv1.ResourceAttributes
	ssar := &captureSSAR{onCreate: func(sar *authorizationv1.SelfSubjectAccessReview) {
		got = sar.Spec.ResourceAttributes.DeepCopy()
		sar.Status.Allowed = true
	}}

	allowed, err := cluster.isAllowed(context.Background(), ssar, storageClassAPI(), "watch")
	require.NoError(t, err)
	assert.True(t, allowed)
	require.NotNil(t, got)
	assert.Equal(t, "watch", got.Verb)
	assert.Equal(t, "storage.k8s.io", got.Group)
	assert.Equal(t, "storageclasses", got.Resource)
}

func TestIsAllowed_Namespaced(t *testing.T) {
	cluster := &clusterCache{namespaces: []string{"app-ns", "other-ns"}}
	api := kube.APIResourceInfo{
		GroupKind: schema.GroupKind{Group: "", Kind: "Secret"},
		GroupVersionResource: schema.GroupVersionResource{
			Group: "", Version: "v1", Resource: "secrets",
		},
		Meta: metav1.APIResource{Name: "secrets", Namespaced: true, Kind: "Secret", Version: "v1"},
	}

	t.Run("allowed if any managed namespace permits the verb", func(t *testing.T) {
		ssar := &nsAwareSSAR{allow: map[string]bool{"app-ns": true, "other-ns": false}}
		allowed, err := cluster.isAllowed(context.Background(), ssar, api, "watch")
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("denied if no managed namespace permits the verb", func(t *testing.T) {
		ssar := &nsAwareSSAR{allow: map[string]bool{"app-ns": false, "other-ns": false}}
		allowed, err := cluster.isAllowed(context.Background(), ssar, api, "watch")
		require.NoError(t, err)
		assert.False(t, allowed)
	})
}

func TestIsRestrictedResource(t *testing.T) {
	forbidden := apierrors.NewForbidden(schema.GroupResource{Group: "storage.k8s.io", Resource: "storageclasses"}, "", assert.AnError)
	unauthorized := apierrors.NewUnauthorized("nope")

	disabled := &clusterCache{respectRBAC: RespectRbacDisabled}
	assert.False(t, disabled.isRestrictedResource(forbidden))

	normal := &clusterCache{respectRBAC: RespectRbacNormal}
	assert.True(t, normal.isRestrictedResource(forbidden))
	assert.True(t, normal.isRestrictedResource(unauthorized))
	assert.False(t, normal.isRestrictedResource(assert.AnError))
}

// captureSSAR invokes onCreate with the review before returning it.
type captureSSAR struct {
	onCreate func(*authorizationv1.SelfSubjectAccessReview)
}

func (c *captureSSAR) Create(ctx context.Context, sar *authorizationv1.SelfSubjectAccessReview, opts metav1.CreateOptions) (*authorizationv1.SelfSubjectAccessReview, error) {
	out := sar.DeepCopy()
	if c.onCreate != nil {
		c.onCreate(out)
	}
	return out, nil
}

var _ authType1.SelfSubjectAccessReviewInterface = (*captureSSAR)(nil)

// nsAwareSSAR answers based on the namespace on the review request.
type nsAwareSSAR struct {
	allow map[string]bool
}

func (n *nsAwareSSAR) Create(ctx context.Context, sar *authorizationv1.SelfSubjectAccessReview, opts metav1.CreateOptions) (*authorizationv1.SelfSubjectAccessReview, error) {
	out := sar.DeepCopy()
	out.Status.Allowed = n.allow[sar.Spec.ResourceAttributes.Namespace]
	return out, nil
}

var _ authType1.SelfSubjectAccessReviewInterface = (*nsAwareSSAR)(nil)
