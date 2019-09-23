package repo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRepo(t *testing.T) {
	t.Run("Unnamed", func(t *testing.T) {
		_, err := NewRepo("http://0.0.0.0", "", "", "", nil, nil, nil)
		assert.EqualError(t, err, "must name repo")
	})
	t.Run("Valid", func(t *testing.T) {
		_, err := NewRepo("https://kubernetes-charts.storage.googleapis.com", "test", "", "", nil, nil, nil)
		assert.NoError(t, err)
	})
}
