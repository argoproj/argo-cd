package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSparseCheckoutKey(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected string
	}{
		{
			name:     "no sparse paths returns empty string",
			paths:    []string{},
			expected: "",
		},
		{
			name:     "nil sparse paths returns empty string",
			paths:    nil,
			expected: "",
		},
		{
			name:     "single path",
			paths:    []string{"apps/frontend"},
			expected: "sparse:apps/frontend",
		},
		{
			name:     "multiple paths sorted alphabetically",
			paths:    []string{"apps/backend", "apps/frontend"},
			expected: "sparse:apps/backend,apps/frontend",
		},
		{
			name:     "paths sorted regardless of input order",
			paths:    []string{"apps/frontend", "apps/backend", "apps/database"},
			expected: "sparse:apps/backend,apps/database,apps/frontend",
		},
		{
			name:     "paths with different prefixes",
			paths:    []string{"infrastructure/terraform", "apps/frontend"},
			expected: "sparse:apps/frontend,infrastructure/terraform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSparseCheckoutKey(tt.paths)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSparseCheckoutKey_Consistency(t *testing.T) {
	// Same paths in different order should produce same key
	paths1 := []string{"apps/frontend", "apps/backend", "apps/database"}
	paths2 := []string{"apps/database", "apps/frontend", "apps/backend"}
	paths3 := []string{"apps/backend", "apps/database", "apps/frontend"}

	key1 := GetSparseCheckoutKey(paths1)
	key2 := GetSparseCheckoutKey(paths2)
	key3 := GetSparseCheckoutKey(paths3)

	assert.Equal(t, key1, key2)
	assert.Equal(t, key2, key3)
	assert.Equal(t, "sparse:apps/backend,apps/database,apps/frontend", key1)
}

func TestGetSparseCheckoutKey_DifferentPathsProduceDifferentKeys(t *testing.T) {
	// Different sparse paths should produce different cache keys
	paths1 := []string{"apps/frontend"}
	paths2 := []string{"apps/backend"}
	paths3 := []string{"apps/frontend", "apps/backend"}

	key1 := GetSparseCheckoutKey(paths1)
	key2 := GetSparseCheckoutKey(paths2)
	key3 := GetSparseCheckoutKey(paths3)

	// All keys should be different
	assert.NotEqual(t, key1, key2)
	assert.NotEqual(t, key1, key3)
	assert.NotEqual(t, key2, key3)

	// Verify they have expected format
	assert.Equal(t, "sparse:apps/frontend", key1)
	assert.Equal(t, "sparse:apps/backend", key2)
	assert.Equal(t, "sparse:apps/backend,apps/frontend", key3)
}
