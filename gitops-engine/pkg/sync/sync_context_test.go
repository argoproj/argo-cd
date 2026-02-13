package sync

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
	"k8s.io/klog/v2/textlogger"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/diff"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube/kubetest"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
)

func newTestSyncCtx(getResourceFunc *func(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error), opts ...SyncOpt) *syncContext {
	sc := syncContext{
		config:    &rest.Config{},
		rawConfig: &rest.Config{},
		namespace: testingutils.FakeArgoCDNamespace,
		revision:  "FooBarBaz",
		disco:     &fakedisco.FakeDiscovery{Fake: &testcore.Fake{Resources: testingutils.StaticAPIResources}},
		log:       textlogger.NewLogger(textlogger.NewConfig()).WithValues("application", "fake-app"),
		resources: map[kube.ResourceKey]reconciledResource{},
		syncRes:   map[string]synccommon.ResourceSyncResult{},
		validate:  true,
	}
	sc.permissionValidator = func(_ *unstructured.Unstructured, _ *metav1.APIResource) error {
		return nil
	}
	mockKubectl := kubetest.MockKubectlCmd{}

	sc.kubectl = &mockKubectl
	mockResourceOps := kubetest.MockResourceOps{}
	sc.resourceOps = &mockResourceOps
	if getResourceFunc != nil {
		mockKubectl.WithGetResourceFunc(*getResourceFunc)
		mockResourceOps.WithGetResourceFunc(*getResourceFunc)
	}

	for _, opt := range opts {
		opt(&sc)
	}
	return &sc
}

// make sure Validate means we don't validate
func TestSyncValidate(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	pod := testingutils.NewPod()
	pod.SetNamespace("fake-argocd-ns")
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{pod},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.validate = false

	syncCtx.Sync()

	// kubectl := syncCtx.kubectl.(*kubetest.MockKubectlCmd)
	resourceOps, _ := syncCtx.resourceOps.(*kubetest.MockResourceOps)
	assert.False(t, resourceOps.GetLastValidate())
}

func TestSyncNotPermittedNamespace(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithPermissionValidator(func(_ *unstructured.Unstructured, _ *metav1.APIResource) error {
		return errors.New("not permitted in project")
	}))
	targetPod := testingutils.NewPod()
	targetPod.SetNamespace("kube-system")
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{targetPod, testingutils.NewService()},
	})
	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Contains(t, resources[0].Message, "not permitted in project")
}

func TestSyncNamespaceCreatedBeforeDryRunWithoutFailure(t *testing.T) {
	pod := testingutils.NewPod()
	syncCtx := newTestSyncCtx(nil, WithNamespaceModifier(func(_, _ *unstructured.Unstructured) (bool, error) {
		return true, nil
	}))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.Sync()
	phase, msg, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for healthy state of /Namespace/fake-argocd-ns", msg)
	require.Len(t, resources, 1)
	assert.Equal(t, "Namespace", resources[0].ResourceKey.Kind)
	assert.Equal(t, synccommon.ResultCodeSynced, resources[0].Status)
}

func TestSyncNamespaceCreatedBeforeDryRunWithFailure(t *testing.T) {
	pod := testingutils.NewPod()
	syncCtx := newTestSyncCtx(nil, WithNamespaceModifier(func(_, _ *unstructured.Unstructured) (bool, error) {
		return true, nil
	}), func(ctx *syncContext) {
		resourceOps := ctx.resourceOps.(*kubetest.MockResourceOps)
		resourceOps.ExecuteForDryRun = true
		resourceOps.Commands = map[string]kubetest.KubectlOutput{}
		resourceOps.Commands[pod.GetName()] = kubetest.KubectlOutput{
			Output: "should not be returned",
			Err:    errors.New("invalid object failing dry-run"),
		}
	})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.Sync()
	phase, msg, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "one or more objects failed to apply (dry run)", msg)
	require.Len(t, resources, 2)
	assert.Equal(t, "Namespace", resources[0].ResourceKey.Kind)
	assert.Equal(t, synccommon.ResultCodeSynced, resources[0].Status)
	assert.Equal(t, "Pod", resources[1].ResourceKey.Kind)
	assert.Equal(t, synccommon.ResultCodeSyncFailed, resources[1].Status)
	assert.Equal(t, "invalid object failing dry-run", resources[1].Message)
}

func TestSyncCreateInSortedOrder(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{testingutils.NewPod(), testingutils.NewService()},
	})
	syncCtx.Sync()

	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Len(t, resources, 2)
	for i := range resources {
		result := resources[i]
		switch result.ResourceKey.Kind {
		case "Pod":
			assert.Equal(t, synccommon.ResultCodeSynced, result.Status)
			assert.Empty(t, result.Message)
		case "Service":
			assert.Empty(t, result.Message)
		default:
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncCustomResources(t *testing.T) {
	type fields struct {
		skipDryRunAnnotationPresent                bool
		skipDryRunAnnotationPresentForAllResources bool
		crdAlreadyPresent                          bool
		crdInSameSync                              bool
	}

	tests := []struct {
		name        string
		fields      fields
		wantDryRun  bool
		wantSuccess bool
	}{
		{"unknown crd", fields{
			skipDryRunAnnotationPresent: false, crdAlreadyPresent: false, crdInSameSync: false,
		}, true, false},
		{"crd present in same sync", fields{
			skipDryRunAnnotationPresent: false, crdAlreadyPresent: false, crdInSameSync: true,
		}, false, true},
		{"crd is already present in cluster", fields{
			skipDryRunAnnotationPresent: false, crdAlreadyPresent: true, crdInSameSync: false,
		}, true, true},
		{"crd is already present in cluster, skip dry run annotated", fields{
			skipDryRunAnnotationPresent: true, crdAlreadyPresent: true, crdInSameSync: false,
		}, true, true},
		{"unknown crd, skip dry run annotated", fields{
			skipDryRunAnnotationPresent: true, crdAlreadyPresent: false, crdInSameSync: false,
		}, false, true},
		{"unknown crd, skip dry run annotated on app level", fields{
			skipDryRunAnnotationPresentForAllResources: true, crdAlreadyPresent: false, crdInSameSync: false,
		}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			knownCustomResourceTypes := []metav1.APIResource{}
			if tt.fields.crdAlreadyPresent {
				knownCustomResourceTypes = append(knownCustomResourceTypes, metav1.APIResource{Kind: "TestCrd", Group: "test.io", Version: "v1", Namespaced: true, Verbs: testingutils.CommonVerbs})
			}

			syncCtx := newTestSyncCtx(nil)
			fakeDisco := syncCtx.disco.(*fakedisco.FakeDiscovery)
			fakeDisco.Resources = append(fakeDisco.Resources, &metav1.APIResourceList{
				GroupVersion: "test.io/v1",
				APIResources: knownCustomResourceTypes,
			})

			cr := testingutils.Unstructured(`
{
  "apiVersion": "test.io/v1",
  "kind": "TestCrd",
  "metadata": {
    "name": "my-resource"
  }
}
`)

			if tt.fields.skipDryRunAnnotationPresent {
				cr.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "SkipDryRunOnMissingResource=true"})
			}

			if tt.fields.skipDryRunAnnotationPresentForAllResources {
				syncCtx.skipDryRunOnMissingResource = true
			}

			resources := []*unstructured.Unstructured{cr}
			if tt.fields.crdInSameSync {
				resources = append(resources, testingutils.NewCRD())
			}

			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   make([]*unstructured.Unstructured, len(resources)),
				Target: resources,
			})

			tasks, successful := syncCtx.getSyncTasks()

			if successful != tt.wantSuccess {
				t.Errorf("successful = %v, want: %v", successful, tt.wantSuccess)
				return
			}

			skipDryRun := false
			for _, task := range tasks {
				if task.targetObj.GetKind() == cr.GetKind() {
					skipDryRun = task.skipDryRun
					break
				}
			}

			assert.Equalf(t, tt.wantDryRun, !skipDryRun, "dryRun = %v, want: %v", !skipDryRun, tt.wantDryRun)
		})
	}
}

func TestSyncSuccessfully(t *testing.T) {
	newSvc := testingutils.NewService()
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false),
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			newSvc.GetName(): health.HealthStatusDegraded, // Always failed
		})))

	pod := testingutils.NewPod()
	pod.SetNamespace(testingutils.FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, pod},
		Target: []*unstructured.Unstructured{newSvc, nil},
	})

	// Since we only have one step, we consider the sync successful if the manifest were applied correctly.
	// In this case, we do not need to run the sync again to evaluate the health
	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Len(t, resources, 2)
	for i := range resources {
		result := resources[i]
		switch result.ResourceKey.Kind {
		case "Pod":
			assert.Equal(t, synccommon.ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		case "Service":
			assert.Equal(t, synccommon.ResultCodeSynced, result.Status)
			assert.Empty(t, result.Message)
		default:
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncSuccessfully_Multistep(t *testing.T) {
	newSvc := testingutils.NewService()
	newSvc.SetNamespace(testingutils.FakeArgoCDNamespace)
	testingutils.Annotate(newSvc, synccommon.AnnotationSyncWave, "0")

	newSvc2 := testingutils.NewService()
	newSvc.SetNamespace(testingutils.FakeArgoCDNamespace)
	newSvc2.SetName("new-svc-2")
	testingutils.Annotate(newSvc2, synccommon.AnnotationSyncWave, "5")

	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false),
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			newSvc.GetName(): health.HealthStatusHealthy,
		})))

	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{newSvc, newSvc2},
	})

	// Since we have multiple step, we need to run the sync again to evaluate the health of current phase
	// (wave 0) and start the new phase (wave 5).
	syncCtx.Sync()
	phase, message, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for healthy state of /Service/my-service", message)
	assert.Len(t, resources, 1)
	assert.Equal(t, kube.GetResourceKey(newSvc), resources[0].ResourceKey)

	// Update the live resources for the next sync
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{newSvc, nil},
		Target: []*unstructured.Unstructured{newSvc, newSvc2},
	})

	syncCtx.Sync()
	phase, message, resources = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Equal(t, "successfully synced (all tasks run)", message)
	assert.Len(t, resources, 2)
}

func TestSyncDeleteSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	svc := testingutils.NewService()
	svc.SetNamespace(testingutils.FakeArgoCDNamespace)
	pod := testingutils.NewPod()
	pod.SetNamespace(testingutils.FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{svc, pod},
		Target: []*unstructured.Unstructured{nil, nil},
	})

	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	for i := range resources {
		result := resources[i]
		switch result.ResourceKey.Kind {
		case "Pod":
			assert.Equal(t, synccommon.ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		case "Service":
			assert.Equal(t, synccommon.ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		default:
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncCreateFailure(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	testSvc := testingutils.NewService()
	mockKubectl := &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			testSvc.GetName(): {
				Output: "",
				Err:    errors.New("foo"),
			},
		},
	}
	syncCtx.kubectl = mockKubectl
	mockResourceOps := &kubetest.MockResourceOps{
		Commands: map[string]kubetest.KubectlOutput{
			testSvc.GetName(): {
				Output: "",
				Err:    errors.New("foo"),
			},
		},
	}
	syncCtx.resourceOps = mockResourceOps
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{testSvc},
	})

	syncCtx.Sync()
	_, _, resources := syncCtx.GetState()

	assert.Len(t, resources, 1)
	result := resources[0]
	assert.Equal(t, synccommon.ResultCodeSyncFailed, result.Status)
	assert.Equal(t, "foo", result.Message)
}

