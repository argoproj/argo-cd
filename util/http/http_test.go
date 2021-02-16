package http

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCookieMaxLength(t *testing.T) {

	cookies, err := MakeCookieMetadata("foo", "bar")
	assert.NoError(t, err)
	assert.Equal(t, "foo=bar", cookies[0])

	// keys will be of format foo, foo-1, foo-2 ..
	cookies, err = MakeCookieMetadata("foo", strings.Repeat("_", (maxLength-5)*maxNumber))
	assert.EqualError(t, err, "invalid cookie value, at 20440 long it is longer than the max length of 20435")
	assert.Equal(t, 0, len(cookies))
}

func TestSplitCookie(t *testing.T) {
	cookieValue := strings.Repeat("_", (maxLength-6)*4)
	cookies, err := MakeCookieMetadata("foo", cookieValue)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(cookies))
	assert.Equal(t, 2, len(strings.Split(cookies[0], "=")))
	token := strings.Split(cookies[0], "=")[1]
	assert.Equal(t, 2, len(strings.Split(token, ":")))
	assert.Equal(t, "4", strings.Split(token, ":")[0])

	cookies = append(cookies, "bar=this-entry-should-be-filtered")
	token, err = JoinCookies("foo", cookies)
	assert.NoError(t, err)
	assert.Equal(t, cookieValue, token)
}
