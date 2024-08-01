package apiclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseHeaders(t *testing.T) {
	t.Run("Header parsed successfully", func(t *testing.T) {
		headerString := []string{"foo:", "foo1:bar1", "foo2:bar2:bar2"}
		headers, err := parseHeaders(headerString)
		assert.NoError(t, err)
		assert.Equal(t, headers.Get("foo"), "")
		assert.Equal(t, headers.Get("foo1"), "bar1")
		assert.Equal(t, headers.Get("foo2"), "bar2:bar2")
	})

	t.Run("Header parsed error", func(t *testing.T) {
		headerString := []string{"foo"}
		_, err := parseHeaders(headerString)
		assert.ErrorContains(t, err, "additional headers must be colon(:)-separated: foo")
	})
}

func Test_parseGRPCHeaders(t *testing.T) {
	t.Run("Header parsed successfully", func(t *testing.T) {
		headerStrings := []string{"origin: https://foo.bar", "content-length: 123"}
		headers, err := parseGRPCHeaders(headerStrings)
		assert.NoError(t, err)
		assert.Equal(t, headers.Get("origin"), []string{" https://foo.bar"})
		assert.Equal(t, headers.Get("content-length"), []string{" 123"})
	})

	t.Run("Header parsed error", func(t *testing.T) {
		headerString := []string{"foo"}
		_, err := parseGRPCHeaders(headerString)
		assert.ErrorContains(t, err, "additional headers must be colon(:)-separated: foo")
	})
}
