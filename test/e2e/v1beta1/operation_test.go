package v1beta1

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// Operation-location tests for v1beta1.
//
// In v1alpha1 the imperative `operation` trigger is a top-level field; in v1beta1
// it is relocated under `status` so that requesting a sync is a status-subresource
// action (gated by applications/status RBAC) rather than a spec write. v1alpha1
// remains the storage version, so the conversion webhook must move `operation`
// between the top level (storage) and `status.operation` (v1beta1 served form) in
// both directions without altering its value.
//
// Unlike the CEL validation tests in this package, these tests perform real writes
// and therefore require the CRD *and* the running conversion webhook (they do not
// use DryRun). They do not require the ArgoCD API server.

func testSyncOperation() *v1alpha1.Operation {
	return &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{
			Revision: "HEAD",
			Prune:    true,
		},
	}
}

// TestV1beta1OperationVisibleUnderStatusAfterV1alpha1Write verifies the read-path
// conversion: an operation written top-level via v1alpha1 (the storage-native path
// used by the ArgoCD server) is surfaced under status.operation when the same object
// is read through the v1beta1 API.
func TestV1beta1OperationVisibleUnderStatusAfterV1alpha1Write(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()
	name := "test-op-read-conversion-" + randomString(5)

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(), newV1beta1App(name, namespace), metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = clientset.ArgoprojV1alpha1().Applications(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	})

	// Write the operation top-level via v1alpha1 (no status subresource on v1alpha1,
	// so a main-resource update persists the top-level operation field).
	live, err := clientset.ArgoprojV1alpha1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	live.Operation = testSyncOperation()
	_, err = clientset.ArgoprojV1alpha1().Applications(namespace).Update(context.Background(), live, metav1.UpdateOptions{})
	require.NoError(t, err)

	// Read via v1beta1: operation must appear under status.
	betaApp, err := clientset.ArgoprojV1beta1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, betaApp.Status.Operation, "operation must be surfaced under status.operation in v1beta1")
	require.NotNil(t, betaApp.Status.Operation.Sync)
	assert.Equal(t, "HEAD", betaApp.Status.Operation.Sync.Revision)
	assert.True(t, betaApp.Status.Operation.Sync.Prune)

	// Read via v1alpha1: operation must still be top-level (storage form).
	alphaApp, err := clientset.ArgoprojV1alpha1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, alphaApp.Operation, "operation must remain top-level in v1alpha1")
	assert.Equal(t, "HEAD", alphaApp.Operation.Sync.Revision)
}

// TestV1beta1OperationStatusSubresourceRoundTripsToStorage is the load-bearing
// assertion: an operation set via the v1beta1 *status subresource* (status.operation)
// must be converted and persisted to the v1alpha1 top-level operation field in
// storage. This is the path the design relies on for requesting a sync through the
// gated status subresource.
func TestV1beta1OperationStatusSubresourceRoundTripsToStorage(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()
	name := "test-op-status-roundtrip-" + randomString(5)

	created, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(), newV1beta1App(name, namespace), metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = clientset.ArgoprojV1alpha1().Applications(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	})

	// Set the operation via the v1beta1 status subresource.
	created.Status.Operation = testSyncOperation()
	_, err = clientset.ArgoprojV1beta1().Applications(namespace).UpdateStatus(context.Background(), created, metav1.UpdateOptions{})
	require.NoError(t, err)

	// It must have landed in v1alpha1 storage as the top-level operation field.
	alphaApp, err := clientset.ArgoprojV1alpha1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, alphaApp.Operation, "status.operation written via v1beta1 must round-trip to top-level operation in v1alpha1 storage")
	require.NotNil(t, alphaApp.Operation.Sync)
	assert.Equal(t, "HEAD", alphaApp.Operation.Sync.Revision)
	assert.True(t, alphaApp.Operation.Sync.Prune)

	// And it must read back consistently through v1beta1.
	betaApp, err := clientset.ArgoprojV1beta1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	require.NotNil(t, betaApp.Status.Operation)
	assert.Equal(t, "HEAD", betaApp.Status.Operation.Sync.Revision)
}

// TestV1beta1OperationNotSettableViaMainResource verifies the gate: because
// `operation` lives under status in v1beta1 and the status subresource is enabled,
// a main-resource write cannot set it. This is what forces manual syncs through the
// status subresource (UI/CLI/API) rather than a plain spec write.
func TestV1beta1OperationNotSettableViaMainResource(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()
	name := "test-op-main-write-ignored-" + randomString(5)

	created, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(), newV1beta1App(name, namespace), metav1.CreateOptions{})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = clientset.ArgoprojV1alpha1().Applications(namespace).Delete(context.Background(), name, metav1.DeleteOptions{})
	})

	// Attempt to set the operation through the main resource (Update, not UpdateStatus).
	// With the status subresource enabled, the apiserver ignores status on main writes,
	// so the operation must not be persisted.
	created.Status.Operation = testSyncOperation()
	_, err = clientset.ArgoprojV1beta1().Applications(namespace).Update(context.Background(), created, metav1.UpdateOptions{})
	require.NoError(t, err)

	betaApp, err := clientset.ArgoprojV1beta1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Nil(t, betaApp.Status.Operation, "operation set via a main-resource write must not be persisted (status subresource gates it)")

	alphaApp, err := clientset.ArgoprojV1alpha1().Applications(namespace).Get(context.Background(), name, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Nil(t, alphaApp.Operation, "operation must remain unset in storage after a v1beta1 main-resource write")
}
