package kube

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/mocks"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing"
	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/cmd/apply"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

func newTestKubectlResourceOperations(t *testing.T) (*kubectlResourceOperations, *mocks.KubectlCommandFacade) {
	t.Helper()

	cmdMocks := mocks.NewKubectlCommandFacade(t)

	k := &kubectlResourceOperations{
		config:        &rest.Config{},
		log:           logr.Discard(),
		tracer:        &tracing.NopTracer{},
		fact:          cmdutil.NewFactory(cmdutil.NewMatchVersionFlags(genericclioptions.NewConfigFlags(true))),
		commandFacade: cmdMocks,
		getClientFunc: func() (kubernetes.Interface, error) {
			return kubefake.NewSimpleClientset(), nil
		},
		outputMode: outputModeLog,
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
	// and server-side apply setting. It uses the facade pattern to track kubectl command executions.

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

func TestOutputModeLog(t *testing.T) {
	// Test normal flow operations with outputModeLog

	t.Run("CreateResource with outputModeLog", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.CreateResource(t.Context(), obj, cmdutil.DryRunNone, false)
		require.NoError(t, err)
	})

	t.Run("ReplaceResource with outputModeLog", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Replace", mock.Anything, mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ReplaceResource(t.Context(), obj, cmdutil.DryRunNone, false)
		require.NoError(t, err)
	})

	t.Run("ApplyResource with outputModeLog and client-side apply", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, false, "test-manager")
		require.NoError(t, err)
	})

	t.Run("ApplyResource with outputModeLog and server-side apply", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, true, "test-manager")
		require.NoError(t, err)
	})
}

func TestOutputModeJSON(t *testing.T) {
	// Test JSON output mode operations

	t.Run("CreateResource with outputModeJSON should fail", func(t *testing.T) {
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.CreateResource(t.Context(), obj, cmdutil.DryRunServer, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CreateResource is not supported with JSON output mode")
	})

	t.Run("ReplaceResource with outputModeJSON should fail", func(t *testing.T) {
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ReplaceResource(t.Context(), obj, cmdutil.DryRunServer, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ReplaceResource is not supported with JSON output mode")
	})

	t.Run("ApplyResource with outputModeJSON without Dry run", func(t *testing.T) {
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, true, "test-manager")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid dry run strategy used with JSON output")
	})

	t.Run("ApplyResource with outputModeJSON requires DryRunServer", func(t *testing.T) {
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunClient, false, false, true, "test-manager")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid dry run strategy used with JSON output")
	})

	t.Run("ApplyResource with outputModeJSON requires server-side apply", func(t *testing.T) {
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunServer, false, false, false, "test-manager")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Apply strategy used with JSON output")
	})

	t.Run("ApplyResource with outputModeJSON return object", func(t *testing.T) {
		obj := testingutils.NewPod()
		jsonObj, err := json.Marshal(obj)
		require.NoError(t, err)

		k, cmdMocks := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			applyOpts := args[0].(*apply.ApplyOptions)
			_, err := applyOpts.Out.Write(jsonObj)
			require.NoError(t, err)
		}).Return(nil)

		result, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunServer, false, false, true, "test-manager")
		require.NoError(t, err)
		assert.JSONEq(t, string(jsonObj), result)
	})

	t.Run("ApplyResource with outputModeJSON with object and stderr returns object", func(t *testing.T) {
		obj := testingutils.NewPod()
		jsonObj, err := json.Marshal(obj)
		require.NoError(t, err)

		k, cmdMocks := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			applyOpts := args[0].(*apply.ApplyOptions)
			_, err := applyOpts.Out.Write(jsonObj)
			require.NoError(t, err)

			// add an stderr message that should not be returned in the result
			_, err = applyOpts.ErrOut.Write([]byte("error message"))
			require.NoError(t, err)
		}).Return(nil)

		result, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunServer, false, false, true, "test-manager")
		require.NoError(t, err)
		assert.JSONEq(t, string(jsonObj), result)
	})

	t.Run("ApplyResource with outputModeJSON without object with a stderr returns error", func(t *testing.T) {
		obj := testingutils.NewPod()

		k, cmdMocks := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			applyOpts := args[0].(*apply.ApplyOptions)

			_, err := applyOpts.ErrOut.Write([]byte("error message"))
			require.NoError(t, err)
		}).Return(nil)

		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunServer, false, false, true, "test-manager")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error message")
	})
}

