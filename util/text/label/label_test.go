package label

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLabels(t *testing.T) {
	validLabels := []string{"key=value", "foo=bar", "intuit=inc"}

	result, err := Parse(validLabels)
	assert.NoError(t, err)
	assert.Len(t, result, 3)

	invalidLabels := []string{"key=value", "too=many=equals"}
	_, err = Parse(invalidLabels)
	assert.Error(t, err)

	emptyLabels := []string{}
	result, err = Parse(emptyLabels)
	assert.NoError(t, err)
	assert.Len(t, result, 0)
}
