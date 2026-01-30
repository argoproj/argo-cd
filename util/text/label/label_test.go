package label

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLabels(t *testing.T) {
	validLabels := []string{"key=value", "foo=bar", "intuit=inc"}

	result, err := Parse(validLabels)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, "bar", result["foo"])
	assert.Equal(t, "inc", result["intuit"])

	invalidLabels := []string{"key=value", "too=many=equals"}
	_, err = Parse(invalidLabels)
	require.Error(t, err)

	emptyLabels := []string{}
	result, err = Parse(emptyLabels)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseRejectsDeletionSyntax(t *testing.T) {
	// Original Parse() should reject deletion syntax
	_, err := Parse([]string{"x-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deletion syntax not supported")

	_, err = Parse([]string{"x=5", "y-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deletion syntax not supported")

	// ParseLabelMap should accept deletion syntax
	lm, err := ParseLabelMap([]string{"x-"})
	require.NoError(t, err)
	assert.True(t, lm.Updates["x"].Delete)
}

func TestParseLabelMap(t *testing.T) {
	// Test basic parsing
	result, err := ParseLabelMap([]string{"x=5", "y=10"})
	require.NoError(t, err)
	assert.Len(t, result.Updates, 2)
	assert.Equal(t, "5", result.Updates["x"].Value)
	assert.False(t, result.Updates["x"].Delete)
	assert.Equal(t, "10", result.Updates["y"].Value)
	assert.False(t, result.Updates["y"].Delete)

	// Test deletion syntax
	result, err = ParseLabelMap([]string{"x-"})
	require.NoError(t, err)
	assert.Len(t, result.Updates, 1)
	assert.True(t, result.Updates["x"].Delete)

	// Test mixed syntax
	result, err = ParseLabelMap([]string{"x=5", "y-"})
	require.NoError(t, err)
	assert.Len(t, result.Updates, 2)
	assert.Equal(t, "5", result.Updates["x"].Value)
	assert.False(t, result.Updates["x"].Delete)
	assert.True(t, result.Updates["y"].Delete)

	// Test invalid deletion syntax
	_, err = ParseLabelMap([]string{"-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid label deletion syntax")
}

func TestLabelMapMerge(t *testing.T) {
	// Test deletion
	lm, _ := ParseLabelMap([]string{"x-"})
	result := lm.Merge(map[string]string{"x": "5", "y": "10"})
	assert.Len(t, result, 1)
	assert.Equal(t, "10", result["y"])
	assert.NotContains(t, result, "x")

	// Test update and deletion
	lm, _ = ParseLabelMap([]string{"x=6", "y-"})
	result = lm.Merge(map[string]string{"x": "5", "y": "10"})
	assert.Len(t, result, 1)
	assert.Equal(t, "6", result["x"])

	// Test deletion of non-existing key
	lm, _ = ParseLabelMap([]string{"z-"})
	result = lm.Merge(map[string]string{"x": "5"})
	assert.Len(t, result, 1)
	assert.Equal(t, "5", result["x"])

	// Test update only
	lm, _ = ParseLabelMap([]string{"x=6"})
	result = lm.Merge(map[string]string{"x": "5", "y": "10"})
	assert.Len(t, result, 2)
	assert.Equal(t, "6", result["x"])
	assert.Equal(t, "10", result["y"])
}

func TestLabelMapPlain(t *testing.T) {
	lm, _ := ParseLabelMap([]string{"x=5", "y-", "z=10"})

	result := lm.Plain()
	assert.Len(t, result, 2)
	assert.Equal(t, "5", result["x"])
	assert.Equal(t, "10", result["z"])
	assert.NotContains(t, result, "y")
}

func TestNewLabelMap(t *testing.T) {
	lm := NewLabelMap()
	assert.NotNil(t, lm)
	assert.NotNil(t, lm.Updates)
	assert.Empty(t, lm.Updates)
}
