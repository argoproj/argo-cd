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
	assert.EqualError(t, err, "invalid cookie, at 4094 long it is longer than the max length of 4093")
	assert.Equal(t, "", cookie)
}
