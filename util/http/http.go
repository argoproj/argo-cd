package http

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httputil"
	"strings"

	log "github.com/sirupsen/logrus"
)

// max number of chunks a cookie can be broken into. To be compatible with
// widest range of browsers, we shouldn't create more than 30 cookies per domain
const maxNumber = 5
const maxLength = 4093

// MakeCookieMetadata generates a string representing a Web cookie.  Yum!
func MakeCookieMetadata(key, value string, flags ...string) ([]string, error) {
	attributes := strings.Join(flags, "; ")

	// cookie: name=value; attributes and key: key-(i) e.g. argocd.token-0
	maxValueLength := maxValueLength(key, attributes)
	numberOfCookies := int(math.Ceil(float64(len(value)) / float64(maxValueLength)))
	if numberOfCookies > maxNumber {
		return nil, fmt.Errorf("invalid cookie value, at %d long it is longer than the max length of %d", len(value), maxValueLength*maxNumber)
	}

	return splitCookie(key, value, attributes), nil
}

// browser has limit on size of cookie, currently 4kb. In order to
// support cookies longer than 4kb, we split cookie into multiple 4kb chunks.
func splitCookie(key, value, attributes string) []string {
	var cookies []string
	valueLength := len(value)

	// cookie: name=value; attributes and key: key-(i) e.g. argocd.token-0
	maxValueLength := maxValueLength(key, attributes)
	var end int
	for i, j := 0, 0; i < valueLength; i, j = i+maxValueLength, j+1 {
		end = i + maxValueLength
		if end > valueLength {
			end = valueLength
		}
		if attributes == "" {
			cookies = append(cookies, fmt.Sprintf("%s-%d=%s", key, j, value[i:end]))
		} else {
			cookies = append(cookies, fmt.Sprintf("%s-%d=%s; %s", key, j, value[i:end], attributes))
		}
	}
	return cookies
}

// JoinCookies combines chunks of cookie based on key as prefix.
// It returns cookie value as string. cookieString is of format
// key1=value1; key2=value2; key3=value3
func JoinCookies(key string, cookieString string) string {
	cookies := make(map[string]string)
	for _, cookie := range strings.Split(cookieString, ";") {
		parts := strings.Split(cookie, "=")
		if len(parts) < 2 {
			continue
		}
		cookies[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}

	var sb strings.Builder
	for i := 0; i < len(cookies); i++ {
		splitKey := fmt.Sprintf("%s-%d", key, i)
		sb.WriteString(cookies[splitKey])
	}
	return sb.String()
}

func maxValueLength(key, attributes string) int {
	if len(attributes) > 0 {
		return maxLength - (len(key) + 3) - (len(attributes) + 2)
	}
	return maxLength - (len(key) + 3)
}

// DebugTransport is a HTTP Client Transport to enable debugging
type DebugTransport struct {
	T http.RoundTripper
}

func (d DebugTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		return nil, err
	}
	log.Printf("%s", reqDump)

	resp, err := d.T.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	log.Printf("%s", respDump)
	return resp, nil
}
