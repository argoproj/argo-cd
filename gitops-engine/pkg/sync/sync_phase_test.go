package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestSyncPhaseNone(t *testing.T) {
	assert.Equal(t, []common.SyncPhase{common.SyncPhaseSync}, syncPhases(&unstructured.Unstructured{}))
}

func TestSyncPhasePreSync(t *testing.T) {
	assert.Equal(t, []common.SyncPhase{common.SyncPhasePreSync}, syncPhases(pod("PreSync")))
}

func TestSyncPhaseSync(t *testing.T) {
	assert.Equal(t, []common.SyncPhase{common.SyncPhaseSync}, syncPhases(pod("Sync")))
}

func TestSyncPhaseSkip(t *testing.T) {
	assert.Nil(t, syncPhases(pod("Skip")))
}

// garbage hooks are still hooks, but have no phases, because some user spelled something wrong
func TestSyncPhaseGarbage(t *testing.T) {
	assert.Nil(t, syncPhases(pod("Garbage")))
}

func TestSyncPhasePost(t *testing.T) {
	assert.Equal(t, []common.SyncPhase{common.SyncPhasePostSync}, syncPhases(pod("PostSync")))
}

func TestSyncPhaseFail(t *testing.T) {
	assert.Equal(t, []common.SyncPhase{common.SyncPhaseSyncFail}, syncPhases(pod("SyncFail")))
}

func TestSyncPhaseTwoPhases(t *testing.T) {
	assert.ElementsMatch(t, []common.SyncPhase{common.SyncPhasePreSync, common.SyncPhasePostSync}, syncPhases(pod("PreSync,PostSync")))
}

func TestSyncDuplicatedPhases(t *testing.T) {
	assert.ElementsMatch(t, []common.SyncPhase{common.SyncPhasePreSync}, syncPhases(pod("PreSync,PreSync")))
	assert.ElementsMatch(t, []common.SyncPhase{common.SyncPhasePreSync}, syncPhases(podWithHelmHook("pre-install,pre-upgrade")))
}

func pod(hookType string) *unstructured.Unstructured {
	return testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/hook", hookType)
}

func podWithHelmHook(hookType string) *unstructured.Unstructured {
	return testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook", hookType)
}