func TestSync_ApplyOutOfSyncOnly(t *testing.T) {
	pod1 := testingutils.NewPod()
	pod1.SetName("pod-1")
	pod1.SetNamespace("fake-argocd-ns")
	pod2 := testingutils.NewPod()
	pod2.SetName("pod-2")
	pod2.SetNamespace("fake-argocd-ns")
	pod3 := testingutils.NewPod()
	pod3.SetName("pod-3")
	pod3.SetNamespace("fake-argocd-ns")

	syncCtx := newTestSyncCtx(nil)
	syncCtx.applyOutOfSyncOnly = true
	t.Run("modificationResult=nil", func(t *testing.T) {
		syncCtx.modificationResult = nil
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3},
			Target: []*unstructured.Unstructured{pod1, nil, pod3},
		})

		syncCtx.Sync()
		phase, _, resources := syncCtx.GetState()
		assert.Equal(t, synccommon.OperationSucceeded, phase)
		assert.Len(t, resources, 3)
	})

	syncCtx = newTestSyncCtx(nil, WithResourceModificationChecker(true, diffResultList()))
	t.Run("applyOutOfSyncOnly=true", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3},
			Target: []*unstructured.Unstructured{pod1, nil, pod3},
		})

		syncCtx.Sync()
		phase, _, resources := syncCtx.GetState()
		assert.Equal(t, synccommon.OperationSucceeded, phase)
		assert.Len(t, resources, 2)
		for _, r := range resources {
			switch r.ResourceKey.Name {
			case "pod-1":
				assert.Equal(t, synccommon.ResultCodeSynced, r.Status)
			case "pod-2":
				assert.Equal(t, synccommon.ResultCodePruneSkipped, r.Status)
			case "pod-3":
				t.Error("pod-3 should have been skipped, as no change")
			}
		}
	})

	pod4 := testingutils.NewPod()
	pod4.SetName("pod-4")
	t.Run("applyOutOfSyncOnly=true and missing resource key", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3, pod4},
			Target: []*unstructured.Unstructured{pod1, nil, pod3, pod4},
		})

		syncCtx.Sync()
		phase, _, resources := syncCtx.GetState()
		assert.Equal(t, synccommon.OperationSucceeded, phase)
		assert.Len(t, resources, 3)
	})

	t.Run("applyOutOfSyncOnly=true and prune=true", func(t *testing.T) {
		syncCtx = newTestSyncCtx(nil, WithResourceModificationChecker(true, diffResultList()))
		syncCtx.applyOutOfSyncOnly = true
		syncCtx.prune = true
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3},
			Target: []*unstructured.Unstructured{pod1, nil, pod3},
		})

		syncCtx.Sync()
		phase, _, resources := syncCtx.GetState()
		assert.Equal(t, synccommon.OperationSucceeded, phase)
		assert.Len(t, resources, 2)
		for _, r := range resources {
			switch r.ResourceKey.Name {
			case "pod-1":
				assert.Equal(t, synccommon.ResultCodeSynced, r.Status)
			case "pod-2":
				assert.Equal(t, synccommon.ResultCodePruned, r.Status)
			case "pod-3":
				t.Error("pod-3 should have been skipped, as no change")
			}
		}
	})

	t.Run("applyOutOfSyncOnly=true and syncwaves", func(t *testing.T) {
		syncCtx = newTestSyncCtx(nil, WithResourceModificationChecker(true, diffResultList()))
		syncCtx.applyOutOfSyncOnly = true
		syncCtx.prune = true
		pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "2"})
		pod2.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "3"})
		pod3.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "1"})

		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3},
			Target: []*unstructured.Unstructured{pod1, nil, pod3},
		})

		syncCtx.Sync()
		phase, _, resources := syncCtx.GetState()
		assert.Equal(t, synccommon.OperationRunning, phase)
		assert.Len(t, resources, 1)
		assert.Equal(t, "pod-1", resources[0].ResourceKey.Name)
		assert.Equal(t, synccommon.ResultCodeSynced, resources[0].Status)
		assert.Equal(t, synccommon.OperationRunning, resources[0].HookPhase)

		syncCtx.Sync()
		phase, _, resources = syncCtx.GetState()
		assert.Equal(t, synccommon.OperationRunning, phase)
		assert.Len(t, resources, 1)
		assert.Equal(t, "pod-1", resources[0].ResourceKey.Name)
		assert.Equal(t, synccommon.ResultCodeSynced, resources[0].Status)
		assert.Equal(t, synccommon.OperationRunning, resources[0].HookPhase)
	})
}

func TestSync_ApplyOutOfSyncOnly_ClusterResources(t *testing.T) {
	ns1 := testingutils.NewNamespace()
	ns1.SetName("ns-1")
	ns1.SetNamespace("")

	ns2 := testingutils.NewNamespace()
	ns2.SetName("ns-2")
	ns1.SetNamespace("")

	ns3 := testingutils.NewNamespace()
	ns3.SetName("ns-3")
	ns3.SetNamespace("")

	ns2Target := testingutils.NewNamespace()
	ns2Target.SetName("ns-2")
	// set namespace for a cluster scoped resource. This is to simulate the behaviour, where the Application's
	// spec.destination.namespace is set for all resources that does not have a namespace set, irrespective of whether
	// the resource is cluster scoped or namespace scoped.
	//
	// Refer to https://github.com/argoproj/argo-cd/gitops-engine/blob/8007df5f6c5dd78a1a8cef73569468ce4d83682c/pkg/sync/sync_context.go#L827-L833
	ns2Target.SetNamespace("ns-2")

	syncCtx := newTestSyncCtx(nil, WithResourceModificationChecker(true, diffResultListClusterResource()))
	syncCtx.applyOutOfSyncOnly = true

	t.Run("cluster resource with target ns having namespace filled", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, ns2, ns3},
			Target: []*unstructured.Unstructured{ns1, ns2Target, ns3},
		})

		syncCtx.Sync()
		phase, _, resources := syncCtx.GetState()
		assert.Equal(t, synccommon.OperationSucceeded, phase)
		assert.Len(t, resources, 1)
		for _, r := range resources {
			switch r.ResourceKey.Name {
			case "ns-1":
				// ns-1 namespace does not exist yet in the cluster, so it must create it and resource must go to
				// synced state.
				assert.Equal(t, synccommon.ResultCodeSynced, r.Status)
			case "ns-2":
				// ns-2 namespace already exist and is synced. However, the target resource has metadata.namespace set for
				// a cluster resource. This namespace must not be synced again, as the object already exists and
				// a change in namespace for a cluster resource has no meaning and hence must not be treated as an
				// out-of-sync resource.
				t.Error("ns-2 should have been skipped, as no change")
			case "ns-3":
				// ns-3 namespace exists and there is no change in the target's metadata.namespace value. So it must not try to sync again.
				t.Error("ns-3 should have been skipped, as no change")
			}
		}
	})
}

func TestSyncPruneFailure(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	mockKubectl := &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			"test-service": {
				Output: "",
				Err:    errors.New("foo"),
			},
		},
	}
	syncCtx.kubectl = mockKubectl
	mockResourceOps := kubetest.MockResourceOps{
		Commands: map[string]kubetest.KubectlOutput{
			"test-service": {
				Output: "",
				Err:    errors.New("foo"),
			},
		},
	}
	syncCtx.resourceOps = &mockResourceOps
	testSvc := testingutils.NewService()
	testSvc.SetName("test-service")
	testSvc.SetNamespace(testingutils.FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{testSvc},
		Target: []*unstructured.Unstructured{testSvc},
	})

	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Len(t, resources, 1)
	result := resources[0]
	assert.Equal(t, synccommon.ResultCodeSyncFailed, result.Status)
	assert.Equal(t, "foo", result.Message)
}

type APIServerMock struct {
	calls       int
	errorStatus int
	errorBody   []byte
}

func (s *APIServerMock) newHttpServer(t *testing.T, apiFailuresCount int) *httptest.Server {
	t.Helper()
	stable := metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Namespaced: true, Kind: "Pod"},
			{Name: "services", Namespaced: true, Kind: "Service", Verbs: metav1.Verbs{"get"}},
			{Name: "namespaces", Namespaced: false, Kind: "Namespace"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s.calls++
		if s.calls <= apiFailuresCount {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(s.errorStatus)
			w.Write(s.errorBody) // nolint:errcheck
			return
		}
		var list any
		switch req.URL.Path {
		case "/api/v1":
			list = &stable
		case "/apis/v1":
			list = &stable
		default:
			t.Logf("unexpected request: %s", req.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		output, err := json.Marshal(list)
		if err != nil {
			t.Errorf("unexpected encoding error: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(output) // nolint:errcheck
	}))
	return server
}

func TestServerResourcesRetry(t *testing.T) {
	type fixture struct {
		apiServerMock *APIServerMock
		httpServer    *httptest.Server
		syncCtx       *syncContext
	}
	setup := func(t *testing.T, apiFailuresCount int) *fixture {
		t.Helper()
		syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, false, false, true))

		unauthorizedStatus := &metav1.Status{
			Status:  metav1.StatusFailure,
			Code:    http.StatusUnauthorized,
			Reason:  metav1.StatusReasonUnauthorized,
			Message: "some error",
		}
		unauthorizedJSON, err := json.Marshal(unauthorizedStatus)
		if err != nil {
			t.Errorf("unexpected encoding error while marshaling unauthorizedStatus: %v", err)
			return nil
		}
		server := &APIServerMock{
			errorStatus: http.StatusUnauthorized,
			errorBody:   unauthorizedJSON,
		}
		httpServer := server.newHttpServer(t, apiFailuresCount)

		syncCtx.disco = discovery.NewDiscoveryClientForConfigOrDie(&rest.Config{Host: httpServer.URL})
		testSvc := testingutils.NewService()
		testSvc.SetName("test-service")
		testSvc.SetNamespace(testingutils.FakeArgoCDNamespace)
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{testSvc, testSvc, testSvc, testSvc},
			Target: []*unstructured.Unstructured{testSvc, testSvc, testSvc, testSvc},
		})
		return &fixture{
			apiServerMock: server,
			httpServer:    httpServer,
			syncCtx:       syncCtx,
		}
	}
	type testCase struct {
		desc               string
		apiFailureCount    int
		apiErrorHTTPStatus int
		expectedAPICalls   int
		expectedResources  int
		expectedPhase      synccommon.OperationPhase
		expectedMessage    string
	}
	testCases := []testCase{
		{
			desc:              "will return success when no api failure",
			apiFailureCount:   0,
			expectedAPICalls:  1,
			expectedResources: 1,
			expectedPhase:     synccommon.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 1 api failure attempt",
			apiFailureCount:   1,
			expectedAPICalls:  2,
			expectedResources: 1,
			expectedPhase:     synccommon.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 2 api failure attempt",
			apiFailureCount:   2,
			expectedAPICalls:  3,
			expectedResources: 1,
			expectedPhase:     synccommon.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 3 api failure attempt",
			apiFailureCount:   3,
			expectedAPICalls:  4,
			expectedResources: 1,
			expectedPhase:     synccommon.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 4 api failure attempt",
			apiFailureCount:   4,
			expectedAPICalls:  5,
			expectedResources: 1,
			expectedPhase:     synccommon.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will fail after 5 api failure attempt",
			apiFailureCount:   5,
			expectedAPICalls:  5,
			expectedResources: 1,
			expectedPhase:     synccommon.OperationFailed,
			expectedMessage:   "not valid",
		},
		{
			desc:               "will not retry if returned error is different than Unauthorized",
			apiErrorHTTPStatus: http.StatusConflict,
			apiFailureCount:    1,
			expectedAPICalls:   1,
			expectedResources:  1,
			expectedPhase:      synccommon.OperationFailed,
			expectedMessage:    "not valid",
		},
	}
	for _, tc := range testCases {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			// Given
			t.Parallel()
			fixture := setup(t, tc.apiFailureCount)
			defer fixture.httpServer.Close()
			if tc.apiErrorHTTPStatus != 0 {
				fixture.apiServerMock.errorStatus = tc.apiErrorHTTPStatus
			}

			// When
			fixture.syncCtx.Sync()
			phase, msg, resources := fixture.syncCtx.GetState()

			// Then
			assert.Equal(t, tc.expectedAPICalls, fixture.apiServerMock.calls, "api calls mismatch")
			assert.Len(t, resources, tc.expectedResources, "resources len mismatch")
			assert.Contains(t, msg, tc.expectedMessage, "expected message mismatch")
			require.Equal(t, tc.expectedPhase, phase, "expected phase mismatch")
			require.Len(t, fixture.syncCtx.syncRes, 1, "sync result len mismatch")
		})
	}
}

