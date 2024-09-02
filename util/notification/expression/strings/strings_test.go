package strings

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewExprs(t *testing.T) {
	funcs := []string{
		"ReplaceAll",
		"ToUpper",
		"ToLower",
	}
	for _, fn := range funcs {
		stringsExprs := NewExprs()
		_, hasFunc := stringsExprs[fn]
		assert.True(t, hasFunc)
	}
}

func TestReplaceAll(t *testing.T) {
	exprs := NewExprs()
	input := "test_replace"
	expected := "test=replace"
	replaceAllFn, ok := exprs["ReplaceAll"].(func(s, old, new string) string)
	assert.True(t, ok)
	actual := replaceAllFn(input, "_", "=")
	assert.Equal(t, expected, actual)
}

func TestUpperAndLower(t *testing.T) {
	testCases := []struct {
		fn       string
		input    string
		expected string
	}{
		{
			fn:       "ToUpper",
			input:    "test",
			expected: "TEST",
		},
		{
			fn:       "ToLower",
			input:    "TEST",
			expected: "test",
		},
	}
	exprs := NewExprs()

	for _, testCase := range testCases {
		t.Run(fmt.Sprintf("With success case: Func: %s", testCase.fn), func(tc *testing.T) {
			toUpperFn, ok := exprs[testCase.fn].(func(s string) string)
			assert.True(t, ok)

			actual := toUpperFn(testCase.input)
			assert.Equal(t, testCase.expected, actual)
		})
	}
}
