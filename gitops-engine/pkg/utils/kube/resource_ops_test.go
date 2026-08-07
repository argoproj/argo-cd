package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubectl/pkg/cmd/apply"
	"k8s.io/kubectl/pkg/cmd/auth"
	"k8s.io/kubectl/pkg/cmd/create"
	"k8s.io/kubectl/pkg/cmd/replace"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/mocks"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/tracing"
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
		outputMode: outputModeLog,
	}
	return k, cmdMocks
}

func TestAuthReconcileWithMissingNamespace(t *testing.T) {
	t.Parallel()
	namespace := "test-ns"

	t.Run("Namespaced resources", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)

		role := testingutils.NewRole()
		role.SetNamespace(namespace)

		_, err := k.rbacReconcile(t.Context(), role, cmdutil.DryRunNone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `namespaces "test-ns" not found`)

		roleBinding := testingutils.NewRoleBinding()
		roleBinding.SetNamespace(namespace)

		_, err = k.rbacReconcile(t.Context(), roleBinding, cmdutil.DryRunNone)
		require.Error(t, err)
		assert.Contains(t, err.Error(), `namespaces "test-ns" not found`)
	})

	t.Run("Cluster-scoped resources", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("AuthReconcile", mock.Anything).Return(nil).Twice()

		clusterRole := testingutils.NewClusterRole()
		clusterRole.SetNamespace(namespace)

		_, err := k.rbacReconcile(t.Context(), clusterRole, cmdutil.DryRunNone)
		require.NoError(t, err)

		clusterRoleBinding := testingutils.NewClusterRoleBinding()
		clusterRoleBinding.SetNamespace(namespace)

		_, err = k.rbacReconcile(t.Context(), clusterRoleBinding, cmdutil.DryRunNone)
		require.NoError(t, err)
	})
}

func TestAuthReconcileUsage(t *testing.T) {
	t.Parallel()
	// This test verifies that the rbacReconcile logic is correctly applied based on the operation type
	// and server-side apply setting.

	role := testingutils.NewRole()

	t.Run("CreateResource should not call auth reconcile", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		_, err := k.CreateResource(t.Context(), role, cmdutil.DryRunNone, false)
		require.NoError(t, err)
		cmdMocks.AssertNotCalled(t, "AuthReconcile")
	})

	t.Run("ReplaceResource should not call auth reconcile", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Replace", mock.Anything, mock.Anything).Return(nil)

		_, err := k.ReplaceResource(t.Context(), role, cmdutil.DryRunNone, false)
		require.NoError(t, err)
		cmdMocks.AssertNotCalled(t, "AuthReconcile")
	})

	t.Run("ApplyResource should not call auth reconcile on server-side apply", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)

		ssa := true
		_, err := k.ApplyResource(t.Context(), role, cmdutil.DryRunNone, false, false, ssa, "")
		require.NoError(t, err)
		cmdMocks.AssertNotCalled(t, "AuthReconcile")
	})

	t.Run("ApplyResource should call auth reconcile on client-side apply", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)
		cmdMocks.On("AuthReconcile", mock.Anything).Return(nil)

		ssa := false
		_, err := k.ApplyResource(t.Context(), role, cmdutil.DryRunNone, false, false, ssa, "")
		require.NoError(t, err)
	})
}

func TestOutputModeLog(t *testing.T) {
	t.Parallel()
	// Test normal flow operations with outputModeLog

	t.Run("CreateResource with outputModeLog", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Create", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.CreateResource(t.Context(), obj, cmdutil.DryRunNone, false)
		require.NoError(t, err)
	})

	t.Run("ReplaceResource with outputModeLog", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Replace", mock.Anything, mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ReplaceResource(t.Context(), obj, cmdutil.DryRunNone, false)
		require.NoError(t, err)
	})

	t.Run("ApplyResource with outputModeLog and client-side apply", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, false, "test-manager")
		require.NoError(t, err)
	})

	t.Run("ApplyResource with outputModeLog and server-side apply", func(t *testing.T) {
		t.Parallel()
		k, cmdMocks := newTestKubectlResourceOperations(t)
		cmdMocks.On("Apply", mock.Anything).Return(nil)

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, true, "test-manager")
		require.NoError(t, err)
	})
}

