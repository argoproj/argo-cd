package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPullRequestGeneratorLegacyProtobufFields(t *testing.T) {
	// Field 10 is a values map entry and field 11 is a boolean in the existing API.
	legacy := []byte{0x52, 6, 0x0a, 1, 'k', 0x12, 1, 'v', 0x58, 1}
	var generator PullRequestGenerator
	require.NoError(t, generator.Unmarshal(legacy))
	assert.Equal(t, map[string]string{"k": "v"}, generator.Values)
	assert.True(t, generator.ContinueOnRepoNotFoundError)
	assert.Nil(t, generator.ScmManager)

	encoded, err := generator.Marshal()
	require.NoError(t, err)
	assert.Contains(t, string(encoded), string(legacy))
}
