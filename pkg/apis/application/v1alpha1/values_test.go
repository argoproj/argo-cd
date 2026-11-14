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

func TestApplicationSourceHelm_Equals(t *testing.T) {
	t.Run("nil == nil", func(t *testing.T) {
		var a, b *ApplicationSourceHelm
		assert.True(t, a.Equals(b))
	})

	t.Run("nil != non-nil", func(t *testing.T) {
		var a *ApplicationSourceHelm
		b := &ApplicationSourceHelm{}
		assert.False(t, a.Equals(b))
		assert.False(t, b.Equals(a))
	})

	t.Run("identical structs are equal", func(t *testing.T) {
		h := &ApplicationSourceHelm{
			ReleaseName:  "my-release",
			ValuesObject: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)},
		}
		assert.True(t, h.Equals(h.DeepCopy()))
	})

	t.Run("HTML-escaped and unescaped ValuesObject are equal", func(t *testing.T) {
		// The Kubernetes API server serves '&' unescaped, while encoding/json HTML-escapes it
		// to &. Equals must treat these as identical.
		escaped, err := json.Marshal(map[string]string{"foo": "&"})
		require.NoError(t, err)
		require.NotEqual(t, `{"foo":"&"}`, string(escaped)) // sanity: inputs really differ

		a := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"foo":"&"}`)}}
		b := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: escaped}}
		assert.True(t, a.Equals(b))
		assert.True(t, b.Equals(a))
	})

	t.Run("different key ordering in ValuesObject is equal", func(t *testing.T) {
		a := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"a":1,"b":2}`)}}
		b := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"b":2,"a":1}`)}}
		assert.True(t, a.Equals(b))
	})

	t.Run("genuinely different ValuesObject are not equal", func(t *testing.T) {
		a := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"foo":"&"}`)}}
		b := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"foo":"bar"}`)}}
		assert.False(t, a.Equals(b))
	})

	t.Run("invalid JSON falls back to byte comparison", func(t *testing.T) {
		raw := []byte(`not json`)
		a := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: raw}}
		b := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: raw}}
		assert.True(t, a.Equals(b))
	})

	t.Run("does not mutate input", func(t *testing.T) {
		escaped, err := json.Marshal(map[string]string{"foo": "&"})
		require.NoError(t, err)
		a := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: []byte(`{"foo":"&"}`)}}
		b := &ApplicationSourceHelm{ValuesObject: &runtime.RawExtension{Raw: escaped}}
		origA := string(a.ValuesObject.Raw)
		origB := string(b.ValuesObject.Raw)
		a.Equals(b)
		assert.Equal(t, origA, string(a.ValuesObject.Raw))
		assert.Equal(t, origB, string(b.ValuesObject.Raw))
	})

	t.Run("non-ValuesObject fields are compared", func(t *testing.T) {
		a := &ApplicationSourceHelm{ReleaseName: "release-a"}
		b := &ApplicationSourceHelm{ReleaseName: "release-b"}
		assert.False(t, a.Equals(b))
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
