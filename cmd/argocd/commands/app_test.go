package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/yaml"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	accountpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/account"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	applicationsetpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	certificatepkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/certificate"
	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	gpgkeypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/gpgkey"
	notificationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
	projectpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	repocredspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repocreds"
	repositorypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"
	sessionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/session"
	settingspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/settings"
	versionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8swatch "k8s.io/apimachinery/pkg/watch"
)

func Test_getInfos(t *testing.T) {
	testCases := []struct {
		name          string
		infos         []string
		expectedInfos []*v1alpha1.Info
	}{
		{
			name:          "empty",
			infos:         []string{},
			expectedInfos: []*v1alpha1.Info{},
		},
		{
			name:  "simple key value",
			infos: []string{"key1=value1", "key2=value2"},
			expectedInfos: []*v1alpha1.Info{
				{Name: "key1", Value: "value1"},
				{Name: "key2", Value: "value2"},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			infos := getInfos(testCase.infos)
			assert.Len(t, infos, len(testCase.expectedInfos))
			sort := func(a, b *v1alpha1.Info) bool { return a.Name < b.Name }
			assert.Empty(t, cmp.Diff(testCase.expectedInfos, infos, cmpopts.SortSlices(sort)))
		})
	}
}

func Test_getRefreshType(t *testing.T) {
	refreshTypeNormal := string(v1alpha1.RefreshTypeNormal)
	refreshTypeHard := string(v1alpha1.RefreshTypeHard)
	testCases := []struct {
		refresh     bool
		hardRefresh bool
		expected    *string
	}{
		{false, false, nil},
		{false, true, &refreshTypeHard},
		{true, false, &refreshTypeNormal},
		{true, true, &refreshTypeHard},
	}

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("hardRefresh=%t refresh=%t", testCase.hardRefresh, testCase.refresh), func(t *testing.T) {
			refreshType := getRefreshType(testCase.refresh, testCase.hardRefresh)
			if testCase.expected == nil {
				assert.Nil(t, refreshType)
			} else {
				assert.NotNil(t, refreshType)
				assert.Equal(t, *testCase.expected, *refreshType)
			}
		})
	}
}

func TestFindRevisionHistoryWithoutPassedId(t *testing.T) {
	histories := v1alpha1.RevisionHistories{}

	histories = append(histories, v1alpha1.RevisionHistory{ID: 1})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 2})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 3})

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, -1)
	if err != nil {
		t.Fatal("Find revision history should fail without errors")
	}

	if history == nil {
		t.Fatal("History should be found")
	}
}

func TestPrintTreeViewAppGet(t *testing.T) {
	var nodes [3]v1alpha1.ResourceNode
	nodes[0].ResourceRef = v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5-6trpt", UID: "92c3a5fe-d13e-4ae2-b8ec-c10dd3543b28"}
	nodes[0].ParentRefs = []v1alpha1.ResourceRef{{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}}
	nodes[1].ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	nodes[1].ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}
	nodes[2].ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}

	nodeMapping := make(map[string]v1alpha1.ResourceNode)
	mapParentToChild := make(map[string][]string)
	parentNode := make(map[string]struct{})

	for _, node := range nodes {
		nodeMapping[node.UID] = node

		if len(node.ParentRefs) > 0 {
			_, ok := mapParentToChild[node.ParentRefs[0].UID]
			if !ok {
				var temp []string
				mapParentToChild[node.ParentRefs[0].UID] = temp
			}
			mapParentToChild[node.ParentRefs[0].UID] = append(mapParentToChild[node.ParentRefs[0].UID], node.UID)
		} else {
			parentNode[node.UID] = struct{}{}
		}
	}

	output, _ := captureOutput(func() error {
		printTreeView(nodeMapping, mapParentToChild, parentNode, nil)
		return nil
	})

	assert.Contains(t, output, "Pod")
	assert.Contains(t, output, "ReplicaSet")
	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "numalogic-rollout-demo-5dcd5457d5-6trpt")
}

func TestPrintTreeViewDetailedAppGet(t *testing.T) {
	var nodes [3]v1alpha1.ResourceNode
	nodes[0].ResourceRef = v1alpha1.ResourceRef{Group: "", Version: "v1", Kind: "Pod", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5-6trpt", UID: "92c3a5fe-d13e-4ae2-b8ec-c10dd3543b28"}
	nodes[0].Health = &v1alpha1.HealthStatus{Status: "Degraded", Message: "Readiness Gate failed"}
	nodes[0].ParentRefs = []v1alpha1.ResourceRef{{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}}
	nodes[1].ResourceRef = v1alpha1.ResourceRef{Group: "apps", Version: "v1", Kind: "ReplicaSet", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo-5dcd5457d5", UID: "75c30dce-1b66-414f-a86c-573a74be0f40"}
	nodes[1].ParentRefs = []v1alpha1.ResourceRef{{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}}
	nodes[2].ResourceRef = v1alpha1.ResourceRef{Group: "argoproj.io", Version: "", Kind: "Rollout", Namespace: "sandbox-rollout-numalogic-demo", Name: "numalogic-rollout-demo", UID: "87f3aab0-f634-4b2c-959a-7ddd30675ed0"}

	nodeMapping := make(map[string]v1alpha1.ResourceNode)
	mapParentToChild := make(map[string][]string)
	parentNode := make(map[string]struct{})

	for _, node := range nodes {
		nodeMapping[node.UID] = node

		if len(node.ParentRefs) > 0 {
			_, ok := mapParentToChild[node.ParentRefs[0].UID]
			if !ok {
				var temp []string
				mapParentToChild[node.ParentRefs[0].UID] = temp
			}
			mapParentToChild[node.ParentRefs[0].UID] = append(mapParentToChild[node.ParentRefs[0].UID], node.UID)
		} else {
			parentNode[node.UID] = struct{}{}
		}
	}

	output, _ := captureOutput(func() error {
		printTreeViewDetailed(nodeMapping, mapParentToChild, parentNode, nil)
		return nil
	})

	assert.Contains(t, output, "Pod")
	assert.Contains(t, output, "ReplicaSet")
	assert.Contains(t, output, "Rollout")
	assert.Contains(t, output, "numalogic-rollout-demo-5dcd5457d5-6trpt")
	assert.Contains(t, output, "Degraded")
	assert.Contains(t, output, "Readiness Gate failed")
}

func TestFindRevisionHistoryWithoutPassedIdWithMultipleSources(t *testing.T) {
	histories := v1alpha1.RevisionHistories{}

	histories = append(histories, v1alpha1.RevisionHistory{ID: 1})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 2})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 3})

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, -1)
	if err != nil {
		t.Fatal("Find revision history should fail without errors")
	}

	if history == nil {
		t.Fatal("History should be found")
	}
}

func TestDefaultWaitOptions(t *testing.T) {
	watch := watchOpts{
		sync:      false,
		health:    false,
		operation: false,
		suspended: false,
	}
	opts := getWatchOpts(watch)
	assert.True(t, opts.sync)
	assert.True(t, opts.health)
	assert.True(t, opts.operation)
	assert.False(t, opts.suspended)
}

func TestOverrideWaitOptions(t *testing.T) {
	watch := watchOpts{
		sync:      true,
		health:    false,
		operation: false,
		suspended: false,
	}
	opts := getWatchOpts(watch)
	assert.True(t, opts.sync)
	assert.False(t, opts.health)
	assert.False(t, opts.operation)
	assert.False(t, opts.suspended)
}

func TestFindRevisionHistoryWithoutPassedIdAndEmptyHistoryList(t *testing.T) {
	histories := v1alpha1.RevisionHistories{}

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, -1)

	if err == nil {
		t.Fatal("Find revision history should fail with errors")
	}

	if history != nil {
		t.Fatal("History should be empty")
	}

	if err.Error() != "Application '' should have at least two successful deployments" {
		t.Fatal("Find revision history should fail with correct error message")
	}
}

func TestFindRevisionHistoryWithPassedId(t *testing.T) {
	histories := v1alpha1.RevisionHistories{}

	histories = append(histories, v1alpha1.RevisionHistory{ID: 1})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 2})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 3, Revision: "123"})

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, 3)
	if err != nil {
		t.Fatal("Find revision history should fail without errors")
	}

	if history == nil {
		t.Fatal("History should be found")
	}

	if history.Revision != "123" {
		t.Fatal("Failed to find correct history with correct revision")
	}
}

