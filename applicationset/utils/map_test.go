package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCombineStringMaps(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		left        map[string]any
		right       map[string]any
		expected    map[string]any
		expectedErr error
	}{
		{
			name:        "combines the maps",
			left:        map[string]any{"foo": "bar"},
			right:       map[string]any{"a": "b"},
			expected:    map[string]any{"a": "b", "foo": "bar"},
			expectedErr: nil,
		},
		{
			name:        "fails if keys are the same but value isn't",
			left:        map[string]any{"foo": "bar", "a": "fail"},
			right:       map[string]any{"a": "b", "c": "d"},
			expected:    map[string]any{"a": "b", "foo": "bar"},
			expectedErr: errors.New("found duplicate key a with different value, a: fail ,b: b"),
		},
		{
			name:        "pass if keys & values are the same",
			left:        map[string]any{"foo": "bar", "a": "b"},
			right:       map[string]any{"a": "b", "c": "d"},
			expected:    map[string]any{"a": "b", "c": "d", "foo": "bar"},
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
