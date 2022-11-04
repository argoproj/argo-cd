package image

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_MatchFuncAny(t *testing.T) {
	assert.True(t, MatchFuncAny("whatever", nil))
}

func Test_MatchFuncNone(t *testing.T) {
	assert.False(t, MatchFuncNone("whatever", nil))
}

func Test_MatchFuncRegexp(t *testing.T) {
	t.Run("Test with valid expression", func(t *testing.T) {
		re := regexp.MustCompile("[a-z]+")
		assert.True(t, MatchFuncRegexp("lemon", re))
		assert.False(t, MatchFuncRegexp("31337", re))
	})
	t.Run("Test with invalid type", func(t *testing.T) {
		assert.False(t, MatchFuncRegexp("lemon", "[a-z]+"))
	})
}