func TestFindRevisionHistoryWithPassedIdThatNotExist(t *testing.T) {
	histories := v1alpha1.RevisionHistories{}

	histories = append(histories, v1alpha1.RevisionHistory{ID: 1})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 2})
	histories = append(histories, v1alpha1.RevisionHistory{ID: 3, Revision: "123"})

	status := v1alpha1.ApplicationStatus{
		Resources:      nil,
		Sync:           v1alpha1.SyncStatus{},
		Health:         v1alpha1.HealthStatus{},
		History:        histories,
		Conditions:     nil,
		ReconciledAt:   nil,
		OperationState: nil,
		ObservedAt:     nil,
		SourceType:     "",
		Summary:        v1alpha1.ApplicationSummary{},
	}

	application := v1alpha1.Application{
		Status: status,
	}

	history, err := findRevisionHistory(&application, 4)

	if err == nil {
		t.Fatal("Find revision history should fail with errors")
	}

	if history != nil {
		t.Fatal("History should be not found")
	}

	if err.Error() != "Application '' does not have deployment id '4' in history\n" {
		t.Fatal("Find revision history should fail with correct error message")
	}
}

func Test_groupObjsByKey(t *testing.T) {
	localObjs := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      "pod-name",
					"namespace": "default",
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]interface{}{
					"name": "certificates.cert-manager.io",
				},
			},
		},
	}
	liveObjs := []*unstructured.Unstructured{
		{
			Object: map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Pod",
				"metadata": map[string]interface{}{
					"name":      "pod-name",
					"namespace": "default",
				},
			},
		},
		{
			Object: map[string]interface{}{
				"apiVersion": "apiextensions.k8s.io/v1",
				"kind":       "CustomResourceDefinition",
				"metadata": map[string]interface{}{
					"name": "certificates.cert-manager.io",
				},
			},
		},
	}

	expected := map[kube.ResourceKey]*unstructured.Unstructured{
		{Group: "", Kind: "Pod", Namespace: "default", Name: "pod-name"}:                                                       localObjs[0],
		{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Namespace: "", Name: "certificates.cert-manager.io"}: localObjs[1],
	}

	objByKey := groupObjsByKey(localObjs, liveObjs, "default")
	assert.Equal(t, expected, objByKey)
}

func TestFormatSyncPolicy(t *testing.T) {
	t.Run("Policy not defined", func(t *testing.T) {
		app := v1alpha1.Application{}

		policy := formatSyncPolicy(app)

		if policy != "Manual" {
			t.Fatalf("Incorrect policy %q, should be Manual", policy)
		}
	})

	t.Run("Auto policy", func(t *testing.T) {
		app := v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				SyncPolicy: &v1alpha1.SyncPolicy{
					Automated: &v1alpha1.SyncPolicyAutomated{},
				},
			},
		}

		policy := formatSyncPolicy(app)

		if policy != "Auto" {
			t.Fatalf("Incorrect policy %q, should be Auto", policy)
		}
	})

	t.Run("Auto policy with prune", func(t *testing.T) {
		app := v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				SyncPolicy: &v1alpha1.SyncPolicy{
					Automated: &v1alpha1.SyncPolicyAutomated{
						Prune: true,
					},
				},
			},
		}

		policy := formatSyncPolicy(app)

		if policy != "Auto-Prune" {
			t.Fatalf("Incorrect policy %q, should be Auto-Prune", policy)
		}
	})
}

func TestFormatConditionSummary(t *testing.T) {
	t.Run("No conditions are defined", func(t *testing.T) {
		app := v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				SyncPolicy: &v1alpha1.SyncPolicy{
					Automated: &v1alpha1.SyncPolicyAutomated{
						Prune: true,
					},
				},
			},
		}

		summary := formatConditionsSummary(app)
		if summary != "<none>" {
			t.Fatalf("Incorrect summary %q, should be <none>", summary)
		}
	})

	t.Run("Few conditions are defined", func(t *testing.T) {
		app := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Conditions: []v1alpha1.ApplicationCondition{
					{
						Type: "type1",
					},
					{
						Type: "type1",
					},
					{
						Type: "type2",
					},
				},
			},
		}

		summary := formatConditionsSummary(app)
		if summary != "type1(2),type2" && summary != "type2,type1(2)" {
			t.Fatalf("Incorrect summary %q, should be type1(2),type2", summary)
		}
	})
}

func TestPrintOperationResult(t *testing.T) {
	t.Run("Operation state is empty", func(t *testing.T) {
		output, _ := captureOutput(func() error {
			printOperationResult(nil)
			return nil
		})

		if output != "" {
			t.Fatalf("Incorrect print operation output %q, should be ''", output)
		}
	})

	t.Run("Operation state sync result is not empty", func(t *testing.T) {
		time := metav1.Date(2020, time.November, 10, 23, 0, 0, 0, time.UTC)
		output, _ := captureOutput(func() error {
			printOperationResult(&v1alpha1.OperationState{
				SyncResult: &v1alpha1.SyncOperationResult{Revision: "revision"},
				FinishedAt: &time,
			})
			return nil
		})

		expectation := "Operation:          Sync\nSync Revision:      revision\nPhase:              \nStart:              0001-01-01 00:00:00 +0000 UTC\nFinished:           2020-11-10 23:00:00 +0000 UTC\nDuration:           2333448h16m18.871345152s\n"
		if output != expectation {
			t.Fatalf("Incorrect print operation output %q, should be %q", output, expectation)
		}
	})

	t.Run("Operation state sync result with message is not empty", func(t *testing.T) {
		time := metav1.Date(2020, time.November, 10, 23, 0, 0, 0, time.UTC)
		output, _ := captureOutput(func() error {
			printOperationResult(&v1alpha1.OperationState{
				SyncResult: &v1alpha1.SyncOperationResult{Revision: "revision"},
				FinishedAt: &time,
				Message:    "test",
			})
			return nil
		})

		expectation := "Operation:          Sync\nSync Revision:      revision\nPhase:              \nStart:              0001-01-01 00:00:00 +0000 UTC\nFinished:           2020-11-10 23:00:00 +0000 UTC\nDuration:           2333448h16m18.871345152s\nMessage:            test\n"
		if output != expectation {
			t.Fatalf("Incorrect print operation output %q, should be %q", output, expectation)
		}
	})
}

func TestPrintApplicationHistoryTable(t *testing.T) {
	histories := []v1alpha1.RevisionHistory{
		{
			ID: 1,
			Source: v1alpha1.ApplicationSource{
				TargetRevision: "1",
				RepoURL:        "test",
			},
		},
		{
			ID: 2,
			Source: v1alpha1.ApplicationSource{
				TargetRevision: "2",
				RepoURL:        "test",
			},
		},
		{
			ID: 3,
			Source: v1alpha1.ApplicationSource{
				TargetRevision: "3",
				RepoURL:        "test",
			},
		},
	}

	output, _ := captureOutput(func() error {
		printApplicationHistoryTable(histories)
		return nil
	})

	expectation := "SOURCE  test\nID      DATE                           REVISION\n1       0001-01-01 00:00:00 +0000 UTC  1\n2       0001-01-01 00:00:00 +0000 UTC  2\n3       0001-01-01 00:00:00 +0000 UTC  3\n"

	if output != expectation {
		t.Fatalf("Incorrect print operation output %q, should be %q", output, expectation)
	}
}

