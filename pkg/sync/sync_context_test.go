package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"
	"k8s.io/klog/v2/klogr"

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	. "github.com/argoproj/gitops-engine/pkg/utils/testing"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

var standardVerbs = v1.Verbs{"create", "delete", "deletecollection", "get", "list", "patch", "update", "watch"}

func newTestSyncCtx(getResourceFunc *func(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error), opts ...SyncOpt) *syncContext {
	fakeDisco := &fakedisco.FakeDiscovery{Fake: &testcore.Fake{}}
	fakeDisco.Resources = append(make([]*v1.APIResourceList, 0),
		&v1.APIResourceList{
			GroupVersion: "v1",
			APIResources: []v1.APIResource{
				{Kind: "Pod", Group: "", Version: "v1", Namespaced: true, Verbs: standardVerbs},
				{Kind: "Service", Group: "", Version: "v1", Namespaced: true, Verbs: standardVerbs},
				{Kind: "Namespace", Group: "", Version: "v1", Namespaced: false, Verbs: standardVerbs},
			},
		},
		&v1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []v1.APIResource{
				{Kind: "Deployment", Group: "apps", Version: "v1", Namespaced: true, Verbs: standardVerbs},
			},
		})
	sc := syncContext{
		config:    &rest.Config{},
		rawConfig: &rest.Config{},
		namespace: FakeArgoCDNamespace,
		revision:  "FooBarBaz",
		disco:     fakeDisco,
		log:       klogr.New().WithValues("application", "fake-app"),
		resources: map[kube.ResourceKey]reconciledResource{},
		syncRes:   map[string]synccommon.ResourceSyncResult{},
		validate:  true,
	}
	sc.permissionValidator = func(un *unstructured.Unstructured, res *v1.APIResource) error {
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
	pod := NewPod()
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
	syncCtx := newTestSyncCtx(nil, WithPermissionValidator(func(un *unstructured.Unstructured, res *v1.APIResource) error {
		return fmt.Errorf("not permitted in project")
	}))
	targetPod := NewPod()
	targetPod.SetNamespace("kube-system")
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{targetPod, NewService()},
	})
	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Contains(t, resources[0].Message, "not permitted in project")
}

func TestSyncCreateInSortedOrder(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, nil},
		Target: []*unstructured.Unstructured{NewPod(), NewService()},
	})
	syncCtx.Sync()

	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Len(t, resources, 2)
	for i := range resources {
		result := resources[i]
		if result.ResourceKey.Kind == "Pod" {
			assert.Equal(t, synccommon.ResultCodeSynced, result.Status)
			assert.Equal(t, "", result.Message)
		} else if result.ResourceKey.Kind == "Service" {
			assert.Equal(t, "", result.Message)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncCustomResources(t *testing.T) {
	type fields struct {
		skipDryRunAnnotationPresent bool
		crdAlreadyPresent           bool
		crdInSameSync               bool
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			knownCustomResourceTypes := []v1.APIResource{}
			if tt.fields.crdAlreadyPresent {
				knownCustomResourceTypes = append(knownCustomResourceTypes, v1.APIResource{Kind: "TestCrd", Group: "argoproj.io", Version: "v1", Namespaced: true, Verbs: standardVerbs})
			}

			syncCtx := newTestSyncCtx(nil)
			fakeDisco := syncCtx.disco.(*fakedisco.FakeDiscovery)
			fakeDisco.Resources = []*v1.APIResourceList{{
				GroupVersion: "argoproj.io/v1",
				APIResources: knownCustomResourceTypes,
			},
				{
					GroupVersion: "apiextensions.k8s.io/v1beta1",
					APIResources: []v1.APIResource{
						{Kind: "CustomResourceDefinition", Group: "apiextensions.k8s.io", Version: "v1beta1", Namespaced: true, Verbs: standardVerbs},
					},
				},
			}

			cr := testingutils.Unstructured(`
{
  "apiVersion": "argoproj.io/v1",
  "kind": "TestCrd",
  "metadata": {
    "name": "my-resource"
  }
}
`)

			if tt.fields.skipDryRunAnnotationPresent {
				cr.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "SkipDryRunOnMissingResource=true"})
			}

			resources := []*unstructured.Unstructured{cr}
			if tt.fields.crdInSameSync {
				resources = append(resources, NewCRD())
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

			if tt.wantDryRun != !skipDryRun {
				t.Errorf("dryRun = %v, want: %v", !skipDryRun, tt.wantDryRun)
			}
		})
	}

}

func TestSyncSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	pod := NewPod()
	pod.SetNamespace(FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil, pod},
		Target: []*unstructured.Unstructured{NewService(), nil},
	})

	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	assert.Len(t, resources, 2)
	for i := range resources {
		result := resources[i]
		if result.ResourceKey.Kind == "Pod" {
			assert.Equal(t, synccommon.ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		} else if result.ResourceKey.Kind == "Service" {
			assert.Equal(t, synccommon.ResultCodeSynced, result.Status)
			assert.Equal(t, "", result.Message)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncDeleteSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	svc := NewService()
	svc.SetNamespace(FakeArgoCDNamespace)
	pod := NewPod()
	pod.SetNamespace(FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{svc, pod},
		Target: []*unstructured.Unstructured{nil, nil},
	})

	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	assert.Equal(t, synccommon.OperationSucceeded, phase)
	for i := range resources {
		result := resources[i]
		if result.ResourceKey.Kind == "Pod" {
			assert.Equal(t, synccommon.ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		} else if result.ResourceKey.Kind == "Service" {
			assert.Equal(t, synccommon.ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncCreateFailure(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	testSvc := NewService()
	mockKubectl := &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			testSvc.GetName(): {
				Output: "",
				Err:    fmt.Errorf("foo"),
			},
		},
	}
	syncCtx.kubectl = mockKubectl
	mockResourceOps := &kubetest.MockResourceOps{
		Commands: map[string]kubetest.KubectlOutput{
			testSvc.GetName(): {
				Output: "",
				Err:    fmt.Errorf("foo"),
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
	pod1 := NewPod()
	pod1.SetName("pod-1")
	pod2 := NewPod()
	pod2.SetName("pod-2")
	pod3 := NewPod()
	pod3.SetName("pod-3")

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

	pod4 := NewPod()
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

func TestSyncPruneFailure(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	mockKubectl := &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			"test-service": {
				Output: "",
				Err:    fmt.Errorf("foo"),
			},
		},
	}
	syncCtx.kubectl = mockKubectl
	mockResourceOps := kubetest.MockResourceOps{
		Commands: map[string]kubetest.KubectlOutput{
			"test-service": {
				Output: "",
				Err:    fmt.Errorf("foo"),
			},
		},
	}
	syncCtx.resourceOps = &mockResourceOps
	testSvc := NewService()
	testSvc.SetName("test-service")
	testSvc.SetNamespace(FakeArgoCDNamespace)
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
			{Name: "services", Namespaced: true, Kind: "Service", Verbs: v1.Verbs{"get"}},
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
		var list interface{}
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
		testSvc := NewService()
		testSvc.SetName("test-service")
		testSvc.SetNamespace(FakeArgoCDNamespace)
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
			expectedPhase:     common.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 1 api failure attempt",
			apiFailureCount:   1,
			expectedAPICalls:  2,
			expectedResources: 1,
			expectedPhase:     common.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 2 api failure attempt",
			apiFailureCount:   2,
			expectedAPICalls:  3,
			expectedResources: 1,
			expectedPhase:     common.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 3 api failure attempt",
			apiFailureCount:   3,
			expectedAPICalls:  4,
			expectedResources: 1,
			expectedPhase:     common.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will return success after 4 api failure attempt",
			apiFailureCount:   4,
			expectedAPICalls:  5,
			expectedResources: 1,
			expectedPhase:     common.OperationSucceeded,
			expectedMessage:   "success",
		},
		{
			desc:              "will fail after 5 api failure attempt",
			apiFailureCount:   5,
			expectedAPICalls:  5,
			expectedResources: 1,
			expectedPhase:     common.OperationFailed,
			expectedMessage:   "not valid",
		},
		{
			desc:               "will not retry if returned error is different than Unauthorized",
			apiErrorHTTPStatus: http.StatusConflict,
			apiFailureCount:    1,
			expectedAPICalls:   1,
			expectedResources:  1,
			expectedPhase:      common.OperationFailed,
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
	targetPod := NewPod()
	targetPod.SetName("do-not-create-me")
	targetPod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})
	liveSvc := NewService()
	liveSvc.SetName("do-not-prune-me")
	liveSvc.SetNamespace(FakeArgoCDNamespace)
	liveSvc.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})

	syncCtx.hooks = []*unstructured.Unstructured{targetPod, liveSvc}
	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()
	assert.Len(t, resources, 0)
	assert.Equal(t, synccommon.OperationSucceeded, phase)
}

// make sure that we do not prune resources with Prune=false
func TestDoNotPrunePruneFalse(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	pod := NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "Prune=false"})
	pod.SetNamespace(FakeArgoCDNamespace)
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
			pod := NewPod()
			pod.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: tt.annotationVal})
			pod.SetNamespace(FakeArgoCDNamespace)
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

