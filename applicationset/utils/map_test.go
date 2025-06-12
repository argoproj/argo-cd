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