func TestPrintApplicationHistoryTableWithMultipleSources(t *testing.T) {
	histories := []v1alpha1.RevisionHistory{
		{
			ID: 0,
			Source: v1alpha1.ApplicationSource{
				TargetRevision: "0",
				RepoURL:        "test",
			},
		},
		{
			ID: 1,
			Revisions: []string{
				"1a",
				"1b",
			},
			// added Source just for testing the fuction
			Source: v1alpha1.ApplicationSource{
				TargetRevision: "-1",
				RepoURL:        "ignore",
			},
			Sources: v1alpha1.ApplicationSources{
				v1alpha1.ApplicationSource{
					RepoURL:        "test-1",
					TargetRevision: "1a",
				},
				v1alpha1.ApplicationSource{
					RepoURL:        "test-2",
					TargetRevision: "1b",
				},
			},
		},
		{
			ID: 2,
			Revisions: []string{
				"2a",
				"2b",
			},
			Sources: v1alpha1.ApplicationSources{
				v1alpha1.ApplicationSource{
					RepoURL:        "test-1",
					TargetRevision: "2a",
				},
				v1alpha1.ApplicationSource{
					RepoURL:        "test-2",
					TargetRevision: "2b",
				},
			},
		},
		{
			ID: 3,
			Revisions: []string{
				"3a",
				"3b",
			},
			Sources: v1alpha1.ApplicationSources{
				v1alpha1.ApplicationSource{
					RepoURL:        "test-1",
					TargetRevision: "3a",
				},
				v1alpha1.ApplicationSource{
					RepoURL:        "test-2",
					TargetRevision: "3b",
				},
			},
		},
	}

	output, _ := captureOutput(func() error {
		printApplicationHistoryTable(histories)
		return nil
	})

	expectation := "SOURCE  test\nID      DATE                           REVISION\n0       0001-01-01 00:00:00 +0000 UTC  0\n\nSOURCE  test-1\nID      DATE                           REVISION\n1       0001-01-01 00:00:00 +0000 UTC  1a\n2       0001-01-01 00:00:00 +0000 UTC  2a\n3       0001-01-01 00:00:00 +0000 UTC  3a\n\nSOURCE  test-2\nID      DATE                           REVISION\n1       0001-01-01 00:00:00 +0000 UTC  1b\n2       0001-01-01 00:00:00 +0000 UTC  2b\n3       0001-01-01 00:00:00 +0000 UTC  3b\n"

	if output != expectation {
		t.Fatalf("Incorrect print operation output %q, should be %q", output, expectation)
	}
}

func TestPrintAppSummaryTable(t *testing.T) {
	output, _ := captureOutput(func() error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				SyncPolicy: &v1alpha1.SyncPolicy{
					Automated: &v1alpha1.SyncPolicyAutomated{
						Prune: true,
					},
				},
				Project:     "default",
				Destination: v1alpha1.ApplicationDestination{Server: "local", Namespace: "argocd"},
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "test",
					TargetRevision: "master",
					Path:           "/test",
					Helm: &v1alpha1.ApplicationSourceHelm{
						ValueFiles: []string{"path1", "path2"},
					},
					Kustomize: &v1alpha1.ApplicationSourceKustomize{NamePrefix: "prefix"},
				},
			},
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Status: v1alpha1.SyncStatusCodeOutOfSync,
				},
				Health: v1alpha1.HealthStatus{
					Status:  health.HealthStatusProgressing,
					Message: "health-message",
				},
			},
		}

		windows := &v1alpha1.SyncWindows{
			{
				Kind:     "allow",
				Schedule: "0 0 * * *",
				Duration: "24h",
				Applications: []string{
					"*-prod",
				},
				ManualSync: true,
			},
			{
				Kind:     "deny",
				Schedule: "0 0 * * *",
				Duration: "24h",
				Namespaces: []string{
					"default",
				},
			},
			{
				Kind:     "allow",
				Schedule: "0 0 * * *",
				Duration: "24h",
				Clusters: []string{
					"in-cluster",
					"cluster1",
				},
			},
		}

		printAppSummaryTable(app, "url", windows)
		return nil
	})

	expectation := `Name:               argocd/test
Project:            default
Server:             local
Namespace:          argocd
URL:                url
Source:
- Repo:             test
  Target:           master
  Path:             /test
  Helm Values:      path1,path2
  Name Prefix:      prefix
SyncWindow:         Sync Denied
Assigned Windows:   allow:0 0 * * *:24h,deny:0 0 * * *:24h,allow:0 0 * * *:24h
Sync Policy:        Automated (Prune)
Sync Status:        OutOfSync from master
Health Status:      Progressing (health-message)
`
	assert.Equalf(t, expectation, output, "Incorrect print app summary output %q, should be %q", output, expectation)
}

func TestPrintAppSummaryTable_MultipleSources(t *testing.T) {
	output, _ := captureOutput(func() error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "argocd",
			},
			Spec: v1alpha1.ApplicationSpec{
				SyncPolicy: &v1alpha1.SyncPolicy{
					Automated: &v1alpha1.SyncPolicyAutomated{
						Prune: true,
					},
				},
				Project:     "default",
				Destination: v1alpha1.ApplicationDestination{Server: "local", Namespace: "argocd"},
				Sources: v1alpha1.ApplicationSources{
					{
						RepoURL:        "test",
						TargetRevision: "master",
						Path:           "/test",
						Helm: &v1alpha1.ApplicationSourceHelm{
							ValueFiles: []string{"path1", "path2"},
						},
						Kustomize: &v1alpha1.ApplicationSourceKustomize{NamePrefix: "prefix"},
					}, {
						RepoURL:        "test2",
						TargetRevision: "master2",
						Path:           "/test2",
					},
				},
			},
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Status: v1alpha1.SyncStatusCodeOutOfSync,
				},
				Health: v1alpha1.HealthStatus{
					Status:  health.HealthStatusProgressing,
					Message: "health-message",
				},
			},
		}

		windows := &v1alpha1.SyncWindows{
			{
				Kind:     "allow",
				Schedule: "0 0 * * *",
				Duration: "24h",
				Applications: []string{
					"*-prod",
				},
				ManualSync: true,
			},
			{
				Kind:     "deny",
				Schedule: "0 0 * * *",
				Duration: "24h",
				Namespaces: []string{
					"default",
				},
			},
			{
				Kind:     "allow",
				Schedule: "0 0 * * *",
				Duration: "24h",
				Clusters: []string{
					"in-cluster",
					"cluster1",
				},
			},
		}

		printAppSummaryTable(app, "url", windows)
		return nil
	})

	expectation := `Name:               argocd/test
Project:            default
Server:             local
Namespace:          argocd
URL:                url
Sources:
- Repo:             test
  Target:           master
  Path:             /test
  Helm Values:      path1,path2
  Name Prefix:      prefix
- Repo:             test2
  Target:           master2
  Path:             /test2
SyncWindow:         Sync Denied
Assigned Windows:   allow:0 0 * * *:24h,deny:0 0 * * *:24h,allow:0 0 * * *:24h
Sync Policy:        Automated (Prune)
Sync Status:        OutOfSync from master
Health Status:      Progressing (health-message)
`
	assert.Equalf(t, expectation, output, "Incorrect print app summary output %q, should be %q", output, expectation)
}

func TestPrintAppConditions(t *testing.T) {
	output, _ := captureOutput(func() error {
		app := &v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Conditions: []v1alpha1.ApplicationCondition{
					{
						Type:    v1alpha1.ApplicationConditionDeletionError,
						Message: "test",
					},
					{
						Type:    v1alpha1.ApplicationConditionExcludedResourceWarning,
						Message: "test2",
					},
					{
						Type:    v1alpha1.ApplicationConditionRepeatedResourceWarning,
						Message: "test3",
					},
				},
			},
		}
		printAppConditions(os.Stdout, app)
		return nil
	})
	expectation := "CONDITION\tMESSAGE\tLAST TRANSITION\nDeletionError\ttest\t<nil>\nExcludedResourceWarning\ttest2\t<nil>\nRepeatedResourceWarning\ttest3\t<nil>\n"
	if output != expectation {
		t.Fatalf("Incorrect print app conditions output %q, should be %q", output, expectation)
	}
}

func TestPrintParams(t *testing.T) {
	testCases := []struct {
		name           string
		app            *v1alpha1.Application
		sourcePosition int
		expectedOutput string
	}{
		{
			name: "Single Source application with valid helm parameters",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Source: &v1alpha1.ApplicationSource{
						Helm: &v1alpha1.ApplicationSourceHelm{
							Parameters: []v1alpha1.HelmParameter{
								{
									Name:  "name1",
									Value: "value1",
								},
								{
									Name:  "name2",
									Value: "value2",
								},
								{
									Name:  "name3",
									Value: "value3",
								},
							},
						},
					},
				},
			},
			sourcePosition: -1,
			expectedOutput: "\n\nNAME   VALUE\nname1  value1\nname2  value2\nname3  value3\n",
		},
		{
			name: "Multi-source application with a valid Source Position",
			app: &v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{
					Sources: []v1alpha1.ApplicationSource{
						{
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{
										Name:  "nameA",
										Value: "valueA",
									},
								},
							},
						},
						{
							Helm: &v1alpha1.ApplicationSourceHelm{
								Parameters: []v1alpha1.HelmParameter{
									{
										Name:  "nameB",
										Value: "valueB",
									},
								},
							},
						},
					},
				},
			},
			sourcePosition: 1,
			expectedOutput: "\n\nNAME   VALUE\nnameA  valueA\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, _ := captureOutput(func() error {
				printParams(tc.app, tc.sourcePosition)
				return nil
			})

			if output != tc.expectedOutput {
				t.Fatalf("Incorrect print params output %q, should be %q\n", output, tc.expectedOutput)
			}
		})
	}
}

