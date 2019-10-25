package plugins

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNames(t *testing.T) {
	names := Names()
	assert.Len(t, names, 2)
}
