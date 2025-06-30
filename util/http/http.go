package http

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/transport"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/env"
)

const (
	maxCookieLength = 4093

	// limit size of the resp to 512KB
	respReadLimit       = int64(524288)
	retryWaitMax        = time.Duration(10) * time.Second
	EnvRetryMax         = "ARGOCD_K8SCLIENT_RETRY_MAX"
	EnvRetryBaseBackoff = "ARGOCD_K8SCLIENT_RETRY_BASE_BACKOFF"
)

// max number of chunks a cookie can be broken into. To be compatible with
// widest range of browsers, you shouldn't create more than 30 cookies per domain
var maxCookieNumber = env.ParseNumFromEnv(common.EnvMaxCookieNumber, 20, 0, math.MaxInt)

// MakeCookieMetadata generates a string representing a Web cookie.  Yum!
func MakeCookieMetadata(key, value string, flags ...string) ([]string, error) {
	attributes := strings.Join(flags, "; ")

	// cookie: name=value; attributes and key: key-(i) e.g. argocd.token-1
	maxValueLength := maxCookieValueLength(key, attributes)
	numberOfCookies := int(math.Ceil(float64(len(value)) / float64(maxValueLength)))
	if numberOfCookies > maxCookieNumber {
		return nil, fmt.Errorf("the authentication token is %d characters long and requires %d cookies but the max number of cookies is %d. Contact your Argo CD administrator to increase the max number of cookies", len(value), numberOfCookies, maxCookieNumber)
	}

	return splitCookie(key, value, attributes), nil
}

// browser has limit on size of cookie, currently 4kb. In order to
// support cookies longer than 4kb, we split cookie into multiple 4kb chunks.
// first chunk will be of format argocd.token=<numberOfChunks>:token; attributes
func splitCookie(key, value, attributes string) []string {
	var cookies []string
	valueLength := len(value)
	// cookie: name=value; attributes and key: key-(i) e.g. argocd.token-1
	maxValueLength := maxCookieValueLength(key, attributes)
	numberOfChunks := int(math.Ceil(float64(valueLength) / float64(maxValueLength)))

	var end int
	for i, j := 0, 0; i < valueLength; i, j = i+maxValueLength, j+1 {
		end = i + maxValueLength
		if end > valueLength {
			end = valueLength
		}

		var cookie string
		if j == 0 && numberOfChunks == 1 {
			cookie = fmt.Sprintf("%s=%s", key, value[i:end])
		} else if j == 0 {
			cookie = fmt.Sprintf("%s=%d:%s", key, numberOfChunks, value[i:end])
		} else {
			cookie = fmt.Sprintf("%s-%d=%s", key, j, value[i:end])
		}
		if attributes != "" {
			cookie = fmt.Sprintf("%s; %s", cookie, attributes)
		}
		cookies = append(cookies, cookie)
	}
	return cookies
}

// JoinCookies combines chunks of cookie based on key as prefix. It returns cookie
// value as string. cookieString is of format key1=value1; key2=value2; key3=value3
// first chunk will be of format argocd.token=<numberOfChunks>:token; attributes
func JoinCookies(key string, cookieList []*http.Cookie) (string, error) {
	cookies := make(map[string]string)
	for _, cookie := range cookieList {
		if !strings.HasPrefix(cookie.Name, key) {
			continue
		}
		cookies[cookie.Name] = cookie.Value
	}

	var sb strings.Builder
	var numOfChunks int
	var err error
	var token string
	var ok bool

	if token, ok = cookies[key]; !ok {
		return "", fmt.Errorf("failed to retrieve cookie %s", key)
	}
	parts := strings.Split(token, ":")

	if len(parts) == 2 {
		if numOfChunks, err = strconv.Atoi(parts[0]); err != nil {
			return "", err
		}
		sb.WriteString(parts[1])
	} else if len(parts) == 1 {
		numOfChunks = 1
		sb.WriteString(parts[0])
	} else {
		return "", fmt.Errorf("invalid cookie for key %s", key)
	}

	for i := 1; i < numOfChunks; i++ {
		sb.WriteString(cookies[fmt.Sprintf("%s-%d", key, i)])
	}
	return sb.String(), nil
}

func maxCookieValueLength(key, attributes string) int {
	if len(attributes) > 0 {
		return maxCookieLength - (len(key) + 3) - (len(attributes) + 2)
	}
	return maxCookieLength - (len(key) + 3)
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

// TransportWithHeader is a HTTP Client Transport with default headers.
type TransportWithHeader struct {
	RoundTripper http.RoundTripper
	Header       http.Header
}

func (rt *TransportWithHeader) RoundTrip(r *http.Request) (*http.Response, error) {
	if rt.Header != nil {
		headers := rt.Header.Clone()
		for k, vs := range r.Header {
			for _, v := range vs {
				headers.Add(k, v)
			}
		}
		r.Header = headers
	}
	return rt.RoundTripper.RoundTrip(r)
}

func WithRetry(maxRetries int64, baseRetryBackoff time.Duration) transport.WrapperFunc {
	return func(rt http.RoundTripper) http.RoundTripper {
		return &retryTransport{
			inner:      rt,
			maxRetries: maxRetries,
			backoff:    baseRetryBackoff,
		}
	}
}

type retryTransport struct {
	inner      http.RoundTripper
	maxRetries int64
	backoff    time.Duration
}

func isRetriable(resp *http.Response) bool {
	if resp == nil {
		return false
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		return true
	}
	return false
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	backoff := t.backoff
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
	}
	for i := 0; i <= int(t.maxRetries); i++ {
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		resp, err = t.inner.RoundTrip(req)
		if i < int(t.maxRetries) && (err != nil || isRetriable(resp)) {
			if resp != nil && resp.Body != nil {
				drainBody(resp.Body)
			}
			if backoff > retryWaitMax {
				backoff = retryWaitMax
			}
			select {
			case <-time.After(backoff):
			case <-req.Context().Done():
				return nil, req.Context().Err()
			}
			backoff *= 2
			continue
		}
		break
	}
	return resp, err
}

func drainBody(body io.ReadCloser) {
	defer body.Close()
	_, err := io.Copy(io.Discard, io.LimitReader(body, respReadLimit))
	if err != nil {
		log.Warnf("error reading response body: %s", err.Error())
	}
}