func withReplaceAnnotation(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: synccommon.SyncOptionReplace})
	return un
}

func TestSync_Replace(t *testing.T) {
	testCases := []struct {
		name        string
		target      *unstructured.Unstructured
		live        *unstructured.Unstructured
		commandUsed string
	}{
		{"NoAnnotation", NewPod(), NewPod(), "apply"},
		{"AnnotationIsSet", withReplaceAnnotation(NewPod()), NewPod(), "replace"},
		{"LiveObjectMissing", withReplaceAnnotation(NewPod()), nil, "create"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			syncCtx := newTestSyncCtx(nil)

			tc.target.SetNamespace(FakeArgoCDNamespace)
			if tc.live != nil {
				tc.live.SetNamespace(FakeArgoCDNamespace)
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

func withServerSideApplyAnnotation(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: synccommon.SyncOptionServerSideApply})
	return un
}

func withReplaceAndServerSideApplyAnnotations(un *unstructured.Unstructured) *unstructured.Unstructured {
	un.SetAnnotations(map[string]string{synccommon.AnnotationSyncOptions: "Replace=true,ServerSideApply=true"})
	return un
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
		{"NoAnnotation", NewPod(), NewPod(), "apply", false, "managerA"},
		{"ServerSideApplyAnnotationIsSet", withServerSideApplyAnnotation(NewPod()), NewPod(), "apply", true, "managerB"},
		{"ServerSideApplyAndReplaceAnnotationsAreSet", withReplaceAndServerSideApplyAnnotations(NewPod()), NewPod(), "replace", false, ""},
		{"ServerSideApplyAndReplaceAnnotationsAreSetNamespace", withReplaceAndServerSideApplyAnnotations(NewNamespace()), NewNamespace(), "update", false, ""},
		{"LiveObjectMissing", withReplaceAnnotation(NewPod()), nil, "create", false, ""},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			syncCtx := newTestSyncCtx(nil)
			syncCtx.serverSideApplyManager = tc.manager

			tc.target.SetNamespace(FakeArgoCDNamespace)
			if tc.live != nil {
				tc.live.SetNamespace(FakeArgoCDNamespace)
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

func TestSelectiveSyncOnly(t *testing.T) {
	pod1 := NewPod()
	pod1.SetName("pod-1")
	pod2 := NewPod()
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

		pod := NewPod()
		pod.SetName("")
		pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync,PostSync"})
		syncCtx.hooks = []*unstructured.Unstructured{pod}

		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 2)
		assert.Contains(t, tasks[0].name(), "foobarb-presync-")
		assert.Contains(t, tasks[1].name(), "foobarb-postsync-")
		assert.Equal(t, "", pod.GetName())
	})

	t.Run("Short revision", func(t *testing.T) {
		syncCtx := newTestSyncCtx(nil)
		pod := NewPod()
		pod.SetName("")
		pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync,PostSync"})
		syncCtx.hooks = []*unstructured.Unstructured{pod}
		syncCtx.revision = "foobar"
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 2)
		assert.Contains(t, tasks[0].name(), "foobar-presync-")
		assert.Contains(t, tasks[1].name(), "foobar-postsync-")
		assert.Equal(t, "", pod.GetName())
	})
}

