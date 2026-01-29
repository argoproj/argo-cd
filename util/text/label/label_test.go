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

	invalidLabels := []string{"key=value", "too=many=equals"}
	_, err = Parse(invalidLabels)
	require.Error(t, err)

	emptyLabels := []string{}
	result, err = Parse(emptyLabels)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseDeletion(t *testing.T) {
	result, err := Parse([]string{"x-"})
	require.NoError(t, err)
	assert.Equal(t, deletionMarker, result["x"])

	result, err = Parse([]string{"x=5", "y-"})
	require.NoError(t, err)
	assert.Equal(t, "5", result["x"])
	assert.Equal(t, deletionMarker, result["y"])

	_, err = Parse([]string{"-"})
	require.Error(t, err)
}

func TestMerge(t *testing.T) {
	result := Merge(map[string]string{"x": "5", "y": "10"}, map[string]string{"x": deletionMarker})
	assert.Len(t, result, 1)
	assert.Equal(t, "10", result["y"])

	result = Merge(map[string]string{"x": "5", "y": "10"}, map[string]string{"x": "6", "y": deletionMarker})
	assert.Len(t, result, 1)
	assert.Equal(t, "6", result["x"])

	result = Merge(map[string]string{"x": "5"}, map[string]string{"y": deletionMarker})
	assert.Len(t, result, 1)
	assert.Equal(t, "5", result["x"])
}