func TestDoNotSyncOrPruneHooks(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, false, false, true))
	targetPod := testingutils.NewPod()
	targetPod.SetName("do-not-create-me")
	targetPod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})
	liveSvc := testingutils.NewService()
	liveSvc.SetName("do-not-prune-me")
	liveSvc.SetNamespace(testingutils.FakeArgoCDNamespace)
	liveSvc.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})

	syncCtx.hooks = []*unstructured.Unstructured{targetPod, liveSvc}
	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()
	assert.Empty(t, resources)
	assert.Equal(t, synccommon.OperationSucceeded, phase)
}

// make sure that we do not prune resources with Prune=false
func TestDoNotPrunePruneFalse(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	pod := testingutils.NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "Prune=false"})
	pod.SetNamespace(testingutils.FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{pod},
		Target: []*unstructured.Unstructured{nil},
	})

	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Len(t, resources, 1)
	assert.Equal(t, synccommon.ResultCodePruneSkipped, resources[0].Status)
	assert.Equal(t, "ignored (no prune)", resources[0].Message)

	syncCtx.Sync()

	phase, _, _ = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationSucceeded, phase)
}

func TestPruneConfirm(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	pod := testingutils.NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "Prune=confirm"})
	pod.SetNamespace(testingutils.FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{pod},
		Target: []*unstructured.Unstructured{nil},
	})

	syncCtx.Sync()
	phase, msg, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Empty(t, resources)
	assert.Equal(t, "waiting for pruning confirmation of /Pod/my-pod", msg)

	syncCtx.pruneConfirmed = true
	syncCtx.Sync()

	phase, _, resources = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Len(t, resources, 1)
	assert.Equal(t, synccommon.ResultCodePruned, resources[0].Status)
	assert.Equal(t, "pruned", resources[0].Message)
}

func TestPruneTrueOverridesPruneDisabled(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(true, false, false, false))
	pod := testingutils.NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "Prune=true"})
	pod.SetNamespace(testingutils.FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{pod},
		Target: []*unstructured.Unstructured{nil},
	})

	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	require.Len(t, resources, 1)
	assert.Equal(t, synccommon.ResultCodePruned, resources[0].Status)
	assert.Equal(t, "pruned (dry run)", resources[0].Message)
}

// // make sure Validate=false means we don't validate
func TestSyncOptionValidate(t *testing.T) {
	tests := []struct {
		name          string
		annotationVal string
		want          bool
	}{
		{"Empty", "", true},
		{"True", "Validate=true", true},
		{"False", "Validate=false", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncCtx := newTestSyncCtx(nil)
			pod := testingutils.NewPod()
			pod.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: tt.annotationVal})
			pod.SetNamespace(testingutils.FakeArgoCDNamespace)
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{pod},
				Target: []*unstructured.Unstructured{pod},
			})

			syncCtx.Sync()

			// kubectl, _ := syncCtx.kubectl.(*kubetest.MockKubectlCmd)
			resourceOps, _ := syncCtx.resourceOps.(*kubetest.MockResourceOps)
			assert.Equal(t, tt.want, resourceOps.GetLastValidate())
		})
	}
}

func TestSync_Replace(t *testing.T) {
	testCases := []struct {
		name        string
		target      *unstructured.Unstructured
		live        *unstructured.Unstructured
		commandUsed string
	}{
		{"NoAnnotation", testingutils.NewPod(), testingutils.NewPod(), "apply"},
		{"AnnotationIsSet", withReplaceAnnotation(testingutils.NewPod()), testingutils.NewPod(), "replace"},
		{"AnnotationIsSetOnLive", testingutils.NewPod(), withReplaceAnnotation(testingutils.NewPod()), "replace"},
		{"LiveObjectMissing", withReplaceAnnotation(testingutils.NewPod()), nil, "create"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			syncCtx := newTestSyncCtx(nil)

			tc.target.SetNamespace(testingutils.FakeArgoCDNamespace)
			if tc.live != nil {
				tc.live.SetNamespace(testingutils.FakeArgoCDNamespace)
			}
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{tc.live},
				Target: []*unstructured.Unstructured{tc.target},
			})

			syncCtx.Sync()

			// kubectl, _ := syncCtx.kubectl.(*kubetest.MockKubectlCmd)
			resourceOps, _ := syncCtx.resourceOps.(*kubetest.MockResourceOps)
			assert.Equal(t, tc.commandUsed, resourceOps.GetLastResourceCommand(kube.GetResourceKey(tc.target)))
		})
	}
}

func TestSync_HookWithReplaceAndBeforeHookCreation_AlreadyDeleted(t *testing.T) {
	// This test a race condition when Delete is called on an already deleted object
	// LiveObj is set, but then the resource is deleted asynchronously in kubernetes
	syncCtx := newTestSyncCtx(nil)

	target := withReplaceAnnotation(testingutils.NewPod())
	target.SetNamespace(testingutils.FakeArgoCDNamespace)
	target = testingutils.Annotate(target, synccommon.AnnotationKeyHookDeletePolicy, string(synccommon.HookDeletePolicyBeforeHookCreation))
	target = testingutils.Annotate(target, synccommon.AnnotationKeyHook, string(synccommon.SyncPhasePreSync))
	live := target.DeepCopy()

	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{live},
		Target: []*unstructured.Unstructured{target},
	})
	syncCtx.hooks = []*unstructured.Unstructured{live}

	client := fake.NewSimpleDynamicClient(runtime.NewScheme())
	deleted := false
	client.PrependReactor("delete", "pods", func(_ testcore.Action) (bool, runtime.Object, error) {
		deleted = true
		// simulate the race conditions where liveObj was not null, but is now deleted in k8s
		return true, nil, apierrors.NewNotFound(corev1.Resource("pods"), live.GetName())
	})
	syncCtx.dynamicIf = client

	syncCtx.Sync()

	resourceOps, _ := syncCtx.resourceOps.(*kubetest.MockResourceOps)
	assert.Equal(t, "create", resourceOps.GetLastResourceCommand(kube.GetResourceKey(target)))
	assert.True(t, deleted)
}

func TestSync_ServerSideApply(t *testing.T) {
	testCases := []struct {
		name            string
		target          *unstructured.Unstructured
		live            *unstructured.Unstructured
		commandUsed     string
		serverSideApply bool
		manager         string
	}{
		{"NoAnnotation", testingutils.NewPod(), testingutils.NewPod(), "apply", false, "managerA"},
		{"ServerSideApplyAnnotationIsSet", withServerSideApplyAnnotation(testingutils.NewPod()), testingutils.NewPod(), "apply", true, "managerB"},
		{"DisableServerSideApplyAnnotationIsSet", withDisableServerSideApplyAnnotation(testingutils.NewPod()), testingutils.NewPod(), "apply", false, "managerB"},
		{"ServerSideApplyAndReplaceAnnotationsAreSet", withReplaceAndServerSideApplyAnnotations(testingutils.NewPod()), testingutils.NewPod(), "replace", false, ""},
		{"ServerSideApplyAndReplaceAnnotationsAreSetNamespace", withReplaceAndServerSideApplyAnnotations(testingutils.NewNamespace()), testingutils.NewNamespace(), "update", false, ""},
		{"LiveObjectMissing", withReplaceAnnotation(testingutils.NewPod()), nil, "create", false, ""},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			syncCtx := newTestSyncCtx(nil)
			syncCtx.serverSideApplyManager = tc.manager

			tc.target.SetNamespace(testingutils.FakeArgoCDNamespace)
			if tc.live != nil {
				tc.live.SetNamespace(testingutils.FakeArgoCDNamespace)
			}
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{tc.live},
				Target: []*unstructured.Unstructured{tc.target},
			})

			syncCtx.Sync()

			// kubectl, _ := syncCtx.kubectl.(*kubetest.MockKubectlCmd)
			resourceOps, _ := syncCtx.resourceOps.(*kubetest.MockResourceOps)
			assert.Equal(t, tc.commandUsed, resourceOps.GetLastResourceCommand(kube.GetResourceKey(tc.target)))
			assert.Equal(t, tc.serverSideApply, resourceOps.GetLastServerSideApply())
			assert.Equal(t, tc.manager, resourceOps.GetLastServerSideApplyManager())
		})
	}
}

func TestSyncContext_ServerSideApplyWithDryRun(t *testing.T) {
	tests := []struct {
		name        string
		scDryRun    bool
		dryRun      bool
		expectedSSA bool
		objToUse    func(*unstructured.Unstructured) *unstructured.Unstructured
	}{
		{"BothFlagsFalseAnnotated", false, false, true, withServerSideApplyAnnotation},
		{"scDryRunTrueAnnotated", true, false, false, withServerSideApplyAnnotation},
		{"dryRunTrueAnnotated", false, true, false, withServerSideApplyAnnotation},
		{"BothFlagsTrueAnnotated", true, true, false, withServerSideApplyAnnotation},
		{"AnnotatedDisabledSSA", false, false, false, withDisableServerSideApplyAnnotation},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sc := newTestSyncCtx(nil)
			sc.dryRun = tc.scDryRun
			targetObj := tc.objToUse(testingutils.NewPod())

			// Execute the shouldUseServerSideApply method and assert expectations
			serverSideApply := sc.shouldUseServerSideApply(targetObj, tc.dryRun)
			assert.Equal(t, tc.expectedSSA, serverSideApply)
		})
	}
}

func TestSync_Force(t *testing.T) {
	testCases := []struct {
		name        string
		target      *unstructured.Unstructured
		live        *unstructured.Unstructured
		commandUsed string
		force       bool
	}{
		{"NoAnnotation", testingutils.NewPod(), testingutils.NewPod(), "apply", false},
		{"ForceApplyAnnotationIsSet", withForceAnnotation(testingutils.NewPod()), testingutils.NewPod(), "apply", true},
		{"ForceReplaceAnnotationIsSet", withForceAndReplaceAnnotations(testingutils.NewPod()), testingutils.NewPod(), "replace", true},
		{"ForceReplaceAnnotationIsSetOnLive", testingutils.NewPod(), withForceAndReplaceAnnotations(testingutils.NewPod()), "replace", true},
		{"LiveObjectMissing", withReplaceAnnotation(testingutils.NewPod()), nil, "create", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			syncCtx := newTestSyncCtx(nil)

			tc.target.SetNamespace(testingutils.FakeArgoCDNamespace)
			if tc.live != nil {
				tc.live.SetNamespace(testingutils.FakeArgoCDNamespace)
			}
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{tc.live},
				Target: []*unstructured.Unstructured{tc.target},
			})

			syncCtx.Sync()

			resourceOps, _ := syncCtx.resourceOps.(*kubetest.MockResourceOps)
			assert.Equal(t, tc.commandUsed, resourceOps.GetLastResourceCommand(kube.GetResourceKey(tc.target)))
			assert.Equal(t, tc.force, resourceOps.GetLastForce())
		})
	}
}