func TestManagedResourceAreNotNamed(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	pod := NewPod()
	pod.SetName("")

	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "", tasks[0].name())
	assert.Equal(t, "", pod.GetName())
}

func TestDeDupingTasks(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, true, false, false))
	pod := NewPod()
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
	pod := NewPod()
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Equal(t, FakeArgoCDNamespace, tasks[0].namespace())
	assert.Equal(t, "", pod.GetNamespace())
}

func TestNamespaceAutoCreation(t *testing.T) {
	pod := NewPod()
	namespace := NewNamespace()
	syncCtx := newTestSyncCtx(nil)
	syncCtx.namespace = FakeArgoCDNamespace
	syncCtx.syncNamespace = func(m, l *unstructured.Unstructured) (bool, error) {
		return true, nil
	}
	namespace.SetName(FakeArgoCDNamespace)

	task, err := createNamespaceTask(syncCtx.namespace)
	assert.NoError(t, err, "Failed creating test data: namespace task")

	//Namespace auto creation pre-sync task should not be there
	//since there is namespace resource in syncCtx.resources
	t.Run("no pre-sync task 1", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{namespace},
		})
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 1)
		assert.NotContains(t, tasks, task)
	})

	//Namespace auto creation pre-sync task should not be there
	//since there is no existing sync result
	t.Run("no pre-sync task 2", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})
		tasks, successful := syncCtx.getSyncTasks()

		assert.True(t, successful)
		assert.Len(t, tasks, 1)
		assert.NotContains(t, tasks, task)
	})

	//Namespace auto creation pre-sync task should be there
	//since there is existing sync result which means that task created this namespace
	t.Run("pre-sync task created", func(t *testing.T) {
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

	//Namespace auto creation pre-sync task not should be there
	//since there is no namespace modifier present
	t.Run("no pre-sync task created", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})
		syncCtx.syncNamespace = nil

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
		assert.Len(t, tasks, 1)
		assert.NotContains(t, tasks, task)
	})
}

