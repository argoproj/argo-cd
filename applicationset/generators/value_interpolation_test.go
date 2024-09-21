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
		params   map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "Simple interpolation",
			values: map[string]string{
				"hello": "{{ world }}",
			},
			params: map[string]interface{}{
				"world": "world!",
			},
			expected: map[string]interface{}{
				"world":        "world!",
				"values.hello": "world!",
			},
		},
		{
			name: "Non-existent",
			values: map[string]string{
				"non-existent": "{{ non-existent }}",
			},
			params: map[string]interface{}{},
			expected: map[string]interface{}{
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
			params: map[string]interface{}{},
			expected: map[string]interface{}{
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
			assert.EqualValues(t, testCase.expected, testCase.params)
		})
	}
}

func TestValueInterpolationWithGoTemplating(t *testing.T) {
	testCases := []struct {
		name     string
		values   map[string]string
		params   map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "Simple interpolation",
			values: map[string]string{
				"hello": "{{ .world }}",
			},
			params: map[string]interface{}{
				"world": "world!",
			},
			expected: map[string]interface{}{
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
			params: map[string]interface{}{},
			expected: map[string]interface{}{
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
			params: map[string]interface{}{},
			expected: map[string]interface{}{
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
			assert.EqualValues(t, testCase.expected, testCase.params)
		})
	}
}
