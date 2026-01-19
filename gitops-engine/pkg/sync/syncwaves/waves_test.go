package syncwaves

import (
	"testing"

	"github.com/stretchr/testify/assert"

	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestWave(t *testing.T) {
	assert.Equal(t, 0, Wave(testingutils.NewPod()))
	assert.Equal(t, 1, Wave(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave", "1")))
	assert.Equal(t, 0, WaveGroup(testingutils.NewPod()))
	assert.Equal(t, 1, WaveGroup(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group", "1")))
	assert.Equal(t, []int{}, WaveGroupDependencies(testingutils.NewPod()))
	assert.Equal(t, []int{}, WaveGroupDependencies(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group-dependencies", "1,2")))
	assert.Equal(t, []int{1}, WaveGroupDependencies(testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group-dependencies", "1,2"), "argocd.argoproj.io/sync-wave-group", "2")))
	assert.Equal(t, []int{1, 2}, WaveGroupDependencies(testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group-dependencies", "1,2"), "argocd.argoproj.io/sync-wave-group", "3")))
	assert.Equal(t, []int{-1, 2}, WaveGroupDependencies(testingutils.Annotate(testingutils.Annotate(testingutils.NewPod(), "argocd.argoproj.io/sync-wave-group-dependencies", "-1,2"), "argocd.argoproj.io/sync-wave-group", "3")))
	assert.Equal(t, 1, Wave(testingutils.Annotate(testingutils.NewPod(), "helm.sh/hook-weight", "1")))
}