func TestNamespaceAutoCreationForNonExistingNs(t *testing.T) {
	getResourceFunc := func(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string) (*unstructured.Unstructured, error) {
		return nil, apierrors.NewNotFound(schema.GroupResource{}, FakeArgoCDNamespace)
	}

	pod := NewPod()
	namespace := NewNamespace()
	syncCtx := newTestSyncCtx(&getResourceFunc)
	syncCtx.namespace = FakeArgoCDNamespace
	namespace.SetName(FakeArgoCDNamespace)

	t.Run("pre-sync task should exist and namespace creator should be called", func(t *testing.T) {
		syncCtx.resources = groupResources(ReconciliationResult{
			Live:   []*unstructured.Unstructured{nil},
			Target: []*unstructured.Unstructured{pod},
		})
		creatorCalled := false
		syncCtx.syncNamespace = func(m, l *unstructured.Unstructured) (bool, error) {
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
		syncCtx.syncNamespace = func(m, l *unstructured.Unstructured) (bool, error) {
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
		syncCtx.syncNamespace = func(m, l *unstructured.Unstructured) (bool, error) {
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

func createNamespaceTask(namespace string) (*syncTask, error) {
	nsSpec := &corev1.Namespace{TypeMeta: v1.TypeMeta{APIVersion: "v1", Kind: kube.NamespaceKind}, ObjectMeta: v1.ObjectMeta{Name: namespace}}
	unstructuredObj, err := kube.ToUnstructured(nsSpec)

	task := &syncTask{phase: synccommon.SyncPhasePreSync, targetObj: unstructuredObj}
	return task, err
}

func TestSyncFailureHookWithSuccessfulSync(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{NewPod()},
	})
	syncCtx.hooks = []*unstructured.Unstructured{newHook(synccommon.HookTypeSyncFail)}

	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationSucceeded, phase)
	// only one result, we did not run the failure failureHook
	assert.Len(t, resources, 1)
}

func TestSyncFailureHookWithFailedSync(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	pod := NewPod()
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.hooks = []*unstructured.Unstructured{newHook(synccommon.HookTypeSyncFail)}
	mockKubectl := &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{pod.GetName(): {Err: fmt.Errorf("")}},
	}
	syncCtx.kubectl = mockKubectl
	mockResourceOps := kubetest.MockResourceOps{
		Commands: map[string]kubetest.KubectlOutput{pod.GetName(): {Err: fmt.Errorf("")}},
	}
	syncCtx.resourceOps = &mockResourceOps

	syncCtx.Sync()
	syncCtx.Sync()

	phase, _, resources := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Len(t, resources, 2)
}

func TestBeforeHookCreation(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	hook := Annotate(Annotate(NewPod(), synccommon.AnnotationKeyHook, "Sync"), synccommon.AnnotationKeyHookDeletePolicy, "BeforeHookCreation")
	hook.SetNamespace(FakeArgoCDNamespace)
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{hook},
		Target: []*unstructured.Unstructured{nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{hook}
	syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme())

	syncCtx.Sync()

	_, _, resources := syncCtx.GetState()
	assert.Len(t, resources, 1)
	assert.Empty(t, resources[0].Message)
	assert.Equal(t, "waiting for completion of hook /Pod/my-pod", syncCtx.message)
}

func TestRunSyncFailHooksFailed(t *testing.T) {
	// Tests that other SyncFail Hooks run even if one of them fail.

	syncCtx := newTestSyncCtx(nil)
	pod := NewPod()
	successfulSyncFailHook := newHook(synccommon.HookTypeSyncFail)
	successfulSyncFailHook.SetName("successful-sync-fail-hook")
	failedSyncFailHook := newHook(synccommon.HookTypeSyncFail)
	failedSyncFailHook.SetName("failed-sync-fail-hook")
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod},
	})
	syncCtx.hooks = []*unstructured.Unstructured{successfulSyncFailHook, failedSyncFailHook}

	mockKubectl := &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			// Fail operation
			pod.GetName(): {Err: fmt.Errorf("")},
			// Fail a single SyncFail hook
			failedSyncFailHook.GetName(): {Err: fmt.Errorf("")}},
	}
	syncCtx.kubectl = mockKubectl
	mockResourceOps := kubetest.MockResourceOps{
		Commands: map[string]kubetest.KubectlOutput{
			// Fail operation
			pod.GetName(): {Err: fmt.Errorf("")},
			// Fail a single SyncFail hook
			failedSyncFailHook.GetName(): {Err: fmt.Errorf("")}},
	}
	syncCtx.resourceOps = &mockResourceOps

	syncCtx.Sync()
	syncCtx.Sync()
	phase, _, resources := syncCtx.GetState()

	// Operation as a whole should fail
	assert.Equal(t, synccommon.OperationFailed, phase)
	// failedSyncFailHook should fail
	assert.Equal(t, synccommon.OperationFailed, resources[1].HookPhase)
	assert.Equal(t, synccommon.ResultCodeSyncFailed, resources[1].Status)
	// successfulSyncFailHook should be synced running (it is an nginx pod)
	assert.Equal(t, synccommon.OperationRunning, resources[2].HookPhase)
	assert.Equal(t, synccommon.ResultCodeSynced, resources[2].Status)
}

type resourceNameHealthOverride map[string]health.HealthStatusCode