func TestAppUrlDefault(t *testing.T) {
	t.Run("Plain text", func(t *testing.T) {
		result := appURLDefault(argocdclient.NewClientOrDie(&argocdclient.ClientOptions{
			ServerAddr: "localhost:80",
			PlainText:  true,
		}), "test")
		expectation := "http://localhost:80/applications/test"
		if result != expectation {
			t.Fatalf("Incorrect url %q, should be %q", result, expectation)
		}
	})
	t.Run("https", func(t *testing.T) {
		result := appURLDefault(argocdclient.NewClientOrDie(&argocdclient.ClientOptions{
			ServerAddr: "localhost:443",
			PlainText:  false,
		}), "test")
		expectation := "https://localhost/applications/test"
		if result != expectation {
			t.Fatalf("Incorrect url %q, should be %q", result, expectation)
		}
	})
}

func TestTruncateString(t *testing.T) {
	result := truncateString("argocdtool", 2)
	expectation := "ar..."
	if result != expectation {
		t.Fatalf("Incorrect truncate string %q, should be %q", result, expectation)
	}
}

func TestGetService(t *testing.T) {
	t.Run("Server", func(t *testing.T) {
		app := &v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				Destination: v1alpha1.ApplicationDestination{
					Server: "test-server",
				},
			},
		}
		result := getServer(app)
		expectation := "test-server"
		if result != expectation {
			t.Fatalf("Incorrect server %q, should be %q", result, expectation)
		}
	})
	t.Run("Name", func(t *testing.T) {
		app := &v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				Destination: v1alpha1.ApplicationDestination{
					Name: "test-name",
				},
			},
		}
		result := getServer(app)
		expectation := "test-name"
		if result != expectation {
			t.Fatalf("Incorrect server name %q, should be %q", result, expectation)
		}
	})
}

func TestTargetObjects(t *testing.T) {
	resources := []*v1alpha1.ResourceDiff{
		{
			TargetState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"argocd\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
		},
		{
			TargetState: "{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"name\":\"test-helm-guestbook\",\"namespace\":\"ns\"},\"spec\":{\"selector\":{\"app\":\"helm-guestbook\",\"release\":\"test\"},\"sessionAffinity\":\"None\",\"type\":\"ClusterIP\"},\"status\":{\"loadBalancer\":{}}}",
		},
	}
	objects, err := targetObjects(resources)
	if err != nil {
		t.Fatal("operation should finish without error")
	}

	if len(objects) != 2 {
		t.Fatalf("incorrect number of objects %v, should be 2", len(objects))
	}

	if objects[0].GetName() != "test-helm-guestbook" {
		t.Fatalf("incorrect name %q, should be %q", objects[0].GetName(), "test-helm-guestbook")
	}
}

func TestTargetObjects_invalid(t *testing.T) {
	resources := []*v1alpha1.ResourceDiff{{TargetState: "{"}}
	_, err := targetObjects(resources)
	assert.Error(t, err)
}

func TestCheckForDeleteEvent(t *testing.T) {
	ctx := context.Background()
	fakeClient := new(fakeAcdClient)

	checkForDeleteEvent(ctx, fakeClient, "testApp")
}

func TestPrintApplicationNames(t *testing.T) {
	output, _ := captureOutput(func() error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		}
		printApplicationNames([]v1alpha1.Application{*app, *app})
		return nil
	})
	expectation := "test\ntest\n"
	if output != expectation {
		t.Fatalf("Incorrect print params output %q, should be %q", output, expectation)
	}
}

func Test_unset(t *testing.T) {
	kustomizeSource := &v1alpha1.ApplicationSource{
		Kustomize: &v1alpha1.ApplicationSourceKustomize{
			NamePrefix: "some-prefix",
			NameSuffix: "some-suffix",
			Version:    "123",
			Images: v1alpha1.KustomizeImages{
				"old1=new:tag",
				"old2=new:tag",
			},
			Replicas: []v1alpha1.KustomizeReplica{
				{
					Name:  "my-deployment",
					Count: intstr.FromInt(2),
				},
				{
					Name:  "my-statefulset",
					Count: intstr.FromInt(4),
				},
			},
		},
	}

	helmSource := &v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			IgnoreMissingValueFiles: true,
			Parameters: []v1alpha1.HelmParameter{
				{
					Name:  "name-1",
					Value: "value-1",
				},
				{
					Name:  "name-2",
					Value: "value-2",
				},
			},
			PassCredentials: true,
			ValuesObject:    &runtime.RawExtension{Raw: []byte("some: yaml")},
			ValueFiles: []string{
				"values-1.yaml",
				"values-2.yaml",
			},
		},
	}

	pluginSource := &v1alpha1.ApplicationSource{
		Plugin: &v1alpha1.ApplicationSourcePlugin{
			Env: v1alpha1.Env{
				{
					Name:  "env-1",
					Value: "env-value-1",
				},
				{
					Name:  "env-2",
					Value: "env-value-2",
				},
			},
		},
	}

	assert.Equal(t, "some-prefix", kustomizeSource.Kustomize.NamePrefix)
	updated, nothingToUnset := unset(kustomizeSource, unsetOpts{namePrefix: true})
	assert.Equal(t, "", kustomizeSource.Kustomize.NamePrefix)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{namePrefix: true})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Equal(t, "some-suffix", kustomizeSource.Kustomize.NameSuffix)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{nameSuffix: true})
	assert.Equal(t, "", kustomizeSource.Kustomize.NameSuffix)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{nameSuffix: true})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Equal(t, "123", kustomizeSource.Kustomize.Version)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{kustomizeVersion: true})
	assert.Equal(t, "", kustomizeSource.Kustomize.Version)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{kustomizeVersion: true})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Len(t, kustomizeSource.Kustomize.Images, 2)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{kustomizeImages: []string{"old1=new:tag"}})
	assert.Len(t, kustomizeSource.Kustomize.Images, 1)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{kustomizeImages: []string{"old1=new:tag"}})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Len(t, kustomizeSource.Kustomize.Replicas, 2)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{kustomizeReplicas: []string{"my-deployment"}})
	assert.Len(t, kustomizeSource.Kustomize.Replicas, 1)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(kustomizeSource, unsetOpts{kustomizeReplicas: []string{"my-deployment"}})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Len(t, helmSource.Helm.Parameters, 2)
	updated, nothingToUnset = unset(helmSource, unsetOpts{parameters: []string{"name-1"}})
	assert.Len(t, helmSource.Helm.Parameters, 1)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(helmSource, unsetOpts{parameters: []string{"name-1"}})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Len(t, helmSource.Helm.ValueFiles, 2)
	updated, nothingToUnset = unset(helmSource, unsetOpts{valuesFiles: []string{"values-1.yaml"}})
	assert.Len(t, helmSource.Helm.ValueFiles, 1)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(helmSource, unsetOpts{valuesFiles: []string{"values-1.yaml"}})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Equal(t, "some: yaml", helmSource.Helm.ValuesString())
	updated, nothingToUnset = unset(helmSource, unsetOpts{valuesLiteral: true})
	assert.Equal(t, "", helmSource.Helm.ValuesString())
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(helmSource, unsetOpts{valuesLiteral: true})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.True(t, helmSource.Helm.IgnoreMissingValueFiles)
	updated, nothingToUnset = unset(helmSource, unsetOpts{ignoreMissingValueFiles: true})
	assert.False(t, helmSource.Helm.IgnoreMissingValueFiles)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(helmSource, unsetOpts{ignoreMissingValueFiles: true})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.True(t, helmSource.Helm.PassCredentials)
	updated, nothingToUnset = unset(helmSource, unsetOpts{passCredentials: true})
	assert.False(t, helmSource.Helm.PassCredentials)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(helmSource, unsetOpts{passCredentials: true})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)

	assert.Len(t, pluginSource.Plugin.Env, 2)
	updated, nothingToUnset = unset(pluginSource, unsetOpts{pluginEnvs: []string{"env-1"}})
	assert.Len(t, pluginSource.Plugin.Env, 1)
	assert.True(t, updated)
	assert.False(t, nothingToUnset)
	updated, nothingToUnset = unset(pluginSource, unsetOpts{pluginEnvs: []string{"env-1"}})
	assert.False(t, updated)
	assert.False(t, nothingToUnset)
}

