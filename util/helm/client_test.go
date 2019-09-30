package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndex(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		_, err := NewClient("", Creds{}).GetIndex()
		assert.Error(t, err)
	})
	t.Run("Stable", func(t *testing.T) {
		index, err := NewClient("https://kubernetes-charts.storage.googleapis.com", Creds{}).GetIndex()
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})
	t.Run("BasicAuth", func(t *testing.T) {
		index, err := NewClient("https://kubernetes-charts.storage.googleapis.com", Creds{
			Username: "my-password",
			Password: "my-username",
		}).GetIndex()
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})
}