func (r resourceNameHealthOverride) GetResourceHealth(obj *unstructured.Unstructured) (*health.HealthStatus, error) {
	if status, ok := r[obj.GetName()]; ok {
		return &health.HealthStatus{Status: status, Message: "test"}, nil
	}
	return nil, nil
}

func TestRunSync_HooksNotDeletedIfPhaseNotCompleted(t *testing.T) {
	completedHook := newHook(synccommon.HookTypePreSync)
	completedHook.SetName("completed-hook")
	completedHook.SetNamespace(FakeArgoCDNamespace)
	_ = Annotate(completedHook, synccommon.AnnotationKeyHookDeletePolicy, "HookSucceeded")

	inProgressHook := newHook(synccommon.HookTypePreSync)
	inProgressHook.SetNamespace(FakeArgoCDNamespace)
	inProgressHook.SetName("in-progress-hook")
	_ = Annotate(inProgressHook, synccommon.AnnotationKeyHookDeletePolicy, "HookSucceeded")

	syncCtx := newTestSyncCtx(nil,
		WithHealthOverride(resourceNameHealthOverride(map[string]health.HealthStatusCode{
			inProgressHook.GetName(): health.HealthStatusProgressing,
		})),
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(completedHook),
			HookPhase:   synccommon.OperationSucceeded,
			SyncPhase:   synccommon.SyncPhasePreSync,
		}, {
			ResourceKey: kube.GetResourceKey(inProgressHook),
			HookPhase:   synccommon.OperationRunning,
			SyncPhase:   synccommon.SyncPhasePreSync,
		}},
			metav1.Now(),
		))
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	syncCtx.dynamicIf = fakeDynamicClient
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount += 1
		return true, nil, nil
	})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{completedHook, inProgressHook},
		Target: []*unstructured.Unstructured{nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{completedHook, inProgressHook}

	syncCtx.kubectl = &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{},
	}

	syncCtx.Sync()

	assert.Equal(t, synccommon.OperationRunning, syncCtx.phase)
	assert.Equal(t, 0, deletedCount)
}

func TestRunSync_HooksDeletedAfterPhaseCompleted(t *testing.T) {
	completedHook1 := newHook(synccommon.HookTypePreSync)
	completedHook1.SetName("completed-hook1")
	completedHook1.SetNamespace(FakeArgoCDNamespace)
	_ = Annotate(completedHook1, synccommon.AnnotationKeyHookDeletePolicy, "HookSucceeded")

	completedHook2 := newHook(synccommon.HookTypePreSync)
	completedHook2.SetNamespace(FakeArgoCDNamespace)
	completedHook2.SetName("completed-hook2")
	_ = Annotate(completedHook2, synccommon.AnnotationKeyHookDeletePolicy, "HookSucceeded")

	syncCtx := newTestSyncCtx(nil,
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(completedHook1),
			HookPhase:   synccommon.OperationSucceeded,
			SyncPhase:   synccommon.SyncPhasePreSync,
		}, {
			ResourceKey: kube.GetResourceKey(completedHook2),
			HookPhase:   synccommon.OperationSucceeded,
			SyncPhase:   synccommon.SyncPhasePreSync,
		}},
			metav1.Now(),
		))
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	syncCtx.dynamicIf = fakeDynamicClient
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount += 1
		return true, nil, nil
	})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{completedHook1, completedHook2},
		Target: []*unstructured.Unstructured{nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{completedHook1, completedHook2}

	syncCtx.kubectl = &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{},
	}

	syncCtx.Sync()

	assert.Equal(t, synccommon.OperationSucceeded, syncCtx.phase)
	assert.Equal(t, 2, deletedCount)
}

