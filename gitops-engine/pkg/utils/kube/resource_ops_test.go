package kube

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/mocks"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func newTestKubectlResourceOperations(t *testing.T) (*kubectlResourceOperations, *mocks.KubectlOptionsRunner) {
	t.Helper()

	cmdMocks := mocks.NewKubectlOptionsRunner(t)

	k := &kubectlResourceOperations{
		config:        &rest.Config{},
		log:           logr.Discard(),
		tracer:        &tracing.NopTracer{},
		fact:          cmdutil.NewFactory(cmdutil.NewMatchVersionFlags(genericclioptions.NewConfigFlags(true))),
		optionsRunner: cmdMocks,
		getClientFunc: func() (kubernetes.Interface, error) {
			return kubefake.NewSimpleClientset(), nil
		},
	}
	return k, cmdMocks
}

func TestAuthReconcileWithMissingNamespace(t *testing.T) {
	namespace := "test-ns"

	t.Run("Namespaced resources", func(t *testing.T) {
		k, _ := newTestKubectlResourceOperations(t)

		role := testingutils.NewRole()
		role.SetNamespace(namespace)

		_, err := k.rbacReconcile(context.Background(), role, cmdutil.DryRunNone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `namespaces "test-ns" not found`)

		roleBinding := testingutils.NewRoleBinding()
		roleBinding.SetNamespace(namespace)

		_, err = k.rbacReconcile(context.Background(), roleBinding, cmdutil.DryRunNone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `namespaces "test-ns" not found`)
	})

	t.Run("Cluster-scoped resources", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("AuthReconcile", mock.Anything).Return(nil).Twice()

		clusterRole := testingutils.NewClusterRole()
		clusterRole.SetNamespace(namespace)

		_, err := k.rbacReconcile(context.Background(), clusterRole, cmdutil.DryRunNone)
		require.NoError(t, err)

		clusterRoleBinding := testingutils.NewClusterRoleBinding()
		clusterRoleBinding.SetNamespace(namespace)

		_, err = k.rbacReconcile(context.Background(), clusterRoleBinding, cmdutil.DryRunNone)
		require.NoError(t, err)
	})
}

func TestAuthReconcileUsage(t *testing.T) {
	// This test verifies that the rbacReconcile logic is correctly applied based on the operation type
	// and server-side apply setting.

	role := testingutils.NewRole()

	t.Run("CreateResource should not call auth reconcile", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		_, err := k.CreateResource(t.Context(), role, cmdutil.DryRunNone, false)
		require.NoError(t, err)
		cmdMocks.AssertNotCalled(t, "AuthReconcile")
	})

	t.Run("ReplaceResource should not call auth reconcile", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Replace", mock.Anything, mock.Anything).Return(nil)

		_, err := k.ReplaceResource(t.Context(), role, cmdutil.DryRunNone, false)
		require.NoError(t, err)
		cmdMocks.AssertNotCalled(t, "AuthReconcile")
	})

	t.Run("ApplyResource should not call auth reconcile on server-side apply", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)

		ssa := true
		_, err := k.ApplyResource(t.Context(), role, cmdutil.DryRunNone, false, false, ssa, "")
		require.NoError(t, err)
		cmdMocks.AssertNotCalled(t, "AuthReconcile")
	})

	t.Run("ApplyResource should call auth reconcile on client-side apply", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)
		cmdMocks.On("AuthReconcile", mock.Anything).Return(nil)

		ssa := false
		_, err := k.ApplyResource(t.Context(), role, cmdutil.DryRunNone, false, false, ssa, "")
		require.NoError(t, err)
	})
}
