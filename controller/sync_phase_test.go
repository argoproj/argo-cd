package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
)

func TestSyncPhaseNone(t *testing.T) {
	assert.Equal(t, []SyncPhase{SyncPhaseSync}, syncPhases(&unstructured.Unstructured{}))
}

func TestSyncPhasePreSync(t *testing.T) {
	assert.Equal(t, []SyncPhase{SyncPhasePreSync}, syncPhases(pod("PreSync")))
}

func TestSyncPhaseSync(t *testing.T) {
	assert.Equal(t, []SyncPhase{SyncPhaseSync}, syncPhases(pod("Sync")))
}

func TestSyncPhaseSkip(t *testing.T) {
	assert.Nil(t, syncPhases(pod("Skip")))
}

// garbage hooks are still hooks, but have no phases, because some user spelled something wrong
func TestSyncPhaseGarbage(t *testing.T) {
	assert.Nil(t, syncPhases(pod("Garbage")))
}

func TestSyncPhasePost(t *testing.T) {
	assert.Equal(t, []SyncPhase{SyncPhasePostSync}, syncPhases(pod("PostSync")))
}

func TestSyncPhaseFail(t *testing.T) {
	assert.Equal(t, []SyncPhase{SyncPhaseSyncFail}, syncPhases(pod("SyncFail")))
}

func TestSyncPhaseTwoPhases(t *testing.T) {
	assert.ElementsMatch(t, []SyncPhase{SyncPhasePreSync, SyncPhasePostSync}, syncPhases(pod("PreSync,PostSync")))
}

func pod(hookType string) *unstructured.Unstructured {
	return test.Annotate(test.NewPod(), "argocd.argoproj.io/hook", hookType)
}