func TestOutputModeJSON(t *testing.T) {
	t.Parallel()
	// Test JSON output mode operations

	t.Run("CreateResource with outputModeJSON should fail", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.CreateResource(t.Context(), obj, cmdutil.DryRunServer, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "CreateResource is not supported with JSON output mode")
	})

	t.Run("ReplaceResource with outputModeJSON should fail", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ReplaceResource(t.Context(), obj, cmdutil.DryRunServer, false)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "ReplaceResource is not supported with JSON output mode")
	})

	t.Run("ApplyResource with outputModeJSON without Dry run", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunNone, false, false, true, "test-manager")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid dry run strategy used with JSON output")
	})

	t.Run("ApplyResource with outputModeJSON requires DryRunServer", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunClient, false, false, true, "test-manager")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid dry run strategy used with JSON output")
	})

	t.Run("ApplyResource with outputModeJSON requires server-side apply", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)
		k.outputMode = outputModeJSON

		obj := testingutils.NewPod()
		_, err := k.ApplyResource(t.Context(), obj, cmdutil.DryRunServer, false, false, false, "test-manager")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid Apply strategy used with JSON output")
	})

	t.Run("ApplyResource with outputModeJSON return object", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
	t.Parallel()
	// Test that newApplyOptions correctly configures all ApplyOptions fields
	t.Run("general options are correctly set", func(t *testing.T) {
		t.Parallel()
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
				t.Parallel()
				k, cmdMocks := newTestKubectlResourceOperations(t)

				var capturedOpts *apply.ApplyOptions
				cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
					capturedOpts = args[0].(*apply.ApplyOptions)
				}).Return(nil)

				obj := testingutils.NewPod()
				_, err := k.ApplyResource(t.Context(), obj, tc.strategy, false, false, false, "test-manager")
				require.NoError(t, err)

				assert.Equal(t, tc.strategy, capturedOpts.DryRunStrategy)
				assert.Equal(t, "test-manager", capturedOpts.FieldManager)
				assert.True(t, capturedOpts.Overwrite)
				if tc.strategy == cmdutil.DryRunClient {
					// workaround for https://github.com/kubernetes/kubernetes/issues/139538
					// in kubectl v1.36
					assert.False(t, capturedOpts.OpenAPIPatch)
				} else {
					assert.True(t, capturedOpts.OpenAPIPatch)
				}
				assert.False(t, capturedOpts.ServerSideApply)
				assert.False(t, capturedOpts.ForceConflicts)
			})
		}
	})

	t.Run("serverSideApply=true sets ServerSideApply=true and ForceConflicts=true", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name          string
			strategy      cmdutil.DryRunStrategy
			expectedError string
		}{
			{"DryRunNone", cmdutil.DryRunNone, ""},
			{"DryRunClient", cmdutil.DryRunClient, "error validating options: --dry-run=client doesn't work with --server-side"},
			{"DryRunServer", cmdutil.DryRunServer, ""},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				k, cmdMocks := newTestKubectlResourceOperations(t)

				var capturedOpts *apply.ApplyOptions
				if tc.expectedError == "" {
					cmdMocks.On("Apply", mock.Anything).Run(func(args mock.Arguments) {
						capturedOpts = args[0].(*apply.ApplyOptions)
					}).Return(nil)
				}

				ssa := true
				obj := testingutils.NewPod()
				_, err := k.ApplyResource(t.Context(), obj, tc.strategy, false, false, ssa, "test-manager")

				if tc.expectedError == "" {
					require.NoError(t, err)
					assert.True(t, capturedOpts.ServerSideApply)
					assert.True(t, capturedOpts.ForceConflicts)
				} else {
					require.Error(t, err)
					assert.Contains(t, err.Error(), tc.expectedError)
				}
			})
		}
	})

	t.Run("force=true sets DeleteOptions.ForceDeletion", func(t *testing.T) {
		t.Parallel()
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
				t.Parallel()
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
		}
	})

	t.Run("outputModeJSON returns JSONPrinter", func(t *testing.T) {
		t.Parallel()
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

func TestCreateOptionsConfiguration(t *testing.T) {
	t.Parallel()
	// Test that newCreateOptions correctly configures all CreateOptions fields

	t.Run("general options are correctly set", func(t *testing.T) {
		t.Parallel()
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
				t.Parallel()
				k, cmdMocks := newTestKubectlResourceOperations(t)

				var capturedOpts *create.CreateOptions
				cmdMocks.On("Create", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					capturedOpts = args[0].(*create.CreateOptions)
				}).Return(nil)

				obj := testingutils.NewPod()
				_, err := k.CreateResource(t.Context(), obj, tc.strategy, false)
				require.NoError(t, err)

				assert.Equal(t, tc.strategy, capturedOpts.DryRunStrategy)
				assert.NotEmpty(t, capturedOpts.FilenameOptions.Filenames)
				assert.NotNil(t, capturedOpts.PrintObj)
			})
		}
	})
}