func TestSelectiveSyncOnly(t *testing.T) {
	pod1 := testingutils.NewPod()
	pod1.SetName("pod-1")
	pod2 := testingutils.NewPod()
	pod2.SetName("pod-2")
	syncCtx := newTestSyncCtx(nil, WithResourcesFilter(func(key kube.ResourceKey, _ *unstructured.Unstructured, _ *unstructured.Unstructured) bool {
		return key.Kind == pod1.GetKind() && key.Name == pod1.GetName()
	}))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod1},
	})
	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "pod-1", tasks[0].name())
}

func TestUnnamedHooksGetUniqueNames(t *testing.T) {
	t.Run("Truncated revision", func(t *testing.T) {
		syncCtx := newTestSyncCtx(nil)

		pod := testingutils.NewPod()
		pod.SetName("")
		pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync,PostSync"})
		syncCtx.hooks = []*unstructured.Unstructured{pod}

		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 2)
		assert.Contains(t, tasks[0].name(), "foobarb-presync-")
		assert.Contains(t, tasks[1].name(), "foobarb-postsync-")
		assert.Empty(t, pod.GetName())
	})

	t.Run("Short revision", func(t *testing.T) {
		syncCtx := newTestSyncCtx(nil)
		pod := testingutils.NewPod()
		pod.SetName("")
		pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync,PostSync"})
		syncCtx.hooks = []*unstructured.Unstructured{pod}
		syncCtx.revision = "foobar"
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 2)
		assert.Contains(t, tasks[0].name(), "foobar-presync-")
		assert.Contains(t, tasks[1].name(), "foobar-postsync-")
		assert.Empty(t, pod.GetName())
	})
}

func TestManagedResourceAreNotNamed(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	pod := testingutils.NewPod()
	pod.SetName("")

	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Empty(t, tasks[0].name())
	assert.Empty(t, pod.GetName())
}

func TestDeDupingTasks(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	pod := testingutils.NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "Sync"})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.hooks = []*unstructured.Unstructured{pod}

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
}

func TestObjectsGetANamespace(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	pod := testingutils.NewPod()
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Equal(t, testingutils.FakeArgoCDNamespace, tasks[0].namespace())
	assert.Empty(t, pod.GetNamespace())
}

func TestNamespaceAutoCreation(t *testing.T) {
	pod := testingutils.NewPod()
	namespace := testingutils.NewNamespace()
	syncCtx := newTestSyncCtx(nil)
	syncCtx.namespace = testingutils.FakeArgoCDNamespace
	syncCtx.syncNamespace = func(_, _ *unstructured.Unstructured) (bool, error) {
		return true, nil
	}
	namespace.SetName(testingutils.FakeArgoCDNamespace)

	task, err := createNamespaceTask(syncCtx.namespace)
	require.NoError(t, err, "Failed creating test data: namespace task")

	// Namespace auto creation pre-sync task should not be there
	// since there is namespace resource in syncCtx.resources
	t.Run("no pre-sync task if resource is managed", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{namespace},
		})
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 1)
		assert.NotContains(t, tasks, task)
	})

	// Namespace auto creation pre-sync task should be there when it is not managed
	t.Run("pre-sync task when resource is not managed", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 2)
		assert.Contains(t, tasks, task)
	})

	// Namespace auto creation pre-sync task should be there after sync
	t.Run("pre-sync task when resource is not managed with existing sync", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})

		res := synccommon.ResourceSyncResult{
			ResourceKey: kube.GetResourceKey(task.obj()),
			Version:     task.version(),
			Status:      task.syncStatus,
			Message:     task.message,
			HookType:    task.hookType(),
			HookPhase:   task.operationState,
			SyncPhase:   task.phase,
		}
		syncCtx.syncRes = map[string]synccommon.ResourceSyncResult{}
		syncCtx.syncRes[task.resultKey()] = res

		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 2)
		assert.Contains(t, tasks, task)
	})

	// Namespace auto creation pre-sync task not should be there
	// since there is no namespace modifier present
	t.Run("no pre-sync task created if no modifier", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})

		syncCtx.syncNamespace = nil

		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 1)
		assert.NotContains(t, tasks, task)
	})
}

func TestNamespaceAutoCreationForNonExistingNs(t *testing.T) {
	getResourceFunc := func(_ context.Context, _ *rest.Config, _ schema.GroupVersionKind, _ string, _ string) (*unstructured.Unstructured, error) {
		return nil, apierrors.NewNotFound(schema.GroupResource{}, testingutils.FakeArgoCDNamespace)
	}

	pod := testingutils.NewPod()
	namespace := testingutils.NewNamespace()
	syncCtx := newTestSyncCtx(&getResourceFunc)
	syncCtx.namespace = testingutils.FakeArgoCDNamespace
	namespace.SetName(testingutils.FakeArgoCDNamespace)

	t.Run("pre-sync task should exist and namespace creator should be called", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})
		creatorCalled := false
		syncCtx.syncNamespace = func(_, _ *unstructured.Unstructured) (bool, error) {
			creatorCalled = true
			return true, nil
		}
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, creatorCalled)
		assert.True(t, successful)
		assert.Len(t, tasks, 2)
	})

	t.Run("pre-sync task should be not created and namespace creator should be called", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})
		creatorCalled := false
		syncCtx.syncNamespace = func(_, _ *unstructured.Unstructured) (bool, error) {
			creatorCalled = true
			return false, nil
		}
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, creatorCalled)
		assert.True(t, successful)
		assert.Len(t, tasks, 1)
	})

	t.Run("pre-sync task error should be created if namespace creator has an error", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})
		creatorCalled := false
		syncCtx.syncNamespace = func(_, _ *unstructured.Unstructured) (bool, error) {
			creatorCalled = true
			return false, errors.New("some error")
		}
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, creatorCalled)
		assert.True(t, successful)
		assert.Len(t, tasks, 2)
		assert.Equal(t, &syncTask{
			phase:          synccommon.SyncPhasePreSync,
			liveObj:        nil,
			targetObj:      tasks[0].targetObj,
			skipDryRun:     false,
			syncStatus:     synccommon.ResultCodeSyncFailed,
			operationState: synccommon.OperationError,
			message:        "namespaceModifier error: some error",
			waveOverride:   nil,
		}, tasks[0])
	})
}

func TestSync_SuccessfulSyncWithSyncFailHook(t *testing.T) {
	hook := newHook("hook-1", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)
	pod := testingutils.NewPod()
	syncFailHook := newHook("sync-fail-hook", synccommon.HookTypeSyncFail, synccommon.HookDeletePolicyHookSucceeded)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			pod.GetName():  health.HealthStatusHealthy,
			hook.GetName(): health.HealthStatusHealthy,
		})))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook, syncFailHook}
	syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme())

	// First sync does dry-run and starts hooks
	syncCtx.Sync()
	phase, message, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for completion of hook /Pod/hook-1 and 1 more resources", message)
	assert.Len(t, resources, 2)

	// Update the live resources for the next sync
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{pod, hook},
		Target: []*unstructured.Unstructured{pod, nil},
	})

	// Second sync completes hooks
	syncCtx.Sync()
	phase, message, resources = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Equal(t, "successfully synced (no more tasks)", message)
	assert.Len(t, resources, 2)
}

func TestSync_FailedSyncWithSyncFailHook_HookFailed(t *testing.T) {
	hook := newHook("hook-1", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)
	pod := testingutils.NewPod()
	syncFailHook := newHook("sync-fail-hook", synccommon.HookTypeSyncFail, synccommon.HookDeletePolicyHookSucceeded)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			pod.GetName():          health.HealthStatusHealthy,
			hook.GetName():         health.HealthStatusDegraded, // Hook failure
			syncFailHook.GetName(): health.HealthStatusHealthy,
		})))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook, syncFailHook}
	syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme())

	// First sync does dry-run and starts hooks
	syncCtx.Sync()
	phase, message, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for completion of hook /Pod/hook-1 and 1 more resources", message)
	assert.Len(t, resources, 2)

	// Update the live resources for the next sync
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{pod, hook},
		Target: []*unstructured.Unstructured{pod, nil},
	})

	// Second sync fails the sync and starts the SyncFail hooks
	syncCtx.Sync()
	phase, message, resources = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for completion of hook /Pod/sync-fail-hook", message)
	assert.Len(t, resources, 3)

	// Update the live resources for the next sync
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{pod, hook, syncFailHook},
		Target: []*unstructured.Unstructured{pod, nil, nil},
	})

	// Third sync completes the SyncFail hooks
	syncCtx.Sync()
	phase, message, resources = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "one or more synchronization tasks completed unsuccessfully", message)
	assert.Len(t, resources, 3)
}

func TestBeforeHookCreation(t *testing.T) {
	finalizerRemoved := false
	syncCtx := newTestSyncCtx(nil)
	hookObj := testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), synccommon.AnnotationKeyHook, "Sync"), synccommon.AnnotationKeyHookDeletePolicy, "BeforeHookCreation")
	hookObj.SetFinalizers([]string{hook.HookFinalizer})
	hookObj.SetNamespace(testingutils.FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{hookObj},
		Target: []*unstructured.Unstructured{nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hookObj}
	client := fake.NewSimpleDynamicClient(runtime.NewScheme(), hookObj)
	client.PrependReactor("update", "pods", func(_ testcore.Action) (bool, runtime.Object, error) {
		finalizerRemoved = true
		return false, nil, nil
	})
	syncCtx.dynamicIf = client

	// First sync will delete the existing hook
	syncCtx.Sync()
	phase, _, _ := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.True(t, finalizerRemoved)

	// Second sync will create the hook
	syncCtx.Sync()
	phase, message, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Len(t, resources, 1)
	assert.Equal(t, synccommon.OperationRunning, resources[0].HookPhase)
	assert.Equal(t, "waiting for completion of hook /Pod/my-pod", message)
}

func TestSync_ExistingHooksWithFinalizer(t *testing.T) {
	hook1 := newHook("existing-hook-1", synccommon.HookTypePreSync, synccommon.HookDeletePolicyBeforeHookCreation)
	hook2 := newHook("existing-hook-2", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookFailed)
	hook3 := newHook("existing-hook-3", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookSucceeded)

	syncCtx := newTestSyncCtx(nil)
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1, hook2, hook3)
	syncCtx.dynamicIf = fakeDynamicClient
	updatedCount := 0
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// Removing the finalizers
		updatedCount++
		return false, nil, nil
	})
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// because of HookDeletePolicyBeforeHookCreation
		deletedCount++
		return false, nil, nil
	})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{hook1, hook2, hook3},
		Target: []*unstructured.Unstructured{nil, nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1, hook2, hook3}

	syncCtx.Sync()
	phase, message, _ := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for deletion of hook /Pod/existing-hook-1", message)
	assert.Equal(t, 3, updatedCount)
	assert.Equal(t, 1, deletedCount)

	_, err := syncCtx.getResource(&syncTask{liveObj: hook1})
	require.Error(t, err, "Expected resource to be deleted")
	assert.True(t, apierrors.IsNotFound(err))
}

