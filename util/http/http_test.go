package http

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCookieMaxLength(t *testing.T) {

	cookie, err := MakeCookieMetadata("foo", "bar")
	assert.NoError(t, err)
	assert.Equal(t, "foo=bar", cookie)

	cookie, err = MakeCookieMetadata("foo", strings.Repeat("_", 4093-3))
	assert.Error(t, err)
	assert.Equal(t, "", cookie)
}
