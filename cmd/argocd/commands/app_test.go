package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseLabels(t *testing.T) {
	validLabels := []string{"key=value", "foo=bar", "intuit=inc"}

	result, err := parseLabels(validLabels)
	assert.NoError(t, err)
	assert.Len(t, result, 3)

	invalidLabels := []string{"key=value", "too=many=equals"}
	_, err = parseLabels(invalidLabels)
	assert.Error(t, err)

	emptyLabels := []string{}
	result, err = parseLabels(emptyLabels)
	assert.NoError(t, err)
	assert.Len(t, result, 0)

}
