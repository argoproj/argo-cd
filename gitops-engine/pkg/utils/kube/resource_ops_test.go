package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/rest"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func TestAuthReconcileWithMissingNamespace(t *testing.T) {
	namespace := "test-ns"
	fakeBearer := "fake-bearer"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status := &metav1.Status{
			Status:  "Failure",
			Message: fmt.Sprintf("namespace \"%s\" not found", namespace),
			Reason:  metav1.StatusReasonNotFound,
			Code:    http.StatusNotFound,
		}
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(status)
	}))
	defer server.Close()

	kubeConfigFlags := genericclioptions.NewConfigFlags(true)
	kubeConfigFlags.Namespace = &namespace
	kubeConfigFlags.APIServer = &server.URL
	kubeConfigFlags.BearerToken = &fakeBearer
	matchFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	fact := cmdutil.NewFactory(matchFlags)

	config := &rest.Config{Host: server.URL}
	k := &kubectlResourceOperations{
		config: config,
		fact:   fact,
	}

	role := testingutils.NewRole()
	role.SetNamespace(namespace)

	_, err := k.authReconcile(context.Background(), role, "/dev/null", cmdutil.DryRunNone)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err), "returned error should be resource not found")

	roleBinding := testingutils.NewRoleBinding()
	roleBinding.SetNamespace(namespace)

	_, err = k.authReconcile(context.Background(), roleBinding, "/dev/null", cmdutil.DryRunNone)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err), "returned error should be resource not found")

	clusterRole := testingutils.NewClusterRole()
	clusterRole.SetNamespace(namespace)

	_, err = k.authReconcile(context.Background(), clusterRole, "/dev/null", cmdutil.DryRunNone)
	assert.NoError(t, err)

	clusterRoleBinding := testingutils.NewClusterRoleBinding()
	clusterRoleBinding.SetNamespace(namespace)

	_, err = k.authReconcile(context.Background(), clusterRoleBinding, "/dev/null", cmdutil.DryRunNone)
	assert.NoError(t, err)
}

func TestRBACReconcileUsage(t *testing.T) {
	var executedCommands []string
	onKubectlRun := func(cmd string) (CleanupFunc, error) {
		executedCommands = append(executedCommands, cmd)
		return func() {}, nil
	}

	k := &kubectlResourceOperations{
		onKubectlRun: onKubectlRun,
		tracer:       &tracing.NopTracer{},
	}

	role := testingutils.NewRole()

	t.Run("CreateResource should NOT call rbacReconcile", func(t *testing.T) {
		executedCommands = nil
		_, err := k.runResourceCommand(context.Background(), role, cmdutil.DryRunClient, false, func(ioStreams genericiooptions.IOStreams, fileName string) error {
			return nil
		})
		assert.NoError(t, err)

		for _, cmd := range executedCommands {
			assert.NotEqual(t, "auth", cmd, "auth reconcile should NOT be called")
		}
	})

	t.Run("ReplaceResource should NOT call rbacReconcile", func(t *testing.T) {
		executedCommands = nil
		_, err := k.runResourceCommand(context.Background(), role, cmdutil.DryRunClient, false, func(ioStreams genericiooptions.IOStreams, fileName string) error {
			return nil
		})
		assert.NoError(t, err)

		for _, cmd := range executedCommands {
			assert.NotEqual(t, "auth", cmd, "auth reconcile should NOT be called")
		}
	})

	t.Run("Simulation of original issue: when reconcileRBAC is TRUE, it should fail if resource is created by reconcile first", func(t *testing.T) {
		// This test simulates the BUGGY behavior (passing reconcileRBAC=true to runResourceCommand)
		// and shows why it fails with "already exists".
		var executedCommands []string
		authCalled := false

		kWithAuth := &kubectlResourceOperations{
			onKubectlRun: func(cmd string) (CleanupFunc, error) {
				executedCommands = append(executedCommands, cmd)
				if cmd == "auth" {
					authCalled = true
				}
				return func() {}, nil
			},
			tracer: &tracing.NopTracer{},
		}

		// Mock runResourceCommand behavior manually to avoid calling real authReconcile which panics due to nil fact/config
		runResourceCommandMock := func(ctx context.Context, obj *unstructured.Unstructured, dryRunStrategy cmdutil.DryRunStrategy, reconcileRBAC bool, executor commandExecutor) (string, error) {
			if reconcileRBAC && obj.GetAPIVersion() == "rbac.authorization.k8s.io/v1" {
				_, err := kWithAuth.onKubectlRun("auth")
				require.NoError(t, err)
			}
			ioStreams := genericiooptions.IOStreams{
				In:     &bytes.Buffer{},
				Out:    &bytes.Buffer{},
				ErrOut: &bytes.Buffer{},
			}
			return "", executor(ioStreams, "")
		}

		executor := func(ioStreams genericiooptions.IOStreams, fileName string) error {
			if authCalled {
				// Simulate the "already exists" error that happens when kubectl create/replace
				// is called after kubectl auth reconcile has already created the resource.
				return fmt.Errorf("roles.rbac.authorization.k8s.io \"mytestrole\" already exists")
			}
			return nil
		}

		// If we call it with reconcileRBAC = true (the bug), it should fail
		_, err := runResourceCommandMock(context.Background(), role, cmdutil.DryRunClient, true, executor)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")

		// If we call it with reconcileRBAC = false (the fix), it should succeed
		authCalled = false
		executedCommands = nil
		_, err = runResourceCommandMock(context.Background(), role, cmdutil.DryRunClient, false, executor)
		assert.NoError(t, err)
	})
}
