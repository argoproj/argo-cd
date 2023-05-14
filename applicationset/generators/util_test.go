package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlattenParameters(t *testing.T) {
	testCases := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "empty",
			input:    map[string]interface{}{},
			expected: map[string]string{},
		}, {
			name:     "flat",
			input:    map[string]interface{}{"foo": "bar"},
			expected: map[string]string{"foo": "bar"},
		}, {
			name: "nested string interface map",
			input: map[string]interface{}{
				"foo": map[string]interface{}{"bar": "baz"},
			},
			expected: map[string]string{"foo.bar": "baz"},
		}, {
			name: "nested multi-type",
			input: map[string]interface{}{
				"integer": 100,
				"array": []string{
					"first",
					"second",
				},
			},
			expected: map[string]string{
				"integer": "100",
				"array.0": "first",
				"array.1": "second",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			result := flattenParameters(testCase.input)
			assert.Equal(t, testCase.expected, result)
		})
	}
}
