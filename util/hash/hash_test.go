package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFNVa(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  uint32
	}{
		{
			name:  "empty string",
			input: "",
			want:  2166136261,
		},
		{
			name:  "ASCII string",
			input: "argo-cd",
			want:  88954688,
		},
		{
			name:  "case-sensitive input",
			input: "Argo CD",
			want:  653228367,
		},
		{
			name:  "Unicode input",
			input: "🚀",
			want:  2141686490,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FNVa(tt.input))
		})
	}
}

func TestObjectHash(t *testing.T) {
	t.Run("simple struct produces hash", func(t *testing.T) {
		type SimpleStruct struct {
			Name string
			ID   int
		}
		obj := SimpleStruct{Name: "test", ID: 42}

		hash, err := ObjectHash(obj)
		require.NoError(t, err)
		assert.NotZero(t, hash)
	})

	t.Run("same object produces same hash", func(t *testing.T) {
		type TestStruct struct {
			Field1 string
			Field2 int
		}
		obj := TestStruct{Field1: "value", Field2: 123}

		hash1, err1 := ObjectHash(obj)
		hash2, err2 := ObjectHash(obj)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different objects produce different hashes", func(t *testing.T) {
		type TestStruct struct {
			Field1 string
			Field2 int
		}
		obj1 := TestStruct{Field1: "value1", Field2: 123}
		obj2 := TestStruct{Field1: "value2", Field2: 123}

		hash1, err1 := ObjectHash(obj1)
		hash2, err2 := ObjectHash(obj2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.NotEqual(t, hash1, hash2)
	})

	t.Run("nested struct produces hash", func(t *testing.T) {
		type Inner struct {
			Value string
		}
		type Outer struct {
			Name  string
			Inner Inner
		}
		obj := Outer{Name: "outer", Inner: Inner{Value: "inner"}}

		hash, err := ObjectHash(obj)
		require.NoError(t, err)
		assert.NotZero(t, hash)
	})
}
