package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetSparseCheckoutKey(t *testing.T) {
	tests := []struct {
		name          string
		paths         []string
		expectEmpty   bool
		expectPrefix  string
	}{
		{
			name:        "no sparse paths returns empty string",
			paths:       []string{},
			expectEmpty: true,
		},
		{
			name:        "nil sparse paths returns empty string",
			paths:       nil,
			expectEmpty: true,
		},
		{
			name:         "single path produces hash",
			paths:        []string{"apps/frontend"},
			expectPrefix: "sparse:",
		},
		{
			name:         "multiple paths produce hash",
			paths:        []string{"apps/backend", "apps/frontend"},
			expectPrefix: "sparse:",
		},
		{
			name:         "paths with whitespace are trimmed",
			paths:        []string{" apps/frontend ", "apps/backend"},
			expectPrefix: "sparse:",
		},
		{
			name:         "paths with trailing slashes are normalized",
			paths:        []string{"apps/frontend/", "apps/backend"},
			expectPrefix: "sparse:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetSparseCheckoutKey(tt.paths)
			if tt.expectEmpty {
				assert.Empty(t, result)
			} else {
				assert.NotEmpty(t, result)
				assert.Contains(t, result, tt.expectPrefix)
				// Hash output should be 16 hex chars (8 bytes) + prefix
				assert.Len(t, result, len("sparse:")+16)
			}
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
	assert.Contains(t, key1, "sparse:")
	
	// Test that whitespace and trailing slashes don't affect the hash
	paths4 := []string{" apps/frontend/ ", "apps/backend/", "  apps/database  "}
	key4 := GetSparseCheckoutKey(paths4)
	assert.Equal(t, key1, key4)
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

	// Verify they all have expected format (sparse: prefix + 16 hex chars)
	assert.Contains(t, key1, "sparse:")
	assert.Contains(t, key2, "sparse:")
	assert.Contains(t, key3, "sparse:")
	assert.Len(t, key1, len("sparse:")+16)
	assert.Len(t, key2, len("sparse:")+16)
	assert.Len(t, key3, len("sparse:")+16)
}
