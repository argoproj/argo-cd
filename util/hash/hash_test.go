package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFNVa(t *testing.T) {
	t.Run("empty string produces hash", func(t *testing.T) {
		hash := FNVa("")
		assert.NotZero(t, hash)
	})

	t.Run("same string produces same hash", func(t *testing.T) {
		hash1 := FNVa("test-string")
		hash2 := FNVa("test-string")
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different strings produce different hashes", func(t *testing.T) {
		hash1 := FNVa("string-one")
		hash2 := FNVa("string-two")
		assert.NotEqual(t, hash1, hash2)
	})
}

func TestGobObjectHash(t *testing.T) {
	t.Run("simple struct produces hash", func(t *testing.T) {
		type SimpleStruct struct {
			Name string
			ID   int
		}
		obj := SimpleStruct{Name: "test", ID: 42}

		hash, err := GobObjectHash(obj)
		require.NoError(t, err)
		assert.NotZero(t, hash)
	})

	t.Run("same object produces same hash", func(t *testing.T) {
		type TestStruct struct {
			Field1 string
			Field2 int
		}
		obj := TestStruct{Field1: "value", Field2: 123}

		hash1, err1 := GobObjectHash(obj)
		hash2, err2 := GobObjectHash(obj)

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

		hash1, err1 := GobObjectHash(obj1)
		hash2, err2 := GobObjectHash(obj2)

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

		hash, err := GobObjectHash(obj)
		require.NoError(t, err)
		assert.NotZero(t, hash)
	})
}

func TestJsonObjectHash(t *testing.T) {
	t.Run("simple struct produces hash", func(t *testing.T) {
		type SimpleStruct struct {
			Name string
			ID   int
		}
		obj := SimpleStruct{Name: "test", ID: 42}

		hash, err := JsonObjectHash(obj)
		require.NoError(t, err)
		assert.NotZero(t, hash)
	})

	t.Run("same object produces same hash", func(t *testing.T) {
		type TestStruct struct {
			Field1 string
			Field2 int
		}
		obj := TestStruct{Field1: "value", Field2: 123}

		hash1, err1 := JsonObjectHash(obj)
		hash2, err2 := JsonObjectHash(obj)

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

		hash1, err1 := JsonObjectHash(obj1)
		hash2, err2 := JsonObjectHash(obj2)

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

		hash, err := JsonObjectHash(obj)
		require.NoError(t, err)
		assert.NotZero(t, hash)
	})

	t.Run("handles maps", func(t *testing.T) {
		obj1 := map[string]any{"key": "value", "num": 42}
		obj2 := map[string]any{"key": "value", "num": 42}

		hash1, err1 := JsonObjectHash(obj1)
		hash2, err2 := JsonObjectHash(obj2)

		require.NoError(t, err1)
		require.NoError(t, err2)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("handles slices", func(t *testing.T) {
		obj1 := []string{"a", "b", "c"}
		obj2 := []string{"a", "b", "c"}
		obj3 := []string{"a", "b", "d"}

		hash1, err1 := JsonObjectHash(obj1)
		hash2, err2 := JsonObjectHash(obj2)
		hash3, err3 := JsonObjectHash(obj3)

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NoError(t, err3)
		assert.Equal(t, hash1, hash2)
		assert.NotEqual(t, hash1, hash3)
	})

	t.Run("handles nil", func(t *testing.T) {
		hash, err := JsonObjectHash(nil)
		require.NoError(t, err)
		assert.NotZero(t, hash)
	})

	t.Run("unmarshalable types return error", func(t *testing.T) {
		resultChan := make(chan int)
		_, err := JsonObjectHash(resultChan)
		require.Error(t, err)
	})
}
