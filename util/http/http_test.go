package http

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCookieMaxLength(t *testing.T) {
	cookies, err := MakeCookieMetadata("foo", "bar")
	assert.NoError(t, err)
	assert.Equal(t, "foo=bar", cookies[0])

	// keys will be of format foo, foo-1, foo-2 ..
	cookies, err = MakeCookieMetadata("foo", strings.Repeat("_", (maxCookieLength-5)*maxCookieNumber))
	assert.EqualError(t, err, "invalid cookie value, at 20440 long it is longer than the max length of 20435")
	assert.Equal(t, 0, len(cookies))
}

func TestSplitCookie(t *testing.T) {
	cookieValue := strings.Repeat("_", (maxCookieLength-6)*4)
	cookies, err := MakeCookieMetadata("foo", cookieValue)
	assert.NoError(t, err)
	assert.Equal(t, 4, len(cookies))
	assert.Equal(t, 2, len(strings.Split(cookies[0], "=")))
	token := strings.Split(cookies[0], "=")[1]
	assert.Equal(t, 2, len(strings.Split(token, ":")))
	assert.Equal(t, "4", strings.Split(token, ":")[0])

	cookies = append(cookies, "bar=this-entry-should-be-filtered")
	var cookieList []*http.Cookie
	for _, cookie := range cookies {
		parts := strings.Split(cookie, "=")
		cookieList = append(cookieList, &http.Cookie{Name: parts[0], Value: parts[1]})
	}
	token, err = JoinCookies("foo", cookieList)
	assert.NoError(t, err)
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
	req, _ := http.NewRequest("GET", "/foo", nil)
	req.Header.Set("Bar", "req_1")
	req.Header.Set("Foo", "req_1")

	// No default headers.
	client.Transport = &TransportWithHeader{
		RoundTripper: &TestRoundTripper{},
	}
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.Header, http.Header{
		"Bar": []string{"req_1"},
		"Foo": []string{"req_1"},
	})

	// with default headers.
	client.Transport = &TransportWithHeader{
		RoundTripper: &TestRoundTripper{},
		Header: http.Header{
			"Foo": []string{"default_1", "default_2"},
		},
	}
	resp, err = client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, resp.Header, http.Header{
		"Bar": []string{"req_1"},
		"Foo": []string{"default_1", "default_2", "req_1"},
	})
}
