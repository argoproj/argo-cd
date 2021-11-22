package strings

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewExprs(t *testing.T) {
	funcs := []string{
		"ReplaceAll",
	}
	for _, fn := range funcs {
		stringsExprs := NewExprs()
		_, hasFunc := stringsExprs[fn]
		assert.True(t, hasFunc)
	}
}