func TestRunSync_HooksDeletedAfterPhaseCompletedFailed(t *testing.T) {
	completedHook1 := newHook(synccommon.HookTypeSync)
	completedHook1.SetName("completed-hook1")
	completedHook1.SetNamespace(FakeArgoCDNamespace)
	_ = Annotate(completedHook1, synccommon.AnnotationKeyHookDeletePolicy, "HookFailed")

	completedHook2 := newHook(synccommon.HookTypeSync)
	completedHook2.SetNamespace(FakeArgoCDNamespace)
	completedHook2.SetName("completed-hook2")
	_ = Annotate(completedHook2, synccommon.AnnotationKeyHookDeletePolicy, "HookFailed")

	syncCtx := newTestSyncCtx(nil,
		WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{{
			ResourceKey: kube.GetResourceKey(completedHook1),
			HookPhase:   synccommon.OperationSucceeded,
			SyncPhase:   synccommon.SyncPhaseSync,
		}, {
			ResourceKey: kube.GetResourceKey(completedHook2),
			HookPhase:   synccommon.OperationFailed,
			SyncPhase:   synccommon.SyncPhaseSync,
		}},
			metav1.Now(),
		))
	fakeDynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	syncCtx.dynamicIf = fakeDynamicClient
	deletedCount := 0
	fakeDynamicClient.PrependReactor("delete", "*", func(action testcore.Action) (handled bool, ret runtime.Object, err error) {
		deletedCount += 1
		return true, nil, nil
	})
	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{completedHook1, completedHook2},
		Target: []*unstructured.Unstructured{nil, nil},
	})
	syncCtx.hooks = []*unstructured.Unstructured{completedHook1, completedHook2}

	syncCtx.kubectl = &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{},
	}

	syncCtx.Sync()

	assert.Equal(t, synccommon.OperationFailed, syncCtx.phase)
	assert.Equal(t, 2, deletedCount)
}

func Test_syncContext_liveObj(t *testing.T) {
	type fields struct {
		compareResult ReconciliationResult
	}
	type args struct {
		obj *unstructured.Unstructured
	}
	obj := NewPod()
	obj.SetNamespace("my-ns")

	found := NewPod()
	foundNoNamespace := NewPod()
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
			if got := sc.liveObj(tt.args.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("syncContext.liveObj() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_syncContext_hasCRDOfGroupKind(t *testing.T) {
	// target
	assert.False(t, (&syncContext{resources: groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{NewCRD()},
	})}).hasCRDOfGroupKind("", ""))
	assert.True(t, (&syncContext{resources: groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{NewCRD()},
	})}).hasCRDOfGroupKind("argoproj.io", "TestCrd"))

	// hook
	assert.False(t, (&syncContext{hooks: []*unstructured.Unstructured{NewCRD()}}).hasCRDOfGroupKind("", ""))
	assert.True(t, (&syncContext{hooks: []*unstructured.Unstructured{NewCRD()}}).hasCRDOfGroupKind("argoproj.io", "TestCrd"))
}

func Test_setRunningPhase_healthyState(t *testing.T) {
	sc := syncContext{log: klogr.New().WithValues("application", "fake-app")}

	sc.setRunningPhase([]*syncTask{{targetObj: NewPod()}, {targetObj: NewPod()}, {targetObj: NewPod()}}, false)

	assert.Equal(t, "waiting for healthy state of /Pod/my-pod and 2 more resources", sc.message)
}

func Test_setRunningPhase_runningHooks(t *testing.T) {
	sc := syncContext{log: klogr.New().WithValues("application", "fake-app")}

	sc.setRunningPhase([]*syncTask{{targetObj: newHook(synccommon.HookTypeSyncFail)}}, false)

	assert.Equal(t, "waiting for completion of hook /Pod/my-pod", sc.message)
}

func Test_setRunningPhase_pendingDeletion(t *testing.T) {
	sc := syncContext{log: klogr.New().WithValues("application", "fake-app")}

	sc.setRunningPhase([]*syncTask{{targetObj: NewPod()}, {targetObj: NewPod()}, {targetObj: NewPod()}}, true)

	assert.Equal(t, "waiting for deletion of /Pod/my-pod and 2 more resources", sc.message)
}

