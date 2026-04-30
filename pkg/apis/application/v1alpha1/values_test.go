package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestValues_NullPreservation(t *testing.T) {
	t.Run("ValuesYAML preserves null values from JSON", func(t *testing.T) {
		source := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{
				Raw: []byte(`{"recommender":{"resources":{"limits":{"cpu":null,"memory":"64Mi"}}}}`),
			},
		}
		yamlOutput := string(source.ValuesYAML())
		assert.Contains(t, yamlOutput, "cpu: null", "null values should be preserved in YAML output")
		assert.Contains(t, yamlOutput, "memory: 64Mi")
	})

	t.Run("SetValuesString preserves null values in nested objects", func(t *testing.T) {
		source := &ApplicationSourceHelm{}
		err := source.SetValuesString(`{"limits": {"cpu": null, "memory": "64Mi"}}`)
		require.NoError(t, err)

		yamlOutput := source.ValuesString()
		assert.Contains(t, yamlOutput, "cpu: null", "null values should be preserved after SetValuesString")
		assert.Contains(t, yamlOutput, "memory: 64Mi")
	})

	t.Run("SetValuesString preserves null values from YAML input", func(t *testing.T) {
		source := &ApplicationSourceHelm{}
		yamlInput := "limits:\n  cpu: null\n  memory: 64Mi"
		err := source.SetValuesString(yamlInput)
		require.NoError(t, err)

		yamlOutput := source.ValuesString()
		assert.Contains(t, yamlOutput, "cpu: null", "null values should be preserved from YAML input")
		assert.Contains(t, yamlOutput, "memory: 64Mi")
	})

	t.Run("ValuesYAML with raw JSON containing null produces valid YAML for Helm", func(t *testing.T) {
		source := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{
				Raw: []byte(`{"podDisruptionBudget":{"enabled":true,"maxUnavailable":1,"minAvailable":null}}`),
			},
		}
		yamlOutput := string(source.ValuesYAML())
		assert.Contains(t, yamlOutput, "minAvailable: null")
		assert.Contains(t, yamlOutput, "maxUnavailable: 1")
		assert.Contains(t, yamlOutput, "enabled: true")
	})

	t.Run("ValuesIsEmpty returns false when values contain only null entries", func(t *testing.T) {
		source := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{
				Raw: []byte(`{"cpu":null}`),
			},
		}
		assert.False(t, source.ValuesIsEmpty(), "values with null entries should not be considered empty")
	})

	t.Run("round-trip JSON marshal/unmarshal preserves null values", func(t *testing.T) {
		source := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{
				Raw: []byte(`{"limits":{"cpu":null,"memory":"64Mi"}}`),
			},
		}

		data, err := source.ValuesObject.MarshalJSON()
		require.NoError(t, err)
		assert.Contains(t, string(data), "null")

		err = source.ValuesObject.UnmarshalJSON(data)
		require.NoError(t, err)

		yamlOutput := string(source.ValuesYAML())
		assert.Contains(t, yamlOutput, "cpu: null",
			"null values should survive JSON round-trip, got: %s", yamlOutput)
	})
}
