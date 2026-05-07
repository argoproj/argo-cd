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

func TestApplicationSourceHelm_LogString(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var h *ApplicationSourceHelm
		assert.Equal(t, "nil", h.LogString())
	})

	t.Run("no ValuesObject falls back to String()", func(t *testing.T) {
		h := &ApplicationSourceHelm{ReleaseName: "my-release"}
		assert.Equal(t, h.String(), h.LogString())
	})

	t.Run("ValuesObject raw bytes are rendered as YAML not integers", func(t *testing.T) {
		h := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{Raw: []byte(`{"image":{"tag":"v1.2.3"}}`)},
		}
		logStr := h.LogString()
		// should not contain raw byte integers like "[123 34 ...]"
		assert.NotContains(t, logStr, "[123")
		// should contain the YAML-rendered values
		assert.Contains(t, logStr, "image:")
		// ValuesObject field should be absent in the output
		assert.NotContains(t, logStr, "ValuesObject:&runtime.RawExtension")
	})

	t.Run("ValuesObject with YAML-encoded bytes is rendered readably", func(t *testing.T) {
		// Some call sites construct RawExtension.Raw directly from YAML rather
		// than JSON; ValuesString() falls back to string(Raw) when JSONToYAML
		// fails, so the log output should still be human-readable.
		h := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{Raw: []byte("image:\n  tag: v1.2.3\n")},
		}
		logStr := h.LogString()
		assert.NotContains(t, logStr, "[105", "log output must not contain raw byte integers, got: %s", logStr)
		assert.Contains(t, logStr, "image:")
		assert.Contains(t, logStr, "tag: v1.2.3")
		assert.NotContains(t, logStr, "ValuesObject:&runtime.RawExtension")
	})
}

func TestApplicationSource_LogString(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var s *ApplicationSource
		assert.Equal(t, "nil", s.LogString())
	})

	t.Run("no Helm section falls back to String()", func(t *testing.T) {
		s := &ApplicationSource{RepoURL: "https://github.com/org/repo", Path: "charts/app"}
		assert.Equal(t, s.String(), s.LogString())
	})

	t.Run("Helm without ValuesObject falls back to String()", func(t *testing.T) {
		s := &ApplicationSource{
			RepoURL: "https://github.com/org/repo",
			Helm:    &ApplicationSourceHelm{ReleaseName: "my-app"},
		}
		assert.Equal(t, s.String(), s.LogString())
	})

	t.Run("ValuesObject raw bytes are rendered as YAML not integers", func(t *testing.T) {
		s := &ApplicationSource{
			RepoURL: "registry-1.docker.io",
			Chart:   "my-chart",
			Helm: &ApplicationSourceHelm{
				ValuesObject: &runtime.RawExtension{Raw: []byte(`{"replicaCount":2,"image":{"tag":"latest"}}`)},
			},
		}
		logStr := s.LogString()
		// must not contain raw byte integers
		assert.NotContains(t, logStr, "[123", "log output must not contain raw byte integers, got: %s", logStr)
		// must contain the YAML representation
		assert.Contains(t, logStr, "replicaCount")
		// ValuesObject binary field must not appear
		assert.NotContains(t, logStr, "ValuesObject:&runtime.RawExtension")
	})

	t.Run("ValuesObject with YAML-encoded bytes is rendered readably", func(t *testing.T) {
		// Some call sites construct RawExtension.Raw directly from YAML rather
		// than JSON; ValuesString() falls back to string(Raw) when JSONToYAML
		// fails, so the log output should still be human-readable.
		s := &ApplicationSource{
			RepoURL: "registry-1.docker.io",
			Chart:   "my-chart",
			Helm: &ApplicationSourceHelm{
				ValuesObject: &runtime.RawExtension{Raw: []byte("replicaCount: 2\nimage:\n  tag: latest\n")},
			},
		}
		logStr := s.LogString()
		assert.NotContains(t, logStr, "[114", "log output must not contain raw byte integers, got: %s", logStr)
		assert.Contains(t, logStr, "replicaCount")
		assert.Contains(t, logStr, "tag: latest")
		assert.NotContains(t, logStr, "ValuesObject:&runtime.RawExtension")
	})
}
