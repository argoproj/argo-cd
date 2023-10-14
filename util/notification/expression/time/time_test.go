package time

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTimeExprs(t *testing.T) {
	funcs := []string{
		"Parse",
		"Now",
	}

	for _, fn := range funcs {
		timeExprs := NewExprs()
		_, hasFunc := timeExprs[fn]
		assert.True(t, hasFunc)
	}
}
