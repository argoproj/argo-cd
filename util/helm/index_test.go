package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndex(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		_, err := GetIndex("", "", "")
		assert.Error(t, err)
	})
	t.Run("Stable", func(t *testing.T) {
		index, err := GetIndex("https://kubernetes-charts.storage.googleapis.com", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})
	t.Run("BasicAuth", func(t *testing.T) {
		index, err := GetIndex("https://kubernetes-charts.storage.googleapis.com", "my-username", "my-password")
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})
}
