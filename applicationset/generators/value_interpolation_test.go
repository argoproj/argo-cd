package generators

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueInterpolation(t *testing.T) {
	testCases := []struct {
		name     string
		values   map[string]string
		params   map[string]any
		expected map[string]any
	}{
		{
			name: "Simple interpolation",
			values: map[string]string{
				"hello": "{{ world }}",
			},
			params: map[string]any{
				"world": "world!",
			},
			expected: map[string]any{
				"world":        "world!",
				"values.hello": "world!",
			},
		},
		{
			name: "Non-existent",
			values: map[string]string{
				"non-existent": "{{ non-existent }}",
			},
			params: map[string]any{},
			expected: map[string]any{
				"values.non-existent": "{{ non-existent }}",
			},
		},
		{
			name: "Billion laughs",
			values: map[string]string{
				"lol1": "lol",
				"lol2": "{{values.lol1}}{{values.lol1}}",
				"lol3": "{{values.lol2}}{{values.lol2}}{{values.lol2}}",
			},
			params: map[string]any{},
			expected: map[string]any{
				"values.lol1": "lol",
				"values.lol2": "{{values.lol1}}{{values.lol1}}",
				"values.lol3": "{{values.lol2}}{{values.lol2}}{{values.lol2}}",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := appendTemplatedValues(testCase.values, testCase.params, false, nil)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, testCase.params)
		})
	}
}

func TestValueInterpolationWithGoTemplating(t *testing.T) {
	testCases := []struct {
		name     string
		values   map[string]string
		params   map[string]any
		expected map[string]any
	}{
		{
			name: "Simple interpolation",
			values: map[string]string{
				"hello": "{{ .world }}",
			},
			params: map[string]any{
				"world": "world!",
			},
			expected: map[string]any{
				"world": "world!",
				"values": map[string]string{
					"hello": "world!",
				},
			},
		},
		{
			name: "Non-existent to default",
			values: map[string]string{
				"non_existent": "{{ default \"bar\" .non_existent }}",
			},
			params: map[string]any{},
			expected: map[string]any{
				"values": map[string]string{
					"non_existent": "bar",
				},
			},
		},
		{
			name: "Billion laughs",
			values: map[string]string{
				"lol1": "lol",
				"lol2": "{{.values.lol1}}{{.values.lol1}}",
				"lol3": "{{.values.lol2}}{{.values.lol2}}{{.values.lol2}}",
			},
			params: map[string]any{},
			expected: map[string]any{
				"values": map[string]string{
					"lol1": "lol",
					"lol2": "<no value><no value>",
					"lol3": "<no value><no value><no value>",
				},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := appendTemplatedValues(testCase.values, testCase.params, true, nil)
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, testCase.params)
		})
	}
}