func TestReplaceOptionsConfiguration(t *testing.T) {
	t.Parallel()
	// Test that newReplaceOptions correctly configures all ReplaceOptions fields

	t.Run("general options are correctly set", func(t *testing.T) {
		t.Parallel()
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
				t.Parallel()
				k, cmdMocks := newTestKubectlResourceOperations(t)

				var capturedOpts *replace.ReplaceOptions
				cmdMocks.On("Replace", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					capturedOpts = args[0].(*replace.ReplaceOptions)
				}).Return(nil)

				obj := testingutils.NewPod()
				obj.SetNamespace("test-namespace")
				_, err := k.ReplaceResource(t.Context(), obj, tc.strategy, false)
				require.NoError(t, err)

				assert.Equal(t, tc.strategy, capturedOpts.DryRunStrategy)
				assert.False(t, capturedOpts.DeleteOptions.ForceDeletion)
				assert.NotEmpty(t, capturedOpts.DeleteOptions.Filenames)
				assert.Equal(t, "test-namespace", capturedOpts.Namespace)
				assert.NotNil(t, capturedOpts.PrintObj)
			})
		}
	})

	t.Run("force=true sets DeleteOptions.ForceDeletion correctly", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name     string
			strategy cmdutil.DryRunStrategy
			expected bool
		}{
			{"DryRunNone", cmdutil.DryRunNone, true},
			{"DryRunClient", cmdutil.DryRunClient, false},
			{"DryRunServer", cmdutil.DryRunServer, false},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				k, cmdMocks := newTestKubectlResourceOperations(t)

				var capturedOpts *replace.ReplaceOptions
				cmdMocks.On("Replace", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
					capturedOpts = args[0].(*replace.ReplaceOptions)
				}).Return(nil)

				obj := testingutils.NewPod()
				_, err := k.ReplaceResource(t.Context(), obj, tc.strategy, true)
				require.NoError(t, err)

				assert.Equal(t, tc.expected, capturedOpts.DeleteOptions.ForceDeletion)
			})
		}
	})
}

// TestRealKubectlOptionsRunner_AuthReconcile_PanicRecovery verifies that the
// recover() wrapper in realKubectlOptionsRunner.AuthReconcile converts a panic
// inside kubectl into a returned error instead of crashing the controller
// (see GitHub #28607).
func TestRealKubectlOptionsRunner_AuthReconcile_PanicRecovery(t *testing.T) {
	t.Parallel()
	runner := &realKubectlOptionsRunner{}
	// A nil *auth.ReconcileOptions panics at opts.RunReconcile() — the same
	// class of panic that occurs when the impersonated SA is forbidden.
	err := runner.AuthReconcile((*auth.ReconcileOptions)(nil))
	require.Error(t, err, "AuthReconcile must return an error rather than propagating the panic")
	assert.Contains(t, err.Error(), "error running kubectl auth reconcile")
}

// TestWarningClients verifies the isolation helper used by the apply/create/
// replace paths without a handler it reuses the shared cached factory/config,
// and with one it returns an isolated config carrying the handler so that
// concurrent operations never share their warnings.
// countingRESTClientGetter is a stub RESTClientGetter that records how many
// times each method is called and returns fixed sentinels, so tests can assert
// that a wrapper delegates (rather than rebuilds) discovery/mapper lookups.
type countingRESTClientGetter struct {
	config          *rest.Config
	mapper          meta.RESTMapper
	discovery       discovery.CachedDiscoveryInterface
	rawLoader       clientcmd.ClientConfig
	restConfigCalls int
	mapperCalls     int
	discoveryCalls  int
	rawLoaderCalls  int
}

func (g *countingRESTClientGetter) ToRESTConfig() (*rest.Config, error) {
	g.restConfigCalls++
	return g.config, nil
}

func (g *countingRESTClientGetter) ToRESTMapper() (meta.RESTMapper, error) {
	g.mapperCalls++
	return g.mapper, nil
}

func (g *countingRESTClientGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	g.discoveryCalls++
	return g.discovery, nil
}

func (g *countingRESTClientGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	g.rawLoaderCalls++
	return g.rawLoader
}

