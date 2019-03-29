package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoFactory(t *testing.T) {
	t.Run("Unnamed", func(t *testing.T) {
		_, err := RepoFactory{}.GetRepo("http://0.0.0.0", "", "", "", nil, nil, nil)
		assert.EqualError(t, err, "must name repo")
	})

	t.Run("GarbageRepo", func(t *testing.T) {
		_, err := RepoFactory{}.GetRepo("http://0.0.0.0", "test", "", "", nil, nil, nil)
		assert.Error(t, err)
	})

	t.Run("Valid", func(t *testing.T) {
		_, err := RepoFactory{}.GetRepo("https://kubernetes-charts.storage.googleapis.com", "test", "", "", nil, nil, nil)
		assert.NoError(t, err)
	})
}
