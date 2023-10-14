package apiclient

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseHeaders(t *testing.T) {
	headerString := []string{"foo:", "foo1:bar1", "foo2:bar2:bar2"}
	headers, err := parseHeaders(headerString)
	assert.NoError(t, err)
	assert.Equal(t, headers.Get("foo"), "")
	assert.Equal(t, headers.Get("foo1"), "bar1")
	assert.Equal(t, headers.Get("foo2"), "bar2:bar2")
}