func TestSync_FailedSyncWithSyncFailHook_ApplyFailed(t *testing.T) {
	// Tests that other SyncFail Hooks run even if one of them fail.
	pod := testingutils.NewPod()
	pod.SetNamespace(testingutils.FakeArgoCDNamespace)
	successfulSyncFailHook := newHook("successful-sync-fail-hook", synccommon.HookTypeSyncFail, synccommon.HookDeletePolicyBeforeHookCreation)
	failedSyncFailHook := newHook("failed-sync-fail-hook", synccommon.HookTypeSyncFail, synccommon.HookDeletePolicyBeforeHookCreation)

	syncCtx := newTestSyncCtx(nil, WithHealthOverride(resourceNameHealthOverride{
		successfulSyncFailHook.GetName(): health.HealthStatusHealthy,
	}))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.hooks = []*unstructured.Unstructured{successfulSyncFailHook, failedSyncFailHook}
	syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme())

	mockResourceOps := kubetest.MockResourceOps{
		Commands: map[string]kubetest.KubectlOutput{
			// Fail operation
			pod.GetName(): {Err: errors.New("fake pod failure")},
			// Fail a single SyncFail hook
			failedSyncFailHook.GetName(): {Err: errors.New("fake hook failure")},
		},
	}
	syncCtx.resourceOps = &mockResourceOps

	// First sync triggers the SyncFail hooks on failure
	syncCtx.Sync()
	phase, message, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for completion of hook /Pod/failed-sync-fail-hook and 1 more hooks", message)
	assert.Len(t, resources, 3)
	podResult := getResourceResult(resources, kube.GetResourceKey(pod))
	require.NotNil(t, podResult, "%s not found", kube.GetResourceKey(pod))
	assert.Equal(t, synccommon.OperationFailed, podResult.HookPhase)
	assert.Equal(t, synccommon.ResultCodeSyncFailed, podResult.Status)

	failedSyncFailHookResult := getResourceResult(resources, kube.GetResourceKey(failedSyncFailHook))
	require.NotNil(t, failedSyncFailHookResult, "%s not found", kube.GetResourceKey(failedSyncFailHook))
	assert.Equal(t, synccommon.OperationFailed, failedSyncFailHookResult.HookPhase)
	assert.Equal(t, synccommon.ResultCodeSyncFailed, failedSyncFailHookResult.Status)

	successfulSyncFailHookResult := getResourceResult(resources, kube.GetResourceKey(successfulSyncFailHook))
	require.NotNil(t, successfulSyncFailHookResult, "%s not found", kube.GetResourceKey(successfulSyncFailHook))
	assert.Equal(t, synccommon.OperationRunning, successfulSyncFailHookResult.HookPhase)
	assert.Equal(t, synccommon.ResultCodeSynced, successfulSyncFailHookResult.Status)

	// Update the live state for the next run
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, successfulSyncFailHook},
		Target: []*unstructured.Unstructured{pod, nil},
	})

	// Second sync completes when the SyncFail hooks are done
	syncCtx.Sync()
	phase, message, resources = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "one or more synchronization tasks completed unsuccessfully, reason: fake pod failure\none or more SyncFail hooks failed, reason: fake hook failure", message)
	assert.Len(t, resources, 3)

	successfulSyncFailHookResult = getResourceResult(resources, kube.GetResourceKey(successfulSyncFailHook))
	require.NotNil(t, successfulSyncFailHookResult, "%s not found", kube.GetResourceKey(successfulSyncFailHook))
	assert.Equal(t, synccommon.OperationSucceeded, successfulSyncFailHookResult.HookPhase)
	assert.Equal(t, synccommon.ResultCodeSynced, successfulSyncFailHookResult.Status)
}

func TestSync_HooksNotDeletedIfPhaseNotCompleted(t *testing.T) {
	hook1 := newHook("hook-1", synccommon.HookTypePreSync, synccommon.HookDeletePolicyBeforeHookCreation)
	hook2 := newHook("hook-2", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookFailed)
	hook3 := newHook("hook-3", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookSucceeded)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			hook1.GetName(): health.HealthStatusProgressing, // At least one is running
			hook2.GetName(): health.HealthStatusHealthy,
			hook3.GetName(): health.HealthStatusHealthy,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(hook1),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       0,
		}, {
			ResourceKey: kube.GetResourceKey(hook2),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       1,
		}, {
			ResourceKey: kube.GetResourceKey(hook3),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       2,
		}},
			metav1.Now(),
		))
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1, hook2, hook3)
	syncCtx.dynamicIf = fakeDynamicClient
	updatedCount := 0
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// Removing the finalizers
		updatedCount++
		return false, nil, nil
	})
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount++
		return false, nil, nil
	})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{hook1, hook2, hook3},
		Target: []*unstructured.Unstructured{nil, nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1, hook2, hook3}

	syncCtx.Sync()
	phase, message, _ := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for completion of hook /Pod/hook-1", message)
	assert.Equal(t, 0, updatedCount)
	assert.Equal(t, 0, deletedCount)
}

func TestSync_HooksDeletedAfterSyncSucceeded(t *testing.T) {
	hook1 := newHook("hook-1", synccommon.HookTypePreSync, synccommon.HookDeletePolicyBeforeHookCreation)
	hook2 := newHook("hook-2", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookFailed)
	hook3 := newHook("hook-3", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookSucceeded)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			hook1.GetName(): health.HealthStatusHealthy,
			hook2.GetName(): health.HealthStatusHealthy,
			hook3.GetName(): health.HealthStatusHealthy,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(hook1),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       0,
		}, {
			ResourceKey: kube.GetResourceKey(hook2),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       1,
		}, {
			ResourceKey: kube.GetResourceKey(hook3),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       2,
		}},
			metav1.Now(),
		))
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1, hook2, hook3)
	syncCtx.dynamicIf = fakeDynamicClient
	updatedCount := 0
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// Removing the finalizers
		updatedCount++
		return false, nil, nil
	})
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount++
		return false, nil, nil
	})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{hook1, hook2, hook3},
		Target: []*unstructured.Unstructured{nil, nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1, hook2, hook3}

	syncCtx.Sync()
	phase, message, _ := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Equal(t, "successfully synced (no more tasks)", message)
	assert.Equal(t, 3, updatedCount)
	assert.Equal(t, 1, deletedCount)

	_, err := syncCtx.getResource(&syncTask{liveObj: hook3})
	require.Error(t, err, "Expected resource to be deleted")
	assert.True(t, apierrors.IsNotFound(err))
}

func TestSync_HooksDeletedAfterSyncFailed(t *testing.T) {
	hook1 := newHook("hook-1", synccommon.HookTypePreSync, synccommon.HookDeletePolicyBeforeHookCreation)
	hook2 := newHook("hook-2", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookFailed)
	hook3 := newHook("hook-3", synccommon.HookTypePreSync, synccommon.HookDeletePolicyHookSucceeded)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			hook1.GetName(): health.HealthStatusDegraded, // At least one has failed
			hook2.GetName(): health.HealthStatusHealthy,
			hook3.GetName(): health.HealthStatusHealthy,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(hook1),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       0,
		}, {
			ResourceKey: kube.GetResourceKey(hook2),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       1,
		}, {
			ResourceKey: kube.GetResourceKey(hook3),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhasePreSync,
			Order:       2,
		}},
			metav1.Now(),
		))
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1, hook2, hook3)
	syncCtx.dynamicIf = fakeDynamicClient
	updatedCount := 0
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// Removing the finalizers
		updatedCount++
		return false, nil, nil
	})
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount++
		return false, nil, nil
	})

	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{hook1, hook2, hook3},
		Target: []*unstructured.Unstructured{nil, nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1, hook2, hook3}

	syncCtx.Sync()
	phase, message, _ := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "one or more synchronization tasks completed unsuccessfully", message)
	assert.Equal(t, 3, updatedCount)
	assert.Equal(t, 1, deletedCount)

	_, err := syncCtx.getResource(&syncTask{liveObj: hook2})
	require.Error(t, err, "Expected resource to be deleted")
	assert.True(t, apierrors.IsNotFound(err))
}

func Test_syncContext_liveObj(t *testing.T) {
	type fields struct {
		compareResult ReconciliationResult
	}
	type args struct {
		obj *unstructured.Unstructured
	}
	obj := testingutils.NewPod()
	obj.SetNamespace("my-ns")

	found := testingutils.NewPod()
	foundNoNamespace := testingutils.NewPod()
	foundNoNamespace.SetNamespace("")

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *unstructured.Unstructured
	}{
		{"None", fields{compareResult: ReconciliationResult{}}, args{obj: &unstructured.Unstructured{}}, nil},
		{"Found", fields{compareResult: ReconciliationResult{Target: []*unstructured.Unstructured{nil}, Live: []*unstructured.Unstructured{found}}}, args{obj: obj}, found},
		{"EmptyNamespace", fields{compareResult: ReconciliationResult{Target: []*unstructured.Unstructured{nil}, Live: []*unstructured.Unstructured{foundNoNamespace}}}, args{obj: obj}, found},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &syncContext{
				resources: groupResources(tt.fields.compareResult),
				hooks:     tt.fields.compareResult.Hooks,
			}
			got := sc.liveObj(tt.args.obj)
			assert.Truef(t, reflect.DeepEqual(got, tt.want), "syncContext.liveObj() = %v, want %v", got, tt.want)
		})
	}
}

func Test_syncContext_hasCRDOfGroupKind(t *testing.T) {
	// target
	assert.False(t, (&syncContext{resources: groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{testingutils.NewCRD()},
	})}).hasCRDOfGroupKind("", ""))
	assert.True(t, (&syncContext{resources: groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{testingutils.NewCRD()},
	})}).hasCRDOfGroupKind("test.io", "TestCrd"))

	// hook
	assert.False(t, (&syncContext{hooks: []*unstructured.Unstructured{testingutils.NewCRD()}}).hasCRDOfGroupKind("", ""))
	assert.True(t, (&syncContext{hooks: []*unstructured.Unstructured{testingutils.NewCRD()}}).hasCRDOfGroupKind("test.io", "TestCrd"))
}

func Test_setRunningPhase(t *testing.T) {
	newPodTask := func(name string) *syncTask {
		pod := testingutils.NewPod()
		pod.SetName(name)
		return &syncTask{targetObj: pod}
	}
	newHookTask := func(name string, hookType synccommon.HookType) *syncTask {
		return &syncTask{targetObj: newHook(name, hookType, synccommon.HookDeletePolicyBeforeHookCreation)}
	}

	tests := []struct {
		name              string
		tasks             syncTasks
		isPendingDeletion bool
		expectedMessage   string
	}{
		{
			name:              "empty tasks",
			tasks:             syncTasks{},
			isPendingDeletion: false,
			expectedMessage:   "",
		},
		{
			name:              "single resource - healthy state",
			tasks:             syncTasks{newPodTask("my-pod")},
			isPendingDeletion: false,
			expectedMessage:   "waiting for healthy state of /Pod/my-pod",
		},
		{
			name:              "multiple resources - healthy state",
			tasks:             syncTasks{newPodTask("pod-1"), newPodTask("pod-2"), newPodTask("pod-3")},
			isPendingDeletion: false,
			expectedMessage:   "waiting for healthy state of /Pod/pod-1 and 2 more resources",
		},
		{
			name:              "single hook - completion",
			tasks:             syncTasks{newHookTask("hook-1", synccommon.HookTypeSync)},
			isPendingDeletion: false,
			expectedMessage:   "waiting for completion of hook /Pod/hook-1",
		},
		{
			name:              "multiple hooks - completion",
			tasks:             syncTasks{newHookTask("hook-1", synccommon.HookTypeSync), newHookTask("hook-2", synccommon.HookTypeSync)},
			isPendingDeletion: false,
			expectedMessage:   "waiting for completion of hook /Pod/hook-1 and 1 more hooks",
		},
		{
			name:              "hooks and resources - prioritizes hooks",
			tasks:             syncTasks{newPodTask("pod-1"), newHookTask("hook-1", synccommon.HookTypeSync), newPodTask("pod-2")},
			isPendingDeletion: false,
			expectedMessage:   "waiting for completion of hook /Pod/hook-1 and 2 more resources",
		},
		{
			name:              "single resource - pending deletion",
			tasks:             syncTasks{newPodTask("my-pod")},
			isPendingDeletion: true,
			expectedMessage:   "waiting for deletion of /Pod/my-pod",
		},
		{
			name:              "multiple resources - pending deletion",
			tasks:             syncTasks{newPodTask("pod-1"), newPodTask("pod-2"), newPodTask("pod-3")},
			isPendingDeletion: true,
			expectedMessage:   "waiting for deletion of /Pod/pod-1 and 2 more resources",
		},
		{
			name:              "single hook - pending deletion",
			tasks:             syncTasks{newHookTask("hook-1", synccommon.HookTypeSync)},
			isPendingDeletion: true,
			expectedMessage:   "waiting for deletion of hook /Pod/hook-1",
		},
		{
			name:              "multiple hooks - pending deletion",
			tasks:             syncTasks{newHookTask("hook-1", synccommon.HookTypeSync), newHookTask("hook-2", synccommon.HookTypeSync)},
			isPendingDeletion: true,
			expectedMessage:   "waiting for deletion of hook /Pod/hook-1 and 1 more hooks",
		},
		{
			name:              "hooks and resources - pending deletion prioritizes hooks",
			tasks:             syncTasks{newPodTask("pod-1"), newHookTask("hook-1", synccommon.HookTypeSync), newPodTask("pod-2")},
			isPendingDeletion: true,
			expectedMessage:   "waiting for deletion of hook /Pod/hook-1 and 2 more resources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sc syncContext
			sc.log = textlogger.NewLogger(textlogger.NewConfig()).WithValues("application", "fake-app")

			sc.setRunningPhase(tt.tasks, tt.isPendingDeletion)

			assert.Equal(t, synccommon.OperationRunning, sc.phase)
			assert.Equal(t, tt.expectedMessage, sc.message)
		})
	}
}

