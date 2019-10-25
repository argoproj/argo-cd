package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNames(t *testing.T) {
	names := Names()
	assert.Len(t, names, 2)
}

func TestGet(t *testing.T) {
	t.Run("Dummy", func(t *testing.T) {
		result, err := Get("test-dummy").Validate("{}")
		assert.NoError(t, err)
		assert.True(t, result.Valid())
	})
	t.Run("Helm", func(t *testing.T) {
		t.Run("Empty", func(t *testing.T) {
			result, err := Get("helm-v3").Validate("{}")
			assert.NoError(t, err)
			assert.True(t, result.Valid())
		})
		t.Run("Valid", func(t *testing.T) {
			result, err := Get("helm-v3").Validate(`{"valueFiles": []}`)
			assert.NoError(t, err)
			assert.True(t, result.Valid())
			assert.Empty(t, result.Errors())
		})
		t.Run("Error", func(t *testing.T) {
			_, err := Get("helm-v3").Validate(`???`)
			assert.Error(t, err)
		})
		t.Run("Invalid", func(t *testing.T) {
			result, err := Get("helm-v3").Validate(`{"invalid": true}`)
			assert.NoError(t, err)
			assert.True(t, result.Valid())
			assert.Empty(t, result.Errors())
		})
	})
}