func TestApplyOptionsConfiguration(t *testing.T) {
	// Test that newApplyOptions correctly configures all ApplyOptions fields

	t.Run("serverSideApply=false sets ServerSideApply=false and ForceConflicts=false", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)

		var capturedOpts *apply.ApplyOptions
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			capturedOpts = args[0].(*apply.ApplyOptions)
		}).Return(nil)

		ssa := false
		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, ssa, "")
		require.NoError(t, err)

		assert.False(t, capturedOpts.ServerSideApply)
		assert.False(t, capturedOpts.ForceConflicts)
	})

	t.Run("serverSideApply=true sets ServerSideApply=true and ForceConflicts=true", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)

		var capturedOpts *apply.ApplyOptions
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			capturedOpts = args[0].(*apply.ApplyOptions)
		}).Return(nil)

		ssa := true
		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunServer, false, false, ssa, "test-manager")
		require.NoError(t, err)

		assert.True(t, capturedOpts.ServerSideApply)
		assert.True(t, capturedOpts.ForceConflicts)
	})

	t.Run("DryRunStrategy is correctly set", func(t *testing.T) {
		testCases := []struct {
			name     string
			strategy cmdutil.DryRunStrategy
		}{
			{"DryRunNone", cmdutil.DryRunNone},
			{"DryRunClient", cmdutil.DryRunClient},
			{"DryRunServer", cmdutil.DryRunServer},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				k, cmdMocks := newTestKubectlResourceOperations(t)

				var capturedOpts *apply.ApplyOptions
				cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
					capturedOpts = args[0].(*apply.ApplyOptions)
				}).Return(nil)

				obj := testingutils.NewPod()
				_, err := k.ApplyResource(t.Context(), obj, tc.strategy, false, false, false, "")
				require.NoError(t, err)

				assert.Equal(t, tc.strategy, capturedOpts.DryRunStrategy)
			})
		}
	})

	t.Run("FieldManager is correctly set", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)

		var capturedOpts *apply.ApplyOptions
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			capturedOpts = args[0].(*apply.ApplyOptions)
		}).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, false, "test-manager")
		require.NoError(t, err)

		assert.Equal(t, "test-manager", capturedOpts.FieldManager)
	})

	t.Run("force=true sets DeleteOptions.ForceDeletion", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)

		var capturedOpts *apply.ApplyOptions
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			capturedOpts = args[0].(*apply.ApplyOptions)
		}).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, true, false, false, "")
		require.NoError(t, err)

		assert.True(t, capturedOpts.DeleteOptions.ForceDeletion)
	})

	t.Run("Overwrite and OpenAPIPatch are always true", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)

		var capturedOpts *apply.ApplyOptions
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			capturedOpts = args[0].(*apply.ApplyOptions)
		}).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, false, "")
		require.NoError(t, err)

		assert.True(t, capturedOpts.Overwrite)
		assert.True(t, capturedOpts.OpenAPIPatch)
	})

	t.Run("outputModeJSON returns JSONPrinter", func(t *testing.T) {
		k, cmdMocks := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		var capturedOpts *apply.ApplyOptions
		cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
			capturedOpts = args[0].(*apply.ApplyOptions)
		}).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunServer, false, false, true, "test-manager")
		require.NoError(t, err)

		// Call ToPrinter and verify it returns a JSON printer
		printer, err := capturedOpts.ToPrinter("configured")
		require.NoError(t, err)
		assert.NotNil(t, printer)

		// Verify it's a JSONPrinter by checking the type
		_, isJSONPrinter := printer.(*printers.JSONPrinter)
		assert.True(t, isJSONPrinter, "Expected printer to be of type *printers.JSONPrinter")

		// Verify ShowManagedFields is set to true for JSON output
		assert.True(t, capturedOpts.PrintFlags.JSONYamlPrintFlags.ShowManagedFields)
	})
}