func TestWarningRESTClientGetter(t *testing.T) {
	t.Parallel()

	t.Run("ToRESTConfig injects the handler on a copy without mutating the shared config", func(t *testing.T) {
		t.Parallel()
		shared := &countingRESTClientGetter{config: &rest.Config{Host: "https://example.com"}}
		wh := rest.NewWarningWriter(&bytes.Buffer{}, rest.WarningWriterOptions{})
		getter := &warningRESTClientGetter{RESTClientGetter: shared, warningHandler: wh}

		cfg, err := getter.ToRESTConfig()
		require.NoError(t, err)
		assert.NotSame(t, shared.config, cfg, "config must be a copy")
		assert.Equal(t, "https://example.com", cfg.Host, "copy preserves the shared config values")
		assert.Equal(t, rest.WarningHandler(wh), cfg.WarningHandler)
		assert.Nil(t, shared.config.WarningHandler, "the shared config must not be mutated")
	})

	t.Run("discovery and mapper lookups delegate to the shared getter", func(t *testing.T) {
		t.Parallel()
		shared := &countingRESTClientGetter{config: &rest.Config{}}
		getter := &warningRESTClientGetter{
			RESTClientGetter: shared,
			warningHandler:   rest.NewWarningWriter(&bytes.Buffer{}, rest.WarningWriterOptions{}),
		}

		gotMapper, err := getter.ToRESTMapper()
		require.NoError(t, err)
		gotDiscovery, err := getter.ToDiscoveryClient()
		require.NoError(t, err)
		gotLoader := getter.ToRawKubeConfigLoader()

		// The wrapper must forward to the shared getter (reusing its caches),
		// returning exactly what it returns, instead of building its own.
		assert.Equal(t, shared.mapper, gotMapper)
		assert.Equal(t, shared.discovery, gotDiscovery)
		assert.Equal(t, shared.rawLoader, gotLoader)
		assert.Equal(t, 1, shared.mapperCalls)
		assert.Equal(t, 1, shared.discoveryCalls)
		assert.Equal(t, 1, shared.rawLoaderCalls)
	})
}

// TestRunResourceCommandSeparatesWarningsFromStderr verifies that the per-resource
// message returned for the UI carries API server warnings (delivered through the
// warning handler) but not kubectl's client-side stderr.
func TestRunResourceCommandSeparatesWarningsFromStderr(t *testing.T) {
	t.Parallel()
	k, _ := newTestKubectlResourceOperations(t)

	// A kubectl client-side notice printed to stderr during create-then-apply.
	const clientSideStderr = "Warning: resource clusterrolebindings/my-crb is missing the " +
		"kubectl.kubernetes.io/last-applied-configuration annotation which is required by kubectl apply."
	// A genuine API server warning delivered through the warning handler.
	const serverWarning = `would violate PodSecurity "restricted"`

	message, err := k.runResourceCommand(context.Background(), testingutils.NewClusterRoleBinding(),
		func(ioStreams genericiooptions.IOStreams, _ string, warningHandler rest.WarningHandler) error {
			_, _ = ioStreams.Out.Write([]byte("clusterrolebinding.rbac.authorization.k8s.io/my-crb configured"))
			_, _ = ioStreams.ErrOut.Write([]byte(clientSideStderr))
			require.NotNil(t, warningHandler, "log output mode must supply a warning handler")
			warningHandler.HandleWarningHeader(299, "", serverWarning)
			return nil
		})
	require.NoError(t, err)

	// The API server warning is surfaced in the UI message...
	assert.Contains(t, message, "clusterrolebinding.rbac.authorization.k8s.io/my-crb configured")
	assert.Contains(t, message, serverWarning)
	// ...but kubectl's client-side stderr is not.
	assert.NotContains(t, message, "last-applied-configuration")
}

func TestHandleLogOutput(t *testing.T) {
	t.Parallel()
	k, _ := newTestKubectlResourceOperations(t)

	t.Run("stdout and API server warnings only", func(t *testing.T) {
		t.Parallel()
		msg, err := k.handleLogOutput("pod/my-pod created", `Warning: would violate PodSecurity "restricted"`)
		require.NoError(t, err)
		assert.Equal(t, `pod/my-pod created. Warning: would violate PodSecurity "restricted"`, msg)
	})

	t.Run("stdout only", func(t *testing.T) {
		t.Parallel()
		msg, err := k.handleLogOutput("pod/my-pod created", "")
		require.NoError(t, err)
		assert.Equal(t, "pod/my-pod created", msg)
	})
}

func TestWarningClients(t *testing.T) {
	t.Parallel()

	t.Run("nil handler reuses the shared factory and config", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)
		fact, cfg := k.warningClients(nil)
		assert.Same(t, k.config, cfg)
		assert.Equal(t, k.fact, fact)
	})

	t.Run("non-nil handler returns an isolated config carrying the handler", func(t *testing.T) {
		t.Parallel()
		k, _ := newTestKubectlResourceOperations(t)
		wh := rest.NewWarningWriter(&bytes.Buffer{}, rest.WarningWriterOptions{})

		fact, cfg := k.warningClients(wh)
		require.NotNil(t, fact)
		assert.NotSame(t, k.config, cfg, "config must be a copy, not the shared one")
		assert.Equal(t, rest.WarningHandler(wh), cfg.WarningHandler)
		assert.Nil(t, k.config.WarningHandler, "the shared config must not be mutated")
	})
}
