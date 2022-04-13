package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/pointer"
	"testing"
)

func TestStringOrObject_UnmarshalJSON(t *testing.T) {
	testCases := []struct{
		name        string
		inputValue  string
		expectError error
		expectValue string
		expectedJsonValue *string
	}{
		{
			name:       "null should not throw an error",
			inputValue: "null",
			expectValue: "",
			expectedJsonValue: pointer.String(`""`),
		},
		{
			name:       "an empty string should not throw an error",
			inputValue: `""`,
			expectValue: "",
		},
		{
			name:       "a string with contents should not throw an error",
			inputValue: `"hello"`,
			expectValue: "hello",
		},
		{
			name:        "an array should throw an error",
			inputValue:  "[]",
			expectError: StringOrObjectMustNotBeArrayError,
		},
		{
			name:       "an empty object should not throw an error",
			inputValue: "{}",
			expectValue: "{}\n",
		},
		{
			name:       "an object with contents should not throw an error",
			inputValue: `{"some": "inputValue"}`,
			expectValue: "some: inputValue\n",
		},
		{
			name:       "a complex object should not throw an error",
			inputValue: `{"a": {"nested": "object"}, "an": ["array"], "bool": true, "number": 1, "some": "string"}`,
			expectValue: "a:\n  nested: object\nan:\n- array\nbool: true\nnumber: 1\nsome: string\n",
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stringOrObject := &StringOrObject{}
			err := stringOrObject.UnmarshalJSON([]byte(testCaseCopy.inputValue))
			assert.ErrorIs(t, err, testCaseCopy.expectError)
			if testCaseCopy.expectError == nil {
				assert.Equal(t, testCaseCopy.expectValue, string(stringOrObject.Value()))
				marshalledJson, err := stringOrObject.MarshalJSON()
				assert.NoError(t, err)
				var expectedJsonValue = testCaseCopy.inputValue  // in most cases, output should be same as input
				if testCaseCopy.expectedJsonValue != nil {
					expectedJsonValue = *testCaseCopy.expectedJsonValue
				}
				assert.Equal(t, expectedJsonValue, string(marshalledJson))
			}
		})
	}
}

func TestStringOrObject_IsEmpty(t *testing.T) {
	testCases := []struct{
		name string
		value string
		expectIsEmpty bool
	}{
		{
			name: "null should be treated as empty",
			value: "null",
			expectIsEmpty: true,
		},
		{
			name: "an empty string should be treated as empty",
			value: `""`,
			expectIsEmpty: true,
		},
		{
			name: "an empty object should not be treated as empty",
			value: "{}",
			expectIsEmpty: false,
		},
		{
			name: "an object with contents should not be treated as empty",
			value: `{"some": "inputValue"}`,
			expectIsEmpty: false,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			stringOrObject := &StringOrObject{}
			err := stringOrObject.UnmarshalJSON([]byte(testCaseCopy.value))
			require.NoError(t, err)
			assert.Equal(t, testCaseCopy.expectIsEmpty, stringOrObject.IsEmpty())
		})
	}
}
