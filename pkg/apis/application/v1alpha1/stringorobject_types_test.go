package v1alpha1

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			expectError: StringOrObjectInvalidTypeError,
		},
		{
			name:        "a number should throw an error",
			inputValue:  "42",
			expectError: StringOrObjectInvalidTypeError,
		},
		{
			name:        "a boolean should throw an error",
			inputValue:  "false",
			expectError: StringOrObjectInvalidTypeError,
		},
		{
			name:       "null should throw an error",
			inputValue: "null",
			expectError: StringOrObjectInvalidTypeError,
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

			stringOrObject := NewStringOrObjectFromString("")
			err := stringOrObject.UnmarshalJSON([]byte(testCaseCopy.inputValue))
			assert.ErrorIs(t, err, testCaseCopy.expectError)
			if testCaseCopy.expectError == nil {
				assert.Equal(t, testCaseCopy.expectValue, string(stringOrObject.YAML()))
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

			stringOrObject := NewStringOrObjectFromString("")
			err := stringOrObject.UnmarshalJSON([]byte(testCaseCopy.value))
			require.NoError(t, err)
			assert.Equal(t, testCaseCopy.expectIsEmpty, stringOrObject.IsEmpty())
		})
	}
}

func TestStringOrObject_SetStringValue(t *testing.T) {
	testCases := []struct{
		name string
		value string
	}{
		{
			name: "invalid YAML should be stored",
			value: "{",
		},
		{
			name: "valid YAML should be stored",
			value: "some: yaml",
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			stringOrObject := NewStringOrObjectFromString("")
			stringOrObject.SetStringValue(testCaseCopy.value)
			assert.Equal(t, testCaseCopy.value, string(stringOrObject.YAML()))
		})
	}
}

func TestStringOrObject_SetYAMLValue(t *testing.T) {
	testCases := []struct{
		name string
		value string
		expectError bool
	}{
		{
			name: "invalid YAML should throw an error",
			value: "{",
			expectError: true,
		},
		{
			name: "valid YAML should be stored",
			value: "some: yaml\n",
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			stringOrObject, err := NewStringOrObjectFromYAML([]byte(testCaseCopy.value))
			if testCaseCopy.expectError {
				assert.Error(t, err)
			} else {
				require.NotNil(t, stringOrObject)
				assert.Equal(t, testCaseCopy.value, string(stringOrObject.YAML()))
			}
		})
	}
}
