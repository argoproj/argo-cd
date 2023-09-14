package time

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewTimeExprs(t *testing.T) {
	funcs := []string{
		"Parse",
		"Now",
		"Format",
	}

	for _, fn := range funcs {
		timeExprs := NewExprs()
		_, hasFunc := timeExprs[fn]
		assert.True(t, hasFunc)
	}
}

func TestTimeFormat(t *testing.T) {
	assert.NotEmpty(t, format(now()))
}