func Test_unset_nothingToUnset(t *testing.T) {
	testCases := []struct {
		name   string
		source v1alpha1.ApplicationSource
	}{
		{"kustomize", v1alpha1.ApplicationSource{Kustomize: &v1alpha1.ApplicationSourceKustomize{}}},
		{"helm", v1alpha1.ApplicationSource{Helm: &v1alpha1.ApplicationSourceHelm{}}},
		{"plugin", v1alpha1.ApplicationSource{Plugin: &v1alpha1.ApplicationSourcePlugin{}}},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			updated, nothingToUnset := unset(&testCaseCopy.source, unsetOpts{})
			assert.False(t, updated)
			assert.True(t, nothingToUnset)
		})
	}
}

func TestFilterAppResources(t *testing.T) {
	// App resources
	var (
		appReplicaSet1 = v1alpha1.ResourceStatus{
			Group:     "apps",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "replicaSet-name1",
		}
		appReplicaSet2 = v1alpha1.ResourceStatus{
			Group:     "apps",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "replicaSet-name2",
		}
		appJob = v1alpha1.ResourceStatus{
			Group:     "batch",
			Kind:      "Job",
			Namespace: "default",
			Name:      "job-name",
		}
		appService1 = v1alpha1.ResourceStatus{
			Group:     "",
			Kind:      "Service",
			Namespace: "default",
			Name:      "service-name1",
		}
		appService2 = v1alpha1.ResourceStatus{
			Group:     "",
			Kind:      "Service",
			Namespace: "default",
			Name:      "service-name2",
		}
		appDeployment = v1alpha1.ResourceStatus{
			Group:     "apps",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "deployment-name",
		}
	)
	app := v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			Resources: []v1alpha1.ResourceStatus{
				appReplicaSet1, appReplicaSet2, appJob, appService1, appService2, appDeployment,
			},
		},
	}
	// Resource filters
	var (
		blankValues = v1alpha1.SyncOperationResource{
			Group:     "",
			Kind:      "",
			Name:      "",
			Namespace: "",
			Exclude:   false,
		}
		// *:*:*
		includeAllResources = v1alpha1.SyncOperationResource{
			Group:     "*",
			Kind:      "*",
			Name:      "*",
			Namespace: "",
			Exclude:   false,
		}
		// !*:*:*
		excludeAllResources = v1alpha1.SyncOperationResource{
			Group:     "*",
			Kind:      "*",
			Name:      "*",
			Namespace: "",
			Exclude:   true,
		}
		// *:Service:*
		includeAllServiceResources = v1alpha1.SyncOperationResource{
			Group:     "*",
			Kind:      "Service",
			Name:      "*",
			Namespace: "",
			Exclude:   false,
		}
		// !*:Service:*
		excludeAllServiceResources = v1alpha1.SyncOperationResource{
			Group:     "*",
			Kind:      "Service",
			Name:      "*",
			Namespace: "",
			Exclude:   true,
		}
		// apps:ReplicaSet:*
		includeAllReplicaSetResource = v1alpha1.SyncOperationResource{
			Group:     "apps",
			Kind:      "ReplicaSet",
			Name:      "*",
			Namespace: "",
			Exclude:   false,
		}
		// apps:ReplicaSet:replicaSet-name1
		includeReplicaSet1Resource = v1alpha1.SyncOperationResource{
			Group:     "apps",
			Kind:      "ReplicaSet",
			Name:      "replicaSet-name1",
			Namespace: "",
			Exclude:   false,
		}
		// !apps:ReplicaSet:replicaSet-name2
		excludeReplicaSet2Resource = v1alpha1.SyncOperationResource{
			Group:     "apps",
			Kind:      "ReplicaSet",
			Name:      "replicaSet-name2",
			Namespace: "",
			Exclude:   true,
		}
	)

	// Filtered resources
	var (
		replicaSet1 = v1alpha1.SyncOperationResource{
			Group:     "apps",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "replicaSet-name1",
		}
		replicaSet2 = v1alpha1.SyncOperationResource{
			Group:     "apps",
			Kind:      "ReplicaSet",
			Namespace: "default",
			Name:      "replicaSet-name2",
		}
		job = v1alpha1.SyncOperationResource{
			Group:     "batch",
			Kind:      "Job",
			Namespace: "default",
			Name:      "job-name",
		}
		service1 = v1alpha1.SyncOperationResource{
			Group:     "",
			Kind:      "Service",
			Namespace: "default",
			Name:      "service-name1",
		}
		service2 = v1alpha1.SyncOperationResource{
			Group:     "",
			Kind:      "Service",
			Namespace: "default",
			Name:      "service-name2",
		}
		deployment = v1alpha1.SyncOperationResource{
			Group:     "apps",
			Kind:      "Deployment",
			Namespace: "default",
			Name:      "deployment-name",
		}
	)
	tests := []struct {
		testName          string
		selectedResources []*v1alpha1.SyncOperationResource
		expectedResult    []*v1alpha1.SyncOperationResource
	}{
		// --resource apps:ReplicaSet:replicaSet-name1 --resource *:Service:*
		{
			testName:          "Include ReplicaSet replicaSet-name1 resource and all service resources",
			selectedResources: []*v1alpha1.SyncOperationResource{&includeAllServiceResources, &includeReplicaSet1Resource},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1, &service1, &service2},
		},
		// --resource apps:ReplicaSet:replicaSet-name1 --resource !*:Service:*
		{
			testName:          "Include ReplicaSet replicaSet-name1 resource and exclude all service resources",
			selectedResources: []*v1alpha1.SyncOperationResource{&excludeAllServiceResources, &includeReplicaSet1Resource},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1},
		},
		// --resource !apps:ReplicaSet:replicaSet-name2 --resource !*:Service:*
		{
			testName:          "Exclude ReplicaSet replicaSet-name2 resource and all service resources",
			selectedResources: []*v1alpha1.SyncOperationResource{&excludeReplicaSet2Resource, &excludeAllServiceResources},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1, &job, &deployment},
		},
		// --resource !apps:ReplicaSet:replicaSet-name2
		{
			testName:          "Exclude ReplicaSet replicaSet-name2 resource",
			selectedResources: []*v1alpha1.SyncOperationResource{&excludeReplicaSet2Resource},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1, &job, &service1, &service2, &deployment},
		},
		// --resource apps:ReplicaSet:replicaSet-name1
		{
			testName:          "Include ReplicaSet replicaSet-name1 resource",
			selectedResources: []*v1alpha1.SyncOperationResource{&includeReplicaSet1Resource},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1},
		},
		// --resource apps:ReplicaSet:* --resource !apps:ReplicaSet:replicaSet-name2
		{
			testName:          "Include All ReplicaSet resource and exclude replicaSet-name1 resource",
			selectedResources: []*v1alpha1.SyncOperationResource{&includeAllReplicaSetResource, &excludeReplicaSet2Resource},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1},
		},
		// --resource !*:Service:*
		{
			testName:          "Exclude Service resources",
			selectedResources: []*v1alpha1.SyncOperationResource{&excludeAllServiceResources},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1, &replicaSet2, &job, &deployment},
		},
		// --resource *:Service:*
		{
			testName:          "Include Service resources",
			selectedResources: []*v1alpha1.SyncOperationResource{&includeAllServiceResources},
			expectedResult:    []*v1alpha1.SyncOperationResource{&service1, &service2},
		},
		// --resource !*:*:*
		{
			testName:          "Exclude all resources",
			selectedResources: []*v1alpha1.SyncOperationResource{&excludeAllResources},
			expectedResult:    nil,
		},
		// --resource *:*:*
		{
			testName:          "Include all resources",
			selectedResources: []*v1alpha1.SyncOperationResource{&includeAllResources},
			expectedResult:    []*v1alpha1.SyncOperationResource{&replicaSet1, &replicaSet2, &job, &service1, &service2, &deployment},
		},
		{
			testName:          "No Filters",
			selectedResources: []*v1alpha1.SyncOperationResource{&blankValues},
			expectedResult:    nil,
		},
		{
			testName:          "Empty Filter",
			selectedResources: []*v1alpha1.SyncOperationResource{},
			expectedResult:    nil,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			filteredResources := filterAppResources(&app, test.selectedResources)
			assert.Equal(t, test.expectedResult, filteredResources)
		})
	}
}

