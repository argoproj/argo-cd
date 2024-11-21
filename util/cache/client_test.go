package cache

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Foo string
	Bar []byte
}

func TestCache(t *testing.T) {
	c := NewInMemoryCache(time.Hour)
	var obj testStruct
	err := c.Get("key", &obj)
	assert.Equal(t, err, ErrCacheMiss)
	cacheObj := testStruct{
		Foo: "foo",
		Bar: []byte("bar"),
	}
	_ = c.Set(&Item{
		Key:    "key",
		Object: &cacheObj,
	})
	cacheObj.Foo = "baz"
	err = c.Get("key", &obj)
	require.NoError(t, err)
	assert.EqualValues(t, "foo", obj.Foo)
	assert.EqualValues(t, "bar", string(obj.Bar))

	err = c.Delete("key")
	require.NoError(t, err)
	err = c.Get("key", &obj)
	assert.Equal(t, err, ErrCacheMiss)
}