func TestSync_SyncWaveHook(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, false, false, false))
	pod1 := testingutils.NewPod()
	pod1.SetName("pod-1")
	pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "-1"})
	pod2 := testingutils.NewPod()
	pod2.SetName("pod-2")
	pod3 := testingutils.NewPod()
	pod3.SetName("pod-3")
	pod3.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PostSync"})

	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{pod1, pod2},
	})
	syncCtx.hooks = []*unstructured.Unstructured{pod3}

	called := false
	syncCtx.syncWaveHook = func(phase synccommon.SyncPhase, wave int, final bool) error {
		called = true
		assert.Equal(t, synccommon.SyncPhaseSync, string(phase))
		assert.Equal(t, -1, wave)
		assert.False(t, final)
		return nil
	}
	syncCtx.Sync()
	assert.True(t, called)

	// call sync again, it should not invoke the SyncWaveHook callback since we only should be
	// doing this after an apply, and not every reconciliation
	called = false
	syncCtx.syncWaveHook = func(_ synccommon.SyncPhase, _ int, _ bool) error {
		called = true
		return nil
	}
	syncCtx.Sync()
	assert.False(t, called)

	// complete wave -1, then call Sync again. Verify we invoke another SyncWaveHook call after applying wave 0
	_, _, results := syncCtx.GetState()
	pod1Res := results[0]
	pod1Res.HookPhase = synccommon.OperationSucceeded
	syncCtx.syncRes[resourceResultKey(pod1Res.ResourceKey, synccommon.SyncPhaseSync)] = pod1Res
	called = false
	syncCtx.syncWaveHook = func(phase synccommon.SyncPhase, wave int, final bool) error {
		called = true
		assert.Equal(t, synccommon.SyncPhaseSync, string(phase))
		assert.Equal(t, 0, wave)
		assert.False(t, final)
		return nil
	}
	syncCtx.Sync()
	assert.True(t, called)

	// complete wave 0. after applying PostSync, we should perform callback and final should be set true
	_, _, results = syncCtx.GetState()
	pod2Res := results[1]
	pod2Res.HookPhase = synccommon.OperationSucceeded
	syncCtx.syncRes[resourceResultKey(pod2Res.ResourceKey, synccommon.SyncPhaseSync)] = pod2Res
	called = false
	syncCtx.syncWaveHook = func(phase synccommon.SyncPhase, wave int, final bool) error {
		called = true
		assert.Equal(t, synccommon.SyncPhasePostSync, string(phase))
		assert.Equal(t, 0, wave)
		assert.True(t, final)
		return nil
	}
	syncCtx.Sync()
	assert.True(t, called)
}

func TestSync_SyncWaveHookError(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, false, false, false))
	pod1 := testingutils.NewPod()
	pod1.SetNamespace(testingutils.FakeArgoCDNamespace)
	pod1.SetName("pod-1")

	syncHook := newHook("sync-hook", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)
	syncFailHook := newHook("sync-fail-hook", synccommon.HookTypeSyncFail, synccommon.HookDeletePolicyBeforeHookCreation)

	syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme())
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod1},
	})
	syncCtx.hooks = []*unstructured.Unstructured{syncHook, syncFailHook}

	called := false
	syncCtx.syncWaveHook = func(_ synccommon.SyncPhase, _ int, _ bool) error {
		called = true
		return errors.New("intentional error")
	}
	syncCtx.Sync()
	assert.True(t, called)
	phase, msg, results := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationError, phase)
	assert.Equal(t, "SyncWaveHook failed: intentional error", msg)
	require.Len(t, results, 2)

	podResult := getResourceResult(results, kube.GetResourceKey(pod1))
	require.NotNil(t, podResult, "%s not found", kube.GetResourceKey(pod1))
	// Hook phase can be either Running or Succeeded due to timing, what matters is the operation failed
	assert.Contains(t, []synccommon.OperationPhase{synccommon.OperationRunning, synccommon.OperationSucceeded}, podResult.HookPhase)

	hookResult := getResourceResult(results, kube.GetResourceKey(syncHook))
	require.NotNil(t, hookResult, "%s not found", kube.GetResourceKey(syncHook))
	assert.Equal(t, "Terminated", hookResult.Message)
}

func TestPruneLast(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	syncCtx.pruneLast = true

	pod1 := testingutils.NewPod()
	pod1.SetName("pod-1")
	pod2 := testingutils.NewPod()
	pod2.SetName("pod-2")
	pod3 := testingutils.NewPod()
	pod3.SetName("pod-3")

	t.Run("syncPhaseSameWave", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3},
			Target: []*unstructured.Unstructured{pod1, nil, nil},
		})
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 3)
		// last wave is the last sync wave for non-prune task + 1
		assert.Equal(t, 1, tasks.lastWave())
	})

	t.Run("syncPhaseDifferentWave", func(t *testing.T) {
		pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "2"})
		pod2.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "1"})
		pod3.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "7"})
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3},
			Target: []*unstructured.Unstructured{pod1, nil, nil},
		})
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 3)
		// last wave is the last sync wave for tasks + 1
		assert.Equal(t, 8, tasks.lastWave())
	})

	t.Run("pruneLastIndividualResources", func(t *testing.T) {
		syncCtx.pruneLast = false

		pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "2"})
		pod2.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "1", synccommon.AnnotationSyncOptions: synccommon.SyncOptionPruneLast})
		pod3.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "7", synccommon.AnnotationSyncOptions: synccommon.SyncOptionPruneLast})
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod2, pod3},
			Target: []*unstructured.Unstructured{pod1, nil, nil},
		})
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 3)
		// last wave is the last sync wave for tasks + 1
		assert.Equal(t, 8, tasks.lastWave())
	})
}

func diffResultList() *diff.DiffResultList {
	pod1 := testingutils.NewPod()
	pod1.SetName("pod-1")
	pod1.SetNamespace(testingutils.FakeArgoCDNamespace)
	pod2 := testingutils.NewPod()
	pod2.SetName("pod-2")
	pod2.SetNamespace(testingutils.FakeArgoCDNamespace)
	pod3 := testingutils.NewPod()
	pod3.SetName("pod-3")
	pod3.SetNamespace(testingutils.FakeArgoCDNamespace)

	diffResultList := diff.DiffResultList{
		Modified: true,
		Diffs:    []diff.DiffResult{},
	}

	podBytes, _ := json.Marshal(pod1)
	diffResultList.Diffs = append(diffResultList.Diffs, diff.DiffResult{NormalizedLive: []byte("null"), PredictedLive: podBytes, Modified: true})

	podBytes, _ = json.Marshal(pod2)
	diffResultList.Diffs = append(diffResultList.Diffs, diff.DiffResult{NormalizedLive: podBytes, PredictedLive: []byte("null"), Modified: true})

	podBytes, _ = json.Marshal(pod3)
	diffResultList.Diffs = append(diffResultList.Diffs, diff.DiffResult{NormalizedLive: podBytes, PredictedLive: podBytes, Modified: false})

	return &diffResultList
}

func TestSyncContext_GetDeleteOptions_Default(t *testing.T) {
	sc := syncContext{}
	opts := sc.getDeleteOptions()
	assert.Equal(t, metav1.DeletePropagationForeground, *opts.PropagationPolicy)
}

func TestSyncContext_GetDeleteOptions_WithPrunePropagationPolicy(t *testing.T) {
	sc := syncContext{}

	policy := metav1.DeletePropagationBackground
	WithPrunePropagationPolicy(&policy)(&sc)

	opts := sc.getDeleteOptions()
	assert.Equal(t, metav1.DeletePropagationBackground, *opts.PropagationPolicy)
}

func Test_executeSyncFailPhase(t *testing.T) {
	sc := syncContext{}
	sc.log = textlogger.NewLogger(textlogger.NewConfig()).WithValues("application", "fake-app")

	tasks := make([]*syncTask, 0)
	tasks = append(tasks, &syncTask{message: "namespace not found"})

	sc.executeSyncFailPhase(nil, tasks, "one or more objects failed to apply")

	assert.Equal(t, "one or more objects failed to apply, reason: namespace not found", sc.message)
}

func Test_executeSyncFailPhase_DuplicatedMessages(t *testing.T) {
	sc := syncContext{}
	sc.log = textlogger.NewLogger(textlogger.NewConfig()).WithValues("application", "fake-app")

	tasks := make([]*syncTask, 0)
	tasks = append(tasks, &syncTask{message: "namespace not found"})
	tasks = append(tasks, &syncTask{message: "namespace not found"})

	sc.executeSyncFailPhase(nil, tasks, "one or more objects failed to apply")

	assert.Equal(t, "one or more objects failed to apply, reason: namespace not found", sc.message)
}

func Test_executeSyncFailPhase_NoTasks(t *testing.T) {
	sc := syncContext{}
	sc.log = textlogger.NewLogger(textlogger.NewConfig()).WithValues("application", "fake-app")

	sc.executeSyncFailPhase(nil, nil, "one or more objects failed to apply")

	assert.Equal(t, "one or more objects failed to apply", sc.message)
}

func Test_executeSyncFailPhase_RunningHooks(t *testing.T) {
	sc := syncContext{
		phase: synccommon.OperationRunning,
	}
	sc.log = textlogger.NewLogger(textlogger.NewConfig()).WithValues("application", "fake-app")

	tasks := make([]*syncTask, 0)
	tasks = append(tasks, &syncTask{operationState: synccommon.OperationRunning})

	sc.executeSyncFailPhase(tasks, nil, "one or more objects failed to apply")

	assert.Equal(t, synccommon.OperationRunning, sc.phase)
}

func Test_executeSyncFailPhase_CompletedHooks(t *testing.T) {
	sc := syncContext{
		phase: synccommon.OperationRunning,
	}
	sc.log = textlogger.NewLogger(textlogger.NewConfig()).WithValues("application", "fake-app")

	failed := make([]*syncTask, 0)
	failed = append(failed, &syncTask{message: "task in error"})

	tasks := make([]*syncTask, 0)
	tasks = append(tasks, &syncTask{operationState: synccommon.OperationSucceeded})
	tasks = append(tasks, &syncTask{operationState: synccommon.OperationFailed, syncStatus: synccommon.ResultCodeSyncFailed, message: "failed to apply"})

	sc.executeSyncFailPhase(tasks, failed, "one or more objects failed to apply")

	assert.Equal(t, "one or more objects failed to apply, reason: task in error\none or more SyncFail hooks failed, reason: failed to apply", sc.message)
	assert.Equal(t, synccommon.OperationFailed, sc.phase)
}

