package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombineStringMaps(t *testing.T) {
	testCases := []struct {
		name        string
		left        map[string]interface{}
		right       map[string]interface{}
		expected    map[string]string
		expectedErr error
	}{
		{
			name:        "combines the maps",
			left:        map[string]interface{}{"foo": "bar"},
			right:       map[string]interface{}{"a": "b"},
			expected:    map[string]string{"a": "b", "foo": "bar"},
			expectedErr: nil,
		},
		{
			name:        "fails if keys are the same but value isn't",
			left:        map[string]interface{}{"foo": "bar", "a": "fail"},
			right:       map[string]interface{}{"a": "b", "c": "d"},
			expected:    map[string]string{"a": "b", "foo": "bar"},
			expectedErr: fmt.Errorf("found duplicate key a with different value, a: fail ,b: b"),
		},
		{
			name:        "pass if keys & values are the same",
			left:        map[string]interface{}{"foo": "bar", "a": "b"},
			right:       map[string]interface{}{"a": "b", "c": "d"},
			expected:    map[string]string{"a": "b", "c": "d", "foo": "bar"},
			expectedErr: nil,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			got, err := CombineStringMaps(testCaseCopy.left, testCaseCopy.right)

			if testCaseCopy.expectedErr != nil {
				assert.EqualError(t, err, testCaseCopy.expectedErr.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, testCaseCopy.expected, got)
			}
		})
	}
}

func TestCombineMaps(t *testing.T) {
	testCases := []struct {
		name     string
		left     map[string]interface{}
		right    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "combines the maps without overlap",
			left: map[string]interface{}{
				"firstKey": map[string]interface{}{
					"foo":  "bar",
					"foo2": true,
					"foo4": map[string]interface{}{
						"foo4foo":  "barbar1",
						"foo4foo2": "barbar2",
						"foo5foo3": "barbar3",
					},
				},
				"secondKey": true,
				"thirdKey": []string{
					"firstIdx",
					"secondIdx",
					"thirdIdx",
				},
			},
			right: map[string]interface{}{
				"otherKey": "otherValue",
			},
			expected: map[string]interface{}{
				"firstKey": map[string]interface{}{
					"foo":  "bar",
					"foo2": true,
					"foo4": map[string]interface{}{
						"foo4foo":  "barbar1",
						"foo4foo2": "barbar2",
						"foo5foo3": "barbar3",
					},
				},
				"secondKey": true,
				"thirdKey": []string{
					"firstIdx",
					"secondIdx",
					"thirdIdx",
				},
				"otherKey": "otherValue",
			},
		},
		{
			name: "combines map with overlaps",
			left: map[string]interface{}{
				"firstKey": map[string]interface{}{
					"foo":  "bar",
					"foo2": "bar2",
					"foo3": true,
					"foo4": map[string]interface{}{
						"foo4foo":  "barbar1",
						"foo4foo2": "barbar2",
						"foo5foo3": "barbar3",
					},
				},
				"secondKey": true,
				"thirdKey": []string{
					"firstIdx",
					"secondIdx",
					"thirdIdx",
				},
				"myKey":      "myValue",
				"myOtherKey": "myOtherValue",
			},
			right: map[string]interface{}{
				"firstKey": map[string]interface{}{
					"foo2": "bar2Override",
					"foo3": false,
					"foo4": map[string]interface{}{
						"foo4foo2": "barbar2Override",
						"foo4foo4": "barbar4",
					},
				},
				"secondKey": false,
				"thirdKey": []string{
					"secondIdx",
					"thirdIdx",
					"fourthIdx",
				},
				"myKey":    "otherValue",
				"otherKey": "anotherValue",
			},
			expected: map[string]interface{}{
				"firstKey": map[string]interface{}{
					"foo":  "bar",
					"foo2": "bar2Override",
					"foo3": false,
					"foo4": map[string]interface{}{
						"foo4foo":  "barbar1",
						"foo4foo2": "barbar2Override",
						"foo5foo3": "barbar3",
						"foo4foo4": "barbar4",
					},
				},
				"secondKey": false,
				"thirdKey": []string{
					"secondIdx",
					"thirdIdx",
					"fourthIdx",
				},
				"myKey":      "otherValue",
				"myOtherKey": "myOtherValue",
				"otherKey":   "anotherValue",
			},
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			got := CombineMaps(testCaseCopy.left, testCaseCopy.right)

			assert.Equal(t, testCaseCopy.expected, got)
		})
	}
}
