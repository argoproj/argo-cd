package path

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestAppFilesHaveChanged_MonoRepoSiblingPaths(t *testing.T) {
	t.Parallel()

	app1Paths := GetSourceRefreshPaths(
		getApp(new("."), new("app/testargo1")),
		v1alpha1.ApplicationSource{Path: "app/testargo1"},
	)
	app2Paths := GetSourceRefreshPaths(
		getApp(new("."), new("app/testargo2")),
		v1alpha1.ApplicationSource{Path: "app/testargo2"},
	)

	changedFiles := []string{"app/testargo2/values.yaml"}

	assert.False(t, AppFilesHaveChanged(app1Paths, changedFiles), "sibling app path change must not affect testargo1")
	assert.True(t, AppFilesHaveChanged(app2Paths, changedFiles), "testargo2 must refresh when its own path changes")
}

func TestAppFilesHaveChanged_EmptyChangedFilesAssumesRefresh(t *testing.T) {
	t.Parallel()

	paths := []string{"app/testargo1"}
	assert.True(t, AppFilesHaveChanged(paths, nil))
}
