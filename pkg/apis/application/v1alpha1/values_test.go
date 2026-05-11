package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValues_SetString(t *testing.T) {
	testCases := []struct {
		name        string
		inputValue  string
		expectError bool
		expectValue string
	}{
		{
			name:        "an empty string should not throw an error",
			inputValue:  `""`,
			expectValue: "\"\"",
		},
		{
			name:        "a string with contents should not throw an error",
			inputValue:  `"hello"`,
			expectValue: "hello",
		},
		{
			name:        "an array should throw an error",
			inputValue:  "[]",
			expectError: true,
		},
		{
			name:        "a number should throw an error",
			inputValue:  "42",
			expectError: true,
		},
		{
			name:        "a boolean should throw an error",
			inputValue:  "false",
			expectError: true,
		},
		{
			name:        "null should throw an error",
			inputValue:  "null",
			expectError: true,
		},
		{
			name:        "an empty object should not throw an error",
			inputValue:  "{}",
			expectValue: "{}",
		},
		{
			name:        "an object with contents should not throw an error",
			inputValue:  `{"some": "inputValue"}`,
			expectValue: "some: inputValue",
		},
		{
			name:        "a complex object should not throw an error",
			inputValue:  `{"a": {"nested": "object"}, "an": ["array"], "bool": true, "number": 1, "some": "string"}`,
			expectValue: "a:\n  nested: object\nan:\n- array\nbool: true\nnumber: 1\nsome: string",
		},
	}

	for _, testCase := range testCases {
		var err error
		t.Run(testCase.name, func(t *testing.T) {
			source := &ApplicationSourceHelm{}
			err = source.SetValuesString(testCase.inputValue)

			if !testCase.expectError {
				assert.Equal(t, testCase.expectValue, source.ValuesString())
				data, err := source.ValuesObject.MarshalJSON()
				require.NoError(t, err)
				err = source.ValuesObject.UnmarshalJSON(data)
				require.NoError(t, err)
				assert.Equal(t, testCase.expectValue, source.ValuesString())
			} else {
				require.Error(t, err)
			}
		})
	}
}
