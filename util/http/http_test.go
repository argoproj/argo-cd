package http

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCookieMaxLength(t *testing.T) {

	cookies, err := MakeCookieMetadata("foo", "bar")
	assert.NoError(t, err)
	assert.Equal(t, "foo-0=bar", cookies[0])

	// keys will be of format foo-0, foo-1 ..
	cookies, err = MakeCookieMetadata("foo", strings.Repeat("_", (maxLength-5)*maxNumber))
	assert.EqualError(t, err, "invalid cookie value, at 20440 long it is longer than the max length of 20435")
	assert.Equal(t, 0, len(cookies))
}

func TestSplitCookie(t *testing.T) {
	cookieValue := strings.Repeat("_", (maxLength-6)*4)
	cookies, err := MakeCookieMetadata("foo", cookieValue)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(cookies))

	token := JoinCookies("foo", strings.Join(cookies, "; "))
	assert.Equal(t, cookieValue, token)
}
