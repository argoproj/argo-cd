package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoFactory(t *testing.T) {

	t.Run("GarbageUrl", func(t *testing.T) {
		_, err := RepoFactory{}.GetRepo("xxx", "", "", "", false)
		assert.EqualError(t, err, "repository not found")
	})

	t.Run("ValidRepo", func(t *testing.T) {
		_, err := RepoFactory{}.GetRepo("https://github.com/argoproj/argocd-example-apps.git", "", "", "", false)
		assert.NoError(t, err)
	})
}