func TestParseSelectedResources(t *testing.T) {
	resources := []string{
		"v1alpha:Application:test",
		"v1alpha:Application:namespace/test",
		"!v1alpha:Application:test",
		"apps:Deployment:default/test",
		"!*:*:*",
	}
	operationResources, err := parseSelectedResources(resources)
	require.NoError(t, err)
	assert.Len(t, operationResources, 5)
	assert.Equal(t, v1alpha1.SyncOperationResource{
		Namespace: "",
		Name:      "test",
		Kind:      application.ApplicationKind,
		Group:     "v1alpha",
	}, *operationResources[0])
	assert.Equal(t, v1alpha1.SyncOperationResource{
		Namespace: "namespace",
		Name:      "test",
		Kind:      application.ApplicationKind,
		Group:     "v1alpha",
	}, *operationResources[1])
	assert.Equal(t, v1alpha1.SyncOperationResource{
		Namespace: "",
		Name:      "test",
		Kind:      "Application",
		Group:     "v1alpha",
		Exclude:   true,
	}, *operationResources[2])
	assert.Equal(t, v1alpha1.SyncOperationResource{
		Namespace: "default",
		Name:      "test",
		Kind:      "Deployment",
		Group:     "apps",
		Exclude:   false,
	}, *operationResources[3])
	assert.Equal(t, v1alpha1.SyncOperationResource{
		Namespace: "",
		Name:      "*",
		Kind:      "*",
		Group:     "*",
		Exclude:   true,
	}, *operationResources[4])
}

func TestParseSelectedResourcesIncorrect(t *testing.T) {
	resources := []string{"v1alpha:test", "v1alpha:Application:namespace/test"}
	_, err := parseSelectedResources(resources)
	assert.ErrorContains(t, err, "v1alpha:test")
}

func TestParseSelectedResourcesIncorrectNamespace(t *testing.T) {
	resources := []string{"v1alpha:Application:namespace/test/unknown"}
	_, err := parseSelectedResources(resources)
	assert.ErrorContains(t, err, "v1alpha:Application:namespace/test/unknown")
}

func TestParseSelectedResourcesEmptyList(t *testing.T) {
	var resources []string
	operationResources, err := parseSelectedResources(resources)
	require.NoError(t, err)
	assert.Empty(t, operationResources)
}

func TestPrintApplicationTableNotWide(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: v1alpha1.ApplicationSpec{
				Destination: v1alpha1.ApplicationDestination{
					Server:    "http://localhost:8080",
					Namespace: "default",
				},
				Project: "prj",
			},
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Status: "OutOfSync",
				},
				Health: v1alpha1.HealthStatus{
					Status: "Healthy",
				},
			},
		}
		output := "table"
		printApplicationTable([]v1alpha1.Application{*app, *app}, &output)
		return nil
	})
	require.NoError(t, err)
	expectation := "NAME      CLUSTER                NAMESPACE  PROJECT  STATUS     HEALTH   SYNCPOLICY  CONDITIONS\napp-name  http://localhost:8080  default    prj      OutOfSync  Healthy  Manual      <none>\napp-name  http://localhost:8080  default    prj      OutOfSync  Healthy  Manual      <none>\n"
	assert.Equal(t, output, expectation)
}

func TestPrintApplicationTableWide(t *testing.T) {
	output, err := captureOutput(func() error {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "app-name",
			},
			Spec: v1alpha1.ApplicationSpec{
				Destination: v1alpha1.ApplicationDestination{
					Server:    "http://localhost:8080",
					Namespace: "default",
				},
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps",
					Path:           "guestbook",
					TargetRevision: "123",
				},
				Project: "prj",
			},
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Status: "OutOfSync",
				},
				Health: v1alpha1.HealthStatus{
					Status: "Healthy",
				},
			},
		}
		output := "wide"
		printApplicationTable([]v1alpha1.Application{*app, *app}, &output)
		return nil
	})
	require.NoError(t, err)
	expectation := "NAME      CLUSTER                NAMESPACE  PROJECT  STATUS     HEALTH   SYNCPOLICY  CONDITIONS  REPO                                             PATH       TARGET\napp-name  http://localhost:8080  default    prj      OutOfSync  Healthy  Manual      <none>      https://github.com/argoproj/argocd-example-apps  guestbook  123\napp-name  http://localhost:8080  default    prj      OutOfSync  Healthy  Manual      <none>      https://github.com/argoproj/argocd-example-apps  guestbook  123\n"
	assert.Equal(t, output, expectation)
}

func TestResourceStateKey(t *testing.T) {
	rst := resourceState{
		Group:     "group",
		Kind:      "kind",
		Namespace: "namespace",
		Name:      "name",
	}

	key := rst.Key()
	assert.Equal(t, "group/kind/namespace/name", key)
}

func TestFormatItems(t *testing.T) {
	rst := resourceState{
		Group:     "group",
		Kind:      "kind",
		Namespace: "namespace",
		Name:      "name",
		Status:    "status",
		Health:    "health",
		Hook:      "hook",
		Message:   "message",
	}
	items := rst.FormatItems()
	assert.Equal(t, "group", items[1])
	assert.Equal(t, "kind", items[2])
	assert.Equal(t, "namespace", items[3])
	assert.Equal(t, "name", items[4])
	assert.Equal(t, "status", items[5])
	assert.Equal(t, "health", items[6])
	assert.Equal(t, "hook", items[7])
	assert.Equal(t, "message", items[8])
}

func TestMerge(t *testing.T) {
	rst := resourceState{
		Group:     "group",
		Kind:      "kind",
		Namespace: "namespace",
		Name:      "name",
		Status:    "status",
		Health:    "health",
		Hook:      "hook",
		Message:   "message",
	}

	rstNew := resourceState{
		Group:     "group",
		Kind:      "kind",
		Namespace: "namespace",
		Name:      "name",
		Status:    "status",
		Health:    "health",
		Hook:      "hook2",
		Message:   "message2",
	}

	updated := rst.Merge(&rstNew)
	assert.True(t, updated)
	assert.Equal(t, rstNew.Hook, rst.Hook)
	assert.Equal(t, rstNew.Message, rst.Message)
	assert.Equal(t, rstNew.Status, rst.Status)
}

func TestMergeWitoutUpdate(t *testing.T) {
	rst := resourceState{
		Group:     "group",
		Kind:      "kind",
		Namespace: "namespace",
		Name:      "name",
		Status:    "status",
		Health:    "health",
		Hook:      "hook",
		Message:   "message",
	}

	rstNew := resourceState{
		Group:     "group",
		Kind:      "kind",
		Namespace: "namespace",
		Name:      "name",
		Status:    "status",
		Health:    "health",
		Hook:      "hook",
		Message:   "message",
	}

	updated := rst.Merge(&rstNew)
	assert.False(t, updated)
}

func TestCheckResourceStatus(t *testing.T) {
	t.Run("Degraded, Suspended and health status passed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: true,
			health:    true,
			degraded:  true,
		}, string(health.HealthStatusHealthy), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.True(t, res)
	})
	t.Run("Degraded, Suspended and health status failed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: true,
			health:    true,
			degraded:  true,
		}, string(health.HealthStatusProgressing), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.False(t, res)
	})
	t.Run("Suspended and health status passed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: true,
			health:    true,
		}, string(health.HealthStatusHealthy), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.True(t, res)
	})
	t.Run("Suspended and health status failed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: true,
			health:    true,
		}, string(health.HealthStatusProgressing), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.False(t, res)
	})
	t.Run("Suspended passed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: true,
			health:    false,
		}, string(health.HealthStatusSuspended), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.True(t, res)
	})
	t.Run("Suspended failed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: true,
			health:    false,
		}, string(health.HealthStatusProgressing), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.False(t, res)
	})
	t.Run("Health passed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: false,
			health:    true,
		}, string(health.HealthStatusHealthy), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.True(t, res)
	})
	t.Run("Health failed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: false,
			health:    true,
		}, string(health.HealthStatusProgressing), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.False(t, res)
	})
	t.Run("Synced passed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{}, string(health.HealthStatusProgressing), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.True(t, res)
	})
	t.Run("Synced failed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{}, string(health.HealthStatusProgressing), string(v1alpha1.SyncStatusCodeOutOfSync), &v1alpha1.Operation{})
		assert.True(t, res)
	})
	t.Run("Degraded passed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: false,
			health:    false,
			degraded:  true,
		}, string(health.HealthStatusDegraded), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.True(t, res)
	})
	t.Run("Degraded failed", func(t *testing.T) {
		res := checkResourceStatus(watchOpts{
			suspended: false,
			health:    false,
			degraded:  true,
		}, string(health.HealthStatusProgressing), string(v1alpha1.SyncStatusCodeSynced), &v1alpha1.Operation{})
		assert.False(t, res)
	})
}