func TestWaveReorderingOfPruneTasks(t *testing.T) {
	ns := testingutils.NewNamespace()
	ns.SetName("ns")
	pod1 := testingutils.NewPod()
	pod1.SetName("pod-1")
	pod2 := testingutils.NewPod()
	pod2.SetName("pod-2")
	pod3 := testingutils.NewPod()
	pod3.SetName("pod-3")
	pod4 := testingutils.NewPod()
	pod4.SetName("pod-4")
	pod5 := testingutils.NewPod()
	pod5.SetName("pod-5")
	pod6 := testingutils.NewPod()
	pod6.SetName("pod-6")
	pod7 := testingutils.NewPod()
	pod7.SetName("pod-7")

	type Test struct {
		name              string
		target            []*unstructured.Unstructured
		live              []*unstructured.Unstructured
		expectedWaveOrder map[string]int
		pruneLast         bool
	}
	runTest := func(test Test) {
		t.Run(test.name, func(t *testing.T) {
			syncCtx := newTestSyncCtx(nil)
			syncCtx.pruneLast = test.pruneLast
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   test.live,
				Target: test.target,
			})
			tasks, successful := syncCtx.getSyncTasks()

			assert.True(t, successful)
			assert.Len(t, tasks, len(test.target))

			for _, task := range tasks {
				assert.Equal(t, test.expectedWaveOrder[task.name()], task.wave())
			}
		})
	}

	// same wave
	sameWaveTests := []Test{
		{
			name:   "sameWave_noPruneTasks",
			live:   []*unstructured.Unstructured{nil, nil, nil, nil, nil},
			target: []*unstructured.Unstructured{ns, pod1, pod2, pod3, pod4},
			// no change in wave order
			expectedWaveOrder: map[string]int{ns.GetName(): 0, pod1.GetName(): 0, pod2.GetName(): 0, pod3.GetName(): 0, pod4.GetName(): 0},
		},
		{
			name:   "sameWave_allPruneTasks",
			live:   []*unstructured.Unstructured{ns, pod1, pod2, pod3, pod4},
			target: []*unstructured.Unstructured{nil, nil, nil, nil, nil},
			// no change in wave order
			expectedWaveOrder: map[string]int{ns.GetName(): 0, pod1.GetName(): 0, pod2.GetName(): 0, pod3.GetName(): 0, pod4.GetName(): 0},
		},
		{
			name:   "sameWave_mixedTasks",
			live:   []*unstructured.Unstructured{ns, pod1, nil, pod3, pod4},
			target: []*unstructured.Unstructured{ns, nil, pod2, nil, nil},
			// no change in wave order
			expectedWaveOrder: map[string]int{ns.GetName(): 0, pod1.GetName(): 0, pod2.GetName(): 0, pod3.GetName(): 0, pod4.GetName(): 0},
		},
	}

	for _, test := range sameWaveTests {
		runTest(test)
	}

	// different wave
	differentWaveTests := []Test{
		{
			name:   "differentWave_noPruneTasks",
			target: []*unstructured.Unstructured{ns, pod1, pod2, pod3, pod4},
			live:   []*unstructured.Unstructured{nil, nil, nil, nil, nil},
			// no change in wave order
			expectedWaveOrder: map[string]int{
				// new wave 		// original wave
				ns.GetName():   0, // 0
				pod1.GetName(): 1, // 1
				pod2.GetName(): 2, // 2
				pod3.GetName(): 3, // 3
				pod4.GetName(): 4, // 4
			},
		},
		{
			name:   "differentWave_allPruneTasks",
			target: []*unstructured.Unstructured{nil, nil, nil, nil, nil},
			live:   []*unstructured.Unstructured{ns, pod1, pod2, pod3, pod4},
			// change in prune wave order
			expectedWaveOrder: map[string]int{
				// new wave 		// original wave
				ns.GetName():   4, // 0
				pod1.GetName(): 3, // 1
				pod2.GetName(): 2, // 2
				pod3.GetName(): 1, // 3
				pod4.GetName(): 0, // 4
			},
		},
		{
			name:   "differentWave_mixedTasks",
			target: []*unstructured.Unstructured{ns, nil, pod2, nil, nil},
			live:   []*unstructured.Unstructured{ns, pod1, nil, pod3, pod4},
			// change in prune wave order
			expectedWaveOrder: map[string]int{
				// new wave 		// original wave
				pod1.GetName(): 4, // 1
				pod3.GetName(): 3, // 3
				pod4.GetName(): 1, // 4

				// no change since non prune tasks
				ns.GetName():   0, // 0
				pod2.GetName(): 2, // 2
			},
		},
	}

	for _, test := range differentWaveTests {
		ns.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "0"})
		pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "1"})
		pod2.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "2"})
		pod3.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "3"})
		pod4.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "4"})

		runTest(test)
	}

	// prune last
	pruneLastTests := []Test{
		{
			name:      "pruneLast",
			pruneLast: true,
			live:      []*unstructured.Unstructured{ns, pod1, pod2, pod3, pod4},
			target:    []*unstructured.Unstructured{ns, nil, nil, nil, nil},
			// change in prune wave order
			expectedWaveOrder: map[string]int{
				// new wave 		// original wave
				pod1.GetName(): 5, // 1
				pod2.GetName(): 5, // 2
				pod3.GetName(): 5, // 3
				pod4.GetName(): 5, // 4

				// no change since non prune tasks
				ns.GetName(): 0, // 0
			},
		},
		{
			name:      "pruneLastIndividualResources",
			pruneLast: false,
			live:      []*unstructured.Unstructured{ns, pod1, pod2, pod3, pod4},
			target:    []*unstructured.Unstructured{ns, nil, nil, nil, nil},
			// change in wave order
			expectedWaveOrder: map[string]int{
				// new wave 		// original wave
				pod1.GetName(): 4, // 1
				pod2.GetName(): 5, // 2
				pod3.GetName(): 2, // 3
				pod4.GetName(): 1, // 4

				// no change since non prune tasks
				ns.GetName(): 0, // 0
			},
		},
	}

	for _, test := range pruneLastTests {
		ns.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "0"})
		pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "1"})
		pod2.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "2", synccommon.AnnotationSyncOptions: synccommon.SyncOptionPruneLast})
		pod3.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "3"})
		pod4.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "4"})

		runTest(test)
	}

	// additional test
	tests := []Test{
		{
			name:   "mixedTasks",
			target: []*unstructured.Unstructured{ns, nil, pod2, nil, nil, nil, pod6, nil},
			live:   []*unstructured.Unstructured{ns, pod1, nil, pod3, pod4, pod5, pod6, pod7},
			// change in prune wave order
			expectedWaveOrder: map[string]int{
				// new wave 		// original wave
				pod1.GetName(): 5, // 1
				pod3.GetName(): 4, // 3
				pod4.GetName(): 4, // 3
				pod5.GetName(): 3, // 4
				pod7.GetName(): 1, // 5

				// no change since non prune tasks
				ns.GetName():   -1, // -1
				pod2.GetName(): 3,  // 3
				pod6.GetName(): 5,  // 5
			},
		},
	}
	for _, test := range tests {
		ns.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "-1"})
		pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "1"})
		pod2.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "3"})
		pod3.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "3"})
		pod4.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "3"})
		pod5.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "4"})
		pod6.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "5"})
		pod7.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "5"})

		runTest(test)
	}
}

func TestWaitForCleanUpBeforeNextWave(t *testing.T) {
	pod1 := testingutils.NewPod()
	pod1.SetName("pod-1")
	pod2 := testingutils.NewPod()
	pod2.SetName("pod-2")
	pod3 := testingutils.NewPod()
	pod3.SetName("pod-3")

	pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "1"})
	pod2.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "2"})
	pod3.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "3"})

	syncCtx := newTestSyncCtx(nil)
	syncCtx.prune = true

	// prune order : pod3 -> pod2 -> pod1
	syncCtx.resources = groupResources(ReconciliationResult{
		Target: []*unstructured.Unstructured{nil, nil, nil},
		Live:   []*unstructured.Unstructured{pod1, pod2, pod3},
	})

	var phase synccommon.OperationPhase
	var msg string
	var results []synccommon.ResourceSyncResult

	// 1st sync should prune only pod3
	syncCtx.Sync()
	phase, _, results = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Len(t, results, 1)
	assert.Equal(t, "pod-3", results[0].ResourceKey.Name)
	assert.Equal(t, synccommon.ResultCodePruned, results[0].Status)

	// simulate successful delete of pod3
	syncCtx.resources = groupResources(ReconciliationResult{
		Target: []*unstructured.Unstructured{nil, nil},
		Live:   []*unstructured.Unstructured{pod1, pod2},
	})

	// next sync should prune only pod2
	syncCtx.Sync()
	phase, _, results = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Len(t, results, 2)
	assert.Equal(t, "pod-2", results[1].ResourceKey.Name)
	assert.Equal(t, synccommon.ResultCodePruned, results[1].Status)

	// add delete timestamp on pod2 to simulate pending delete
	pod2.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})

	// next sync should wait for deletion of pod2 from cluster,
	// it should not move to next wave and prune pod1
	syncCtx.Sync()
	phase, msg, results = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationRunning, phase)
	assert.Equal(t, "waiting for deletion of /Pod/pod-2", msg)
	assert.Len(t, results, 2)

	// simulate successful delete of pod2
	syncCtx.resources = groupResources(ReconciliationResult{
		Target: []*unstructured.Unstructured{nil},
		Live:   []*unstructured.Unstructured{pod1},
	})

	// next sync should proceed with next wave
	// i.e deletion of pod1
	syncCtx.Sync()
	phase, _, results = syncCtx.GetState()
	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Len(t, results, 3)
	assert.Equal(t, "pod-3", results[0].ResourceKey.Name)
	assert.Equal(t, "pod-2", results[1].ResourceKey.Name)
	assert.Equal(t, "pod-1", results[2].ResourceKey.Name)
	assert.Equal(t, synccommon.ResultCodePruned, results[0].Status)
	assert.Equal(t, synccommon.ResultCodePruned, results[1].Status)
	assert.Equal(t, synccommon.ResultCodePruned, results[2].Status)
}

func BenchmarkSync(b *testing.B) {
	podManifest := `{
	  "apiVersion": "v1",
	  "kind": "Pod",
	  "metadata": {
		"name": "my-pod"
	  },
	  "spec": {
		"containers": [
		${containers}
		]
	  }
	}`
	container := `{
			"image": "nginx:1.7.9",
			"name": "nginx",
			"resources": {
			  "requests": {
				"cpu": "0.2"
			  }
			}
		  }`

	maxContainers := 10
	for i := 0; i < b.N; i++ {
		b.StopTimer()

		containerCount := min(i+1, maxContainers)

		containerStr := strings.Repeat(container+",", containerCount)
		containerStr = containerStr[:len(containerStr)-1]

		manifest := strings.ReplaceAll(podManifest, "${containers}", containerStr)
		pod := testingutils.Unstructured(manifest)
		pod.SetNamespace(testingutils.FakeArgoCDNamespace)

		syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
		syncCtx.log = logr.Discard()
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil, pod},
			Target: []*unstructured.Unstructured{testingutils.NewService(), nil},
		})

		b.StartTimer()
		syncCtx.Sync()
	}
}