func TestSyncWaveHook(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, false, false, false))
	pod1 := NewPod()
	pod1.SetName("pod-1")
	pod1.SetAnnotations(map[string]string{synccommon.AnnotationSyncWave: "-1"})
	pod2 := NewPod()
	pod2.SetName("pod-2")
	pod3 := NewPod()
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
	syncCtx.syncWaveHook = func(phase synccommon.SyncPhase, wave int, final bool) error {
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

func TestSyncWaveHookFail(t *testing.T) {
	syncCtx := newTestSyncCtx(nil, WithOperationSettings(false, false, false, false))
	pod1 := NewPod()
	pod1.SetName("pod-1")

	syncCtx.resources = groupResources(ReconciliationResult{
		Live:   []*unstructured.Unstructured{nil},
		Target: []*unstructured.Unstructured{pod1},
	})

	called := false
	syncCtx.syncWaveHook = func(phase synccommon.SyncPhase, wave int, final bool) error {
		called = true
		return errors.New("intentional error")
	}
	syncCtx.Sync()
	assert.True(t, called)
	phase, msg, results := syncCtx.GetState()
	assert.Equal(t, synccommon.OperationFailed, phase)
	assert.Equal(t, "SyncWaveHook failed: intentional error", msg)
	assert.Equal(t, synccommon.OperationRunning, results[0].HookPhase)
}

func TestPruneLast(t *testing.T) {
	syncCtx := newTestSyncCtx(nil)
	syncCtx.pruneLast = true

	pod1 := NewPod()
	pod1.SetName("pod-1")
	pod2 := NewPod()
	pod2.SetName("pod-2")
	pod3 := NewPod()
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
		// last wave is the last sync wave for non-prune task + 1
		assert.Equal(t, 3, tasks.lastWave())
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
		// last wave is the last sync wave for non-prune task + 1
		assert.Equal(t, 3, tasks.lastWave())
	})
}

func diffResultList() *diff.DiffResultList {
	pod1 := NewPod()
	pod1.SetName("pod-1")
	pod1.SetNamespace(FakeArgoCDNamespace)
	pod2 := NewPod()
	pod2.SetName("pod-2")
	pod2.SetNamespace(FakeArgoCDNamespace)
	pod3 := NewPod()
	pod3.SetName("pod-3")
	pod3.SetNamespace(FakeArgoCDNamespace)

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
	assert.Equal(t, v1.DeletePropagationForeground, *opts.PropagationPolicy)
}

func TestSyncContext_GetDeleteOptions_WithPrunePropagationPolicy(t *testing.T) {
	sc := syncContext{}

	policy := v1.DeletePropagationBackground
	WithPrunePropagationPolicy(&policy)(&sc)

	opts := sc.getDeleteOptions()
	assert.Equal(t, v1.DeletePropagationBackground, *opts.PropagationPolicy)
}

func TestSetOperationFailed(t *testing.T) {
	sc := syncContext{}
	sc.log = klogr.New().WithValues("application", "fake-app")

	tasks := make([]*syncTask, 0)
	tasks = append(tasks, &syncTask{message: "namespace not found"})

	sc.setOperationFailed(nil, tasks, "one or more objects failed to apply")

	assert.Equal(t, sc.message, "one or more objects failed to apply, reason: namespace not found")

}

func TestSetOperationFailedDuplicatedMessages(t *testing.T) {
	sc := syncContext{}
	sc.log = klogr.New().WithValues("application", "fake-app")

	tasks := make([]*syncTask, 0)
	tasks = append(tasks, &syncTask{message: "namespace not found"})
	tasks = append(tasks, &syncTask{message: "namespace not found"})

	sc.setOperationFailed(nil, tasks, "one or more objects failed to apply")

	assert.Equal(t, sc.message, "one or more objects failed to apply, reason: namespace not found")

}

func TestSetOperationFailedNoTasks(t *testing.T) {
	sc := syncContext{}
	sc.log = klogr.New().WithValues("application", "fake-app")

	sc.setOperationFailed(nil, nil, "one or more objects failed to apply")

	assert.Equal(t, sc.message, "one or more objects failed to apply")

}
