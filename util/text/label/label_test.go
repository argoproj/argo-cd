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