func Test_hasAppChanged(t *testing.T) {
	type args struct {
		appReq *v1alpha1.Application
		appRes *v1alpha1.Application
		upsert bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "App has changed - Labels, Annotations, Finalizers empty",
			args: args{
				appReq: testApp("foo", "default", map[string]string{}, map[string]string{}, []string{}),
				appRes: testApp("foo", "foo", nil, nil, nil),
				upsert: true,
			},
			want: true,
		},
		{
			name: "App unchanged - Labels, Annotations, Finalizers populated",
			args: args{
				appReq: testApp("foo", "default", map[string]string{"foo": "bar"}, map[string]string{"foo": "bar"}, []string{"foo"}),
				appRes: testApp("foo", "default", map[string]string{"foo": "bar"}, map[string]string{"foo": "bar"}, []string{"foo"}),
				upsert: true,
			},
			want: false,
		},
		{
			name: "Apps unchanged - Using empty maps/list locally versus server returning nil",
			args: args{
				appReq: testApp("foo", "default", map[string]string{}, map[string]string{}, []string{}),
				appRes: testApp("foo", "default", nil, nil, nil),
				upsert: true,
			},
			want: false,
		},
		{
			name: "App unchanged - Using empty project locally versus server returning default",
			args: args{
				appReq: testApp("foo", "", map[string]string{}, map[string]string{}, []string{}),
				appRes: testApp("foo", "default", nil, nil, nil),
			},
			want: false,
		},
		{
			name: "App unchanged - From upsert=false",
			args: args{
				appReq: testApp("foo", "foo", map[string]string{}, map[string]string{}, []string{}),
				appRes: testApp("foo", "default", nil, nil, nil),
				upsert: false,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasAppChanged(tt.args.appReq, tt.args.appRes, tt.args.upsert); got != tt.want {
				t.Errorf("hasAppChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func testApp(name, project string, labels map[string]string, annotations map[string]string, finalizers []string) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
			Finalizers:  finalizers,
		},
		Spec: v1alpha1.ApplicationSpec{
			Source: &v1alpha1.ApplicationSource{
				RepoURL: "https://github.com/argoproj/argocd-example-apps.git",
			},
			Project: project,
		},
	}
}

func TestWaitOnApplicationStatus_JSON_YAML_WideOutput(t *testing.T) {
	acdClient := &customAcdClient{&fakeAcdClient{}}
	ctx := context.Background()
	var selectResource []*v1alpha1.SyncOperationResource
	watch := watchOpts{
		sync:      false,
		health:    false,
		operation: true,
		suspended: false,
	}
	watch = getWatchOpts(watch)

	output, err := captureOutput(func() error {
		_, _, _ = waitOnApplicationStatus(ctx, acdClient, "app-name", 0, watch, selectResource, "json")
		return nil
	},
	)
	require.NoError(t, err)
	assert.True(t, json.Valid([]byte(output)))

	output, err = captureOutput(func() error {
		_, _, _ = waitOnApplicationStatus(ctx, acdClient, "app-name", 0, watch, selectResource, "yaml")
		return nil
	})

	require.NoError(t, err)
	err = yaml.Unmarshal([]byte(output), &v1alpha1.Application{})
	require.NoError(t, err)

	output, _ = captureOutput(func() error {
		_, _, _ = waitOnApplicationStatus(ctx, acdClient, "app-name", 0, watch, selectResource, "")
		return nil
	})
	timeStr := time.Now().Format("2006-01-02T15:04:05-07:00")

	expectation := `TIMESTAMP                  GROUP        KIND   NAMESPACE                  NAME    STATUS   HEALTH        HOOK  MESSAGE
%s            Service     default         service-name1    Synced  Healthy              
%s   apps  Deployment     default                  test    Synced  Healthy              

Name:               argocd/test
Project:            default
Server:             local
Namespace:          argocd
URL:                http://localhost:8080/applications/app-name
Source:
- Repo:             test
  Target:           master
  Path:             /test
  Helm Values:      path1,path2
  Name Prefix:      prefix
SyncWindow:         Sync Allowed
Sync Policy:        Automated (Prune)
Sync Status:        OutOfSync from master
Health Status:      Progressing (health-message)

Operation:          Sync
Sync Revision:      revision
Phase:              
Start:              0001-01-01 00:00:00 +0000 UTC
Finished:           2020-11-10 23:00:00 +0000 UTC
Duration:           2333448h16m18.871345152s
Message:            test

GROUP  KIND        NAMESPACE  NAME           STATUS  HEALTH   HOOK  MESSAGE
       Service     default    service-name1  Synced  Healthy        
apps   Deployment  default    test           Synced  Healthy        
`
	expectation = fmt.Sprintf(expectation, timeStr, timeStr)
	expectationParts := strings.Split(expectation, "\n")
	slices.Sort(expectationParts)
	expectationSorted := strings.Join(expectationParts, "\n")
	outputParts := strings.Split(output, "\n")
	slices.Sort(outputParts)
	outputSorted := strings.Join(outputParts, "\n")
	// Need to compare sorted since map entries may not keep a specific order during serialization, leading to flakiness.
	assert.Equalf(t, expectationSorted, outputSorted, "Incorrect output %q, should be %q (items order doesn't matter)", output, expectation)
}

type customAcdClient struct {
	*fakeAcdClient
}

func (c *customAcdClient) WatchApplicationWithRetry(ctx context.Context, appName string, revision string) chan *v1alpha1.ApplicationWatchEvent {
	appEventsCh := make(chan *v1alpha1.ApplicationWatchEvent)
	_, appClient := c.NewApplicationClientOrDie()
	app, _ := appClient.Get(ctx, &applicationpkg.ApplicationQuery{})

	newApp := v1alpha1.Application{
		TypeMeta:   app.TypeMeta,
		ObjectMeta: app.ObjectMeta,
		Spec:       app.Spec,
		Status:     app.Status,
		Operation:  app.Operation,
	}

	go func() {
		appEventsCh <- &v1alpha1.ApplicationWatchEvent{
			Type:        watch.Bookmark,
			Application: newApp,
		}
		close(appEventsCh)
	}()

	return appEventsCh
}

func (c *customAcdClient) NewApplicationClientOrDie() (io.Closer, applicationpkg.ApplicationServiceClient) {
	return &fakeConnection{}, &fakeAppServiceClient{}
}

func (c *customAcdClient) NewSettingsClientOrDie() (io.Closer, settingspkg.SettingsServiceClient) {
	return &fakeConnection{}, &fakeSettingsServiceClient{}
}

type fakeConnection struct{}

func (c *fakeConnection) Close() error {
	return nil
}

type fakeSettingsServiceClient struct{}

func (f fakeSettingsServiceClient) Get(ctx context.Context, in *settingspkg.SettingsQuery, opts ...grpc.CallOption) (*settingspkg.Settings, error) {
	return &settingspkg.Settings{
		URL: "http://localhost:8080",
	}, nil
}

func (f fakeSettingsServiceClient) GetPlugins(ctx context.Context, in *settingspkg.SettingsQuery, opts ...grpc.CallOption) (*settingspkg.SettingsPluginsResponse, error) {
	return nil, nil
}

type fakeAppServiceClient struct{}

func (c *fakeAppServiceClient) Get(ctx context.Context, in *applicationpkg.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	time := metav1.Date(2020, time.November, 10, 23, 0, 0, 0, time.UTC)
	return &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune: true,
				},
			},
			Project:     "default",
			Destination: v1alpha1.ApplicationDestination{Server: "local", Namespace: "argocd"},
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "test",
				TargetRevision: "master",
				Path:           "/test",
				Helm: &v1alpha1.ApplicationSourceHelm{
					ValueFiles: []string{"path1", "path2"},
				},
				Kustomize: &v1alpha1.ApplicationSourceKustomize{NamePrefix: "prefix"},
			},
		},
		Status: v1alpha1.ApplicationStatus{
			Resources: []v1alpha1.ResourceStatus{
				{
					Group:     "",
					Kind:      "Service",
					Namespace: "default",
					Name:      "service-name1",
					Status:    "Synced",
					Health: &v1alpha1.HealthStatus{
						Status:  health.HealthStatusHealthy,
						Message: "health-message",
					},
				},
				{
					Group:     "apps",
					Kind:      "Deployment",
					Namespace: "default",
					Name:      "test",
					Status:    "Synced",
					Health: &v1alpha1.HealthStatus{
						Status:  health.HealthStatusHealthy,
						Message: "health-message",
					},
				},
			},
			OperationState: &v1alpha1.OperationState{
				SyncResult: &v1alpha1.SyncOperationResult{
					Revision: "revision",
				},
				FinishedAt: &time,
				Message:    "test",
			},
			Sync: v1alpha1.SyncStatus{
				Status: v1alpha1.SyncStatusCodeOutOfSync,
			},
			Health: v1alpha1.HealthStatus{
				Status:  health.HealthStatusProgressing,
				Message: "health-message",
			},
		},
	}, nil
}

