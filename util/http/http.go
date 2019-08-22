package http

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"strings"

	log "github.com/sirupsen/logrus"
)

// MakeCookieMetadata generates a string representing a Web cookie.  Yum!
func MakeCookieMetadata(key, value string, flags ...string) (string, error) {
	components := []string{
		fmt.Sprintf("%s=%s", key, value),
	}
	components = append(components, flags...)
	header := strings.Join(components, "; ")

	const maxLength = 4093
	headerLength := len(header)
	if headerLength > maxLength {
		return "", fmt.Errorf("invalid cookie, at %d long it is longer than the max length of %d", headerLength, maxLength)
	}
	return header, nil
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
