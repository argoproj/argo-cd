package v1alpha1

import (
	"encoding/json"
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

func TestApplicationSourceHelm_NormalizeValuesObject(t *testing.T) {
	t.Run("nil receiver does not panic", func(t *testing.T) {
		var h *ApplicationSourceHelm
		assert.NotPanics(t, h.NormalizeValuesObject)
	})

	t.Run("nil ValuesObject is left untouched", func(t *testing.T) {
		h := &ApplicationSourceHelm{Values: "foo: bar"}
		h.NormalizeValuesObject()
		assert.Nil(t, h.ValuesObject)
	})

	t.Run("byte-different but semantically equal values normalize to identical bytes", func(t *testing.T) {
		// The Kubernetes API server serves '&' unescaped, while encoding/json HTML-escapes it to
		// &. Normalization must collapse both representations to the same canonical bytes so they
		// compare equal.
		escaped, err := json.Marshal(map[string]string{"foo": "&"})
		require.NoError(t, err)
		require.NotEqual(t, `{"foo":"&"}`, string(escaped)) // sanity: the inputs really do differ

		unescapedSrc := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"foo":"&"}`)}}
		escapedSrc := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: escaped}}
		unescapedSrc.NormalizeValuesObject()
		escapedSrc.NormalizeValuesObject()
		assert.Equal(t, string(escapedSrc.ValuesObject.Raw), string(unescapedSrc.ValuesObject.Raw))
	})

	t.Run("is idempotent and sorts keys", func(t *testing.T) {
		h := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"b":2,"a":1}`)}}
		h.NormalizeValuesObject()
		first := string(h.ValuesObject.Raw)
		assert.Equal(t, `{"a":1,"b":2}`, first)
		h.NormalizeValuesObject()
		assert.Equal(t, first, string(h.ValuesObject.Raw))
	})

	t.Run("invalid JSON is left untouched", func(t *testing.T) {
		h := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`not json`)}}
		h.NormalizeValuesObject()
		assert.Equal(t, `not json`, string(h.ValuesObject.Raw))
	})
}

func TestApplicationSourceHelm_String(t *testing.T) {
	t.Run("nil receiver returns nil", func(t *testing.T) {
		var h *ApplicationSourceHelm
		assert.Equal(t, "nil", h.String())
	})

	t.Run("nil ValuesObject falls through to generated-style format", func(t *testing.T) {
		h := &ApplicationSourceHelm{Values: "foo: bar"}
		s := h.String()
		assert.Contains(t, s, "&ApplicationSourceHelm{")
		assert.Contains(t, s, "Values:foo: bar")
		assert.Contains(t, s, "ValuesObject:nil")
	})

	t.Run("ValuesObject with JSON-encoded bytes is rendered as YAML not integers", func(t *testing.T) {
		h := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{Raw: []byte(`{"image":{"tag":"v1.2.3"}}`)},
		}
		s := h.String()
		assert.NotContains(t, s, "[123", "must not contain raw byte integers, got: %s", s)
		assert.Contains(t, s, "image:")
		assert.Contains(t, s, "tag: v1.2.3")
	})

	t.Run("ValuesObject with YAML-encoded bytes is rendered readably", func(t *testing.T) {
		// Some call sites construct RawExtension.Raw directly from YAML rather
		// than JSON; ValuesString() falls back to string(Raw) when JSONToYAML
		// fails, so the output should still be human-readable.
		h := &ApplicationSourceHelm{
			ValuesObject: &runtime.RawExtension{Raw: []byte("image:\n  tag: v1.2.3\n")},
		}
		s := h.String()
		assert.NotContains(t, s, "[105", "must not contain raw byte integers, got: %s", s)
		assert.Contains(t, s, "image:")
		assert.Contains(t, s, "tag: v1.2.3")
	})
}

func TestApplicationSource_String(t *testing.T) {
	t.Run("ValuesObject in Helm child renders as YAML through parent String", func(t *testing.T) {
		s := &ApplicationSource{
			RepoURL: "registry-1.docker.io",
			Chart:   "my-chart",
			Helm: &ApplicationSourceHelm{
				ValuesObject: &runtime.RawExtension{Raw: []byte(`{"replicaCount":2}`)},
			},
		}
		out := s.String()
		assert.NotContains(t, out, "[123", "must not contain raw byte integers, got: %s", out)
		assert.Contains(t, out, "replicaCount: 2")
	})

	t.Run("ValuesObject with YAML-encoded bytes also renders readably via parent", func(t *testing.T) {
		s := &ApplicationSource{
			RepoURL: "registry-1.docker.io",
			Chart:   "my-chart",
			Helm: &ApplicationSourceHelm{
				ValuesObject: &runtime.RawExtension{Raw: []byte("replicaCount: 2\nimage:\n  tag: latest\n")},
			},
		}
		out := s.String()
		assert.NotContains(t, out, "[114", "must not contain raw byte integers, got: %s", out)
		assert.Contains(t, out, "replicaCount: 2")
		assert.Contains(t, out, "tag: latest")
	})
}