func (c *fakeAppServiceClient) List(ctx context.Context, in *applicationpkg.ApplicationQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationList, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) ListResourceEvents(ctx context.Context, in *applicationpkg.ApplicationResourceEventsQuery, opts ...grpc.CallOption) (*v1.EventList, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) Watch(ctx context.Context, in *applicationpkg.ApplicationQuery, opts ...grpc.CallOption) (applicationpkg.ApplicationService_WatchClient, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) Create(ctx context.Context, in *applicationpkg.ApplicationCreateRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) GetApplicationSyncWindows(ctx context.Context, in *applicationpkg.ApplicationSyncWindowsQuery, opts ...grpc.CallOption) (*applicationpkg.ApplicationSyncWindowsResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) RevisionMetadata(ctx context.Context, in *applicationpkg.RevisionMetadataQuery, opts ...grpc.CallOption) (*v1alpha1.RevisionMetadata, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) RevisionChartDetails(ctx context.Context, in *applicationpkg.RevisionMetadataQuery, opts ...grpc.CallOption) (*v1alpha1.ChartDetails, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) GetManifests(ctx context.Context, in *applicationpkg.ApplicationManifestQuery, opts ...grpc.CallOption) (*apiclient.ManifestResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) GetManifestsWithFiles(ctx context.Context, opts ...grpc.CallOption) (applicationpkg.ApplicationService_GetManifestsWithFilesClient, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) Update(ctx context.Context, in *applicationpkg.ApplicationUpdateRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) UpdateSpec(ctx context.Context, in *applicationpkg.ApplicationUpdateSpecRequest, opts ...grpc.CallOption) (*v1alpha1.ApplicationSpec, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) Patch(ctx context.Context, in *applicationpkg.ApplicationPatchRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) Delete(ctx context.Context, in *applicationpkg.ApplicationDeleteRequest, opts ...grpc.CallOption) (*applicationpkg.ApplicationResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) Sync(ctx context.Context, in *applicationpkg.ApplicationSyncRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) ManagedResources(ctx context.Context, in *applicationpkg.ResourcesQuery, opts ...grpc.CallOption) (*applicationpkg.ManagedResourcesResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) ResourceTree(ctx context.Context, in *applicationpkg.ResourcesQuery, opts ...grpc.CallOption) (*v1alpha1.ApplicationTree, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) WatchResourceTree(ctx context.Context, in *applicationpkg.ResourcesQuery, opts ...grpc.CallOption) (applicationpkg.ApplicationService_WatchResourceTreeClient, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) Rollback(ctx context.Context, in *applicationpkg.ApplicationRollbackRequest, opts ...grpc.CallOption) (*v1alpha1.Application, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) TerminateOperation(ctx context.Context, in *applicationpkg.OperationTerminateRequest, opts ...grpc.CallOption) (*applicationpkg.OperationTerminateResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) GetResource(ctx context.Context, in *applicationpkg.ApplicationResourceRequest, opts ...grpc.CallOption) (*applicationpkg.ApplicationResourceResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) PatchResource(ctx context.Context, in *applicationpkg.ApplicationResourcePatchRequest, opts ...grpc.CallOption) (*applicationpkg.ApplicationResourceResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) ListResourceActions(ctx context.Context, in *applicationpkg.ApplicationResourceRequest, opts ...grpc.CallOption) (*applicationpkg.ResourceActionsListResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) RunResourceAction(ctx context.Context, in *applicationpkg.ResourceActionRunRequest, opts ...grpc.CallOption) (*applicationpkg.ApplicationResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) DeleteResource(ctx context.Context, in *applicationpkg.ApplicationResourceDeleteRequest, opts ...grpc.CallOption) (*applicationpkg.ApplicationResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) PodLogs(ctx context.Context, in *applicationpkg.ApplicationPodLogsQuery, opts ...grpc.CallOption) (applicationpkg.ApplicationService_PodLogsClient, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) ListLinks(ctx context.Context, in *applicationpkg.ListAppLinksRequest, opts ...grpc.CallOption) (*applicationpkg.LinksResponse, error) {
	return nil, nil
}

func (c *fakeAppServiceClient) ListResourceLinks(ctx context.Context, in *applicationpkg.ApplicationResourceRequest, opts ...grpc.CallOption) (*applicationpkg.LinksResponse, error) {
	return nil, nil
}

type fakeAcdClient struct{}

func (c *fakeAcdClient) ClientOptions() argocdclient.ClientOptions {
	return argocdclient.ClientOptions{}
}
func (c *fakeAcdClient) HTTPClient() (*http.Client, error) { return nil, nil }
func (c *fakeAcdClient) OIDCConfig(context.Context, *settingspkg.Settings) (*oauth2.Config, *oidc.Provider, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewRepoClient() (io.Closer, repositorypkg.RepositoryServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewRepoClientOrDie() (io.Closer, repositorypkg.RepositoryServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewRepoCredsClient() (io.Closer, repocredspkg.RepoCredsServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewRepoCredsClientOrDie() (io.Closer, repocredspkg.RepoCredsServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewCertClient() (io.Closer, certificatepkg.CertificateServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewCertClientOrDie() (io.Closer, certificatepkg.CertificateServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewClusterClient() (io.Closer, clusterpkg.ClusterServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewClusterClientOrDie() (io.Closer, clusterpkg.ClusterServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewGPGKeyClient() (io.Closer, gpgkeypkg.GPGKeyServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewGPGKeyClientOrDie() (io.Closer, gpgkeypkg.GPGKeyServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewApplicationClient() (io.Closer, applicationpkg.ApplicationServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewApplicationSetClient() (io.Closer, applicationsetpkg.ApplicationSetServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewApplicationClientOrDie() (io.Closer, applicationpkg.ApplicationServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewApplicationSetClientOrDie() (io.Closer, applicationsetpkg.ApplicationSetServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewNotificationClient() (io.Closer, notificationpkg.NotificationServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewNotificationClientOrDie() (io.Closer, notificationpkg.NotificationServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewSessionClient() (io.Closer, sessionpkg.SessionServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewSessionClientOrDie() (io.Closer, sessionpkg.SessionServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewSettingsClient() (io.Closer, settingspkg.SettingsServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewSettingsClientOrDie() (io.Closer, settingspkg.SettingsServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewVersionClient() (io.Closer, versionpkg.VersionServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewVersionClientOrDie() (io.Closer, versionpkg.VersionServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewProjectClient() (io.Closer, projectpkg.ProjectServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewProjectClientOrDie() (io.Closer, projectpkg.ProjectServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) NewAccountClient() (io.Closer, accountpkg.AccountServiceClient, error) {
	return nil, nil, nil
}

func (c *fakeAcdClient) NewAccountClientOrDie() (io.Closer, accountpkg.AccountServiceClient) {
	return nil, nil
}

func (c *fakeAcdClient) WatchApplicationWithRetry(ctx context.Context, appName string, revision string) chan *v1alpha1.ApplicationWatchEvent {
	appEventsCh := make(chan *v1alpha1.ApplicationWatchEvent)

	go func() {
		modifiedEvent := new(v1alpha1.ApplicationWatchEvent)
		modifiedEvent.Type = k8swatch.Modified
		appEventsCh <- modifiedEvent
		deletedEvent := new(v1alpha1.ApplicationWatchEvent)
		deletedEvent.Type = k8swatch.Deleted
		appEventsCh <- deletedEvent
	}()
	return appEventsCh
}