func TestNeedsClientSideApplyMigration(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)

	tests := []struct {
		name     string
		liveObj  *unstructured.Unstructured
		expected bool
	}{
		{
			name:     "nil object",
			liveObj:  nil,
			expected: false,
		},
		{
			name:     "object with no managed fields",
			liveObj:  testingutils.NewPod(),
			expected: false,
		},
		{
			name: "object with kubectl-client-side-apply fields",
			liveObj: func() *unstructured.Unstructured {
				obj := testingutils.NewPod()
				obj.SetManagedFields([]metav1.ManagedFieldsEntry{
					{
						Manager:   "kubectl-client-side-apply",
						Operation: metav1.ManagedFieldsOperationUpdate,
						FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:annotations":{}}}`)},
					},
				})
				return obj
			}(),
			expected: true,
		},
		{
			name: "object with only argocd-controller fields",
			liveObj: func() *unstructured.Unstructured {
				obj := testingutils.NewPod()
				obj.SetManagedFields([]metav1.ManagedFieldsEntry{
					{
						Manager:   "argocd-controller",
						Operation: metav1.ManagedFieldsOperationApply,
						FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)},
					},
				})
				return obj
			}(),
			expected: false,
		},
		{
			name: "object with mixed field managers",
			liveObj: func() *unstructured.Unstructured {
				obj := testingutils.NewPod()
				obj.SetManagedFields([]metav1.ManagedFieldsEntry{
					{
						Manager:   "kubectl-client-side-apply",
						Operation: metav1.ManagedFieldsOperationUpdate,
						FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:annotations":{}}}`)},
					},
					{
						Manager:   "argocd-controller",
						Operation: metav1.ManagedFieldsOperationApply,
						FieldsV1:  &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)},
					},
				})
				return obj
			}(),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := syncCtx.needsClientSideApplyMigration(tt.liveObj, "kubectl-client-side-apply")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func diffResultListClusterResource() *diff.DiffResultList {
	ns1 := testingutils.NewNamespace()
	ns1.SetName("ns-1")
	ns2 := testingutils.NewNamespace()
	ns2.SetName("ns-2")
	ns3 := testingutils.NewNamespace()
	ns3.SetName("ns-3")

	diffResultList := diff.DiffResultList{
		Modified: true,
		Diffs:    []diff.DiffResult{},
	}

	nsBytes, _ := json.Marshal(ns1)
	diffResultList.Diffs = append(diffResultList.Diffs, diff.DiffResult{NormalizedLive: nsBytes, PredictedLive: nsBytes, Modified: false})

	nsBytes, _ = json.Marshal(ns2)
	diffResultList.Diffs = append(diffResultList.Diffs, diff.DiffResult{NormalizedLive: nsBytes, PredictedLive: nsBytes, Modified: false})

	nsBytes, _ = json.Marshal(ns3)
	diffResultList.Diffs = append(diffResultList.Diffs, diff.DiffResult{NormalizedLive: nsBytes, PredictedLive: nsBytes, Modified: false})

	return &diffResultList
}

func TestTerminate(t *testing.T) {
	obj := testingutils.NewPod()
	obj.SetNamespace(testingutils.FakeArgoCDNamespace)

	syncCtx := newTestSyncCtx(nil,
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(obj),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
		}},
			metav1.Now(),
		))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{obj},
		Target: []*unstructured.Unstructured{obj},
	})
	syncCtx.hooks = []*unstructured.Unstructured{}

	syncCtx.Terminate()
	assert.Equal(t, synccommon.OperationFailed, syncCtx.phase)
	assert.Equal(t, "Operation terminated", syncCtx.message)
}

func TestTerminate_Hooks_Running(t *testing.T) {
	hook1 := newHook("hook-1", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)
	hook2 := newHook("hook-2", synccommon.HookTypeSync, synccommon.HookDeletePolicyHookFailed)
	hook3 := newHook("hook-3", synccommon.HookTypeSync, synccommon.HookDeletePolicyHookSucceeded)

	obj := testingutils.NewPod()
	obj.SetNamespace(testingutils.FakeArgoCDNamespace)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			hook1.GetName(): health.HealthStatusProgressing,
			hook2.GetName(): health.HealthStatusProgressing,
			hook3.GetName(): health.HealthStatusProgressing,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(hook1),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       0,
		}, {
			ResourceKey: kube.GetResourceKey(hook2),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       1,
		}, {
			ResourceKey: kube.GetResourceKey(hook3),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       2,
		}, {
			ResourceKey: kube.GetResourceKey(obj),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       3,
		}},
			metav1.Now(),
		))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{obj, hook1, hook2, hook3},
		Target: []*unstructured.Unstructured{obj, nil, nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1, hook2, hook3}
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1, hook2, hook3)
	syncCtx.dynamicIf = fakeDynamicClient
	updatedCount := 0
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// Removing the finalizers
		updatedCount++
		return false, nil, nil
	})
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount++
		return false, nil, nil
	})

	syncCtx.Terminate()
	phase, message, results := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "Operation terminated", message)
	require.Len(t, results, 4)
	assert.Equal(t, kube.GetResourceKey(hook1), results[0].ResourceKey)
	assert.Equal(t, synccommon.OperationFailed, results[0].HookPhase)
	assert.Equal(t, "Terminated", results[0].Message)
	assert.Equal(t, kube.GetResourceKey(hook2), results[1].ResourceKey)
	assert.Equal(t, synccommon.OperationFailed, results[1].HookPhase)
	assert.Equal(t, "Terminated", results[1].Message)
	assert.Equal(t, kube.GetResourceKey(hook3), results[2].ResourceKey)
	assert.Equal(t, synccommon.OperationFailed, results[2].HookPhase)
	assert.Equal(t, "Terminated", results[2].Message)
	assert.Equal(t, 3, updatedCount)
	assert.Equal(t, 3, deletedCount)
}

func TestTerminate_Hooks_Running_Healthy(t *testing.T) {
	hook1 := newHook("hook-1", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)
	hook2 := newHook("hook-2", synccommon.HookTypeSync, synccommon.HookDeletePolicyHookFailed)
	hook3 := newHook("hook-3", synccommon.HookTypeSync, synccommon.HookDeletePolicyHookSucceeded)

	obj := testingutils.NewPod()
	obj.SetNamespace(testingutils.FakeArgoCDNamespace)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			hook1.GetName(): health.HealthStatusHealthy,
			hook2.GetName(): health.HealthStatusHealthy,
			hook3.GetName(): health.HealthStatusHealthy,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(hook1),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       0,
		}, {
			ResourceKey: kube.GetResourceKey(hook2),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       1,
		}, {
			ResourceKey: kube.GetResourceKey(hook3),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       2,
		}, {
			ResourceKey: kube.GetResourceKey(obj),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       3,
		}},
			metav1.Now(),
		))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{obj, hook1, hook2, hook3},
		Target: []*unstructured.Unstructured{obj, nil, nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1, hook2, hook3}
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1, hook2, hook3)
	syncCtx.dynamicIf = fakeDynamicClient
	updatedCount := 0
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// Removing the finalizers
		updatedCount++
		return false, nil, nil
	})
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount++
		return false, nil, nil
	})

	syncCtx.Terminate()
	phase, message, results := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "Operation terminated", message)
	require.Len(t, results, 4)
	assert.Equal(t, kube.GetResourceKey(hook1), results[0].ResourceKey)
	assert.Equal(t, synccommon.OperationSucceeded, results[0].HookPhase)
	assert.Equal(t, "test", results[0].Message)
	assert.Equal(t, kube.GetResourceKey(hook2), results[1].ResourceKey)
	assert.Equal(t, synccommon.OperationSucceeded, results[1].HookPhase)
	assert.Equal(t, "test", results[1].Message)
	assert.Equal(t, kube.GetResourceKey(hook3), results[2].ResourceKey)
	assert.Equal(t, synccommon.OperationSucceeded, results[2].HookPhase)
	assert.Equal(t, "test", results[2].Message)
	assert.Equal(t, 3, updatedCount)
	assert.Equal(t, 1, deletedCount)

	_, err := syncCtx.getResource(&syncTask{liveObj: hook2})
	require.Error(t, err, "Expected resource to be deleted")
	assert.True(t, apierrors.IsNotFound(err))
}

func TestTerminate_Hooks_Completed(t *testing.T) {
	hook1 := newHook("hook-1", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)
	hook2 := newHook("hook-2", synccommon.HookTypeSync, synccommon.HookDeletePolicyHookFailed)
	hook3 := newHook("hook-3", synccommon.HookTypeSync, synccommon.HookDeletePolicyHookSucceeded)

	obj := testingutils.NewPod()
	obj.SetNamespace(testingutils.FakeArgoCDNamespace)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			hook1.GetName(): health.HealthStatusHealthy,
			hook2.GetName(): health.HealthStatusHealthy,
			hook3.GetName(): health.HealthStatusHealthy,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(hook1),
			HookPhase:   synccommon.OperationSucceeded,
			Message:     "hook1 completed",
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       0,
		}, {
			ResourceKey: kube.GetResourceKey(hook2),
			HookPhase:   synccommon.OperationFailed,
			Message:     "hook2 failed",
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       1,
		}, {
			ResourceKey: kube.GetResourceKey(hook3),
			HookPhase:   synccommon.OperationError,
			Message:     "hook3 error",
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       2,
		}, {
			ResourceKey: kube.GetResourceKey(obj),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       3,
		}},
			metav1.Now(),
		))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{obj, hook1, hook2, hook3},
		Target: []*unstructured.Unstructured{obj, nil, nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1, hook2, hook3}
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1, hook2, hook3)
	syncCtx.dynamicIf = fakeDynamicClient
	updatedCount := 0
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		// Removing the finalizers
		updatedCount++
		return false, nil, nil
	})
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount++
		return false, nil, nil
	})

	syncCtx.Terminate()
	phase, message, results := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "Operation terminated", message)
	require.Len(t, results, 4)
	assert.Equal(t, kube.GetResourceKey(hook1), results[0].ResourceKey)
	assert.Equal(t, synccommon.OperationSucceeded, results[0].HookPhase)
	assert.Equal(t, "hook1 completed", results[0].Message)
	assert.Equal(t, kube.GetResourceKey(hook2), results[1].ResourceKey)
	assert.Equal(t, synccommon.OperationFailed, results[1].HookPhase)
	assert.Equal(t, "hook2 failed", results[1].Message)
	assert.Equal(t, kube.GetResourceKey(hook3), results[2].ResourceKey)
	assert.Equal(t, synccommon.OperationError, results[2].HookPhase)
	assert.Equal(t, "hook3 error", results[2].Message)
	assert.Equal(t, 3, updatedCount)
	assert.Equal(t, 0, deletedCount)
}

func TestTerminate_Hooks_Error(t *testing.T) {
	hook1 := newHook("hook-1", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)

	obj := testingutils.NewPod()
	obj.SetNamespace(testingutils.FakeArgoCDNamespace)

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			hook1.GetName(): health.HealthStatusHealthy,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(hook1),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       0,
		}, {
			ResourceKey: kube.GetResourceKey(obj),
			HookPhase:   synccommon.OperationRunning,
			Status:      synccommon.ResultCodeSynced,
			SyncPhase:   synccommon.SyncPhaseSync,
			Order:       1,
		}},
			metav1.Now(),
		))
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{obj, hook1},
		Target: []*unstructured.Unstructured{obj, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook1}
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), hook1)
	syncCtx.dynamicIf = fakeDynamicClient
	fakeDynamicClient.PrependReactor("update", "*", func(_ testcore.Action) (handled bool, ret runtime.Object, err error) {
		return true, nil, apierrors.NewInternalError(errors.New("update failed"))
	})

	syncCtx.Terminate()
	phase, message, results := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationError, phase)
	assert.Equal(t, "Operation termination had errors", message)
	require.Len(t, results, 2)
	assert.Equal(t, kube.GetResourceKey(hook1), results[0].ResourceKey)
	assert.Equal(t, synccommon.OperationError, results[0].HookPhase)
	assert.Contains(t, results[0].Message, "update failed")
}
