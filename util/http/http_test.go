package http

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCookieMaxLength(t *testing.T) {
	cookies, err := MakeCookieMetadata("foo", "bar")
	require.NoError(t, err)
	assert.Equal(t, "foo=bar", cookies[0])

	// keys will be of format foo, foo-1, foo-2 ..
	cookies, err = MakeCookieMetadata("foo", strings.Repeat("_", (maxCookieLength-5)*maxCookieNumber))
	require.EqualError(t, err, "the authentication token is 81760 characters long and requires 21 cookies but the max number of cookies is 20. Contact your Argo CD administrator to increase the max number of cookies")
	assert.Empty(t, cookies)
}

func TestCookieWithAttributes(t *testing.T) {
	flags := []string{"SameSite=lax", "httpOnly"}

	cookies, err := MakeCookieMetadata("foo", "bar", flags...)
	require.NoError(t, err)
	assert.Equal(t, "foo=bar; SameSite=lax; httpOnly", cookies[0])
}

func TestSplitCookie(t *testing.T) {
	cookieValue := strings.Repeat("_", (maxCookieLength-6)*4)
	cookies, err := MakeCookieMetadata("foo", cookieValue)
	require.NoError(t, err)
	assert.Len(t, cookies, 4)
	assert.Len(t, strings.Split(cookies[0], "="), 2)
	token := strings.Split(cookies[0], "=")[1]
	assert.Len(t, strings.Split(token, ":"), 2)
	assert.Equal(t, "4", strings.Split(token, ":")[0])

	cookies = append(cookies, "bar=this-entry-should-be-filtered")
	var cookieList []*http.Cookie
	for _, cookie := range cookies {
		parts := strings.Split(cookie, "=")
		cookieList = append(cookieList, &http.Cookie{Name: parts[0], Value: parts[1]})
	}
	token, err = JoinCookies("foo", cookieList)
	require.NoError(t, err)
	assert.Equal(t, cookieValue, token)
}

// TestRoundTripper just copy request headers to the resposne.
type TestRoundTripper struct{}

func (rt TestRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := http.Response{}
	resp.Header = http.Header{}
	for k, vs := range req.Header {
		for _, v := range vs {
			resp.Header.Add(k, v)
		}
	}
	return &resp, nil
}

func TestTransportWithHeader(t *testing.T) {
	client := &http.Client{}
	req, _ := http.NewRequest(http.MethodGet, "/foo", nil)
	req.Header.Set("Bar", "req_1")
	req.Header.Set("Foo", "req_1")

	// No default headers.
	client.Transport = &TransportWithHeader{
		RoundTripper: &TestRoundTripper{},
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.Header{
		"Bar": []string{"req_1"},
		"Foo": []string{"req_1"},
	}, resp.Header)

	// with default headers.
	client.Transport = &TransportWithHeader{
		RoundTripper: &TestRoundTripper{},
		Header: http.Header{
			"Foo": []string{"default_1", "default_2"},
		},
	}
	resp, err = client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.Header{
		"Bar": []string{"req_1"},
		"Foo": []string{"default_1", "default_2", "req_1"},
	}, resp.Header)
}
