package metrics

import (
	"net"
	"net/http"
	"os"
	"regexp"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"
)

type failureRetryRoundTripper struct {
	roundTripper http.RoundTripper
}

var validActionPatterns = []*regexp.Regexp{
	regexp.MustCompile("/apis/argoproj.io/.*/applications(/)?.*"),
	regexp.MustCompile("/apis/argoproj.io/.*/appprojects(/)?.*"),
}

func isInterested(path string) bool {
	for i := range validActionPatterns {
		if validActionPatterns[i].MatchString(path) {
			return true
		}
	}
	return false
}

func shouldRetry(counter int, r *http.Request, response *http.Response, err error) bool {
	if counter <= 0 {
		return false
	}
	if !isInterested(r.URL.Path) {
		return false
	}
	if err != nil {
		switch err.(type) {
		case *os.SyscallError:
			newErr, _ := err.(*os.SyscallError)
			log.Println("SyscallError")
			if newErr.Timeout() {
				return true
			}
		case *net.OpError:
			newErr, _ := err.(*net.OpError)
			log.Println("OpError")
			if newErr.Timeout() {
				return true
			}
		default:
			return false
		}
	}

	if response != nil && (response.StatusCode == 504 || response.StatusCode == 503) {
		log.Debug("failureRetryRoundTripper: retry")
		return true
	}

	//for testing purpose
	if response != nil && r.Method == "GET" {
		log.Debug("failureRetryRoundTripper: retry")
		return true
	}
	return false
}

func (mrt *failureRetryRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, roundTimeErr := mrt.roundTripper.RoundTrip(r)
	//log.Debug("failureRetryRoundTripper: ", r.URL.Path, " ", r.Method, " resp ", resp, " error ", roundTimeErr)
	counter := 0
	for shouldRetry(counter, r, resp, roundTimeErr) {
		log.Debug("failureRetryRoundTripper: ", r.URL.Path, " ", r.Method, " due to last status code ", resp.StatusCode)
		resp, roundTimeErr = mrt.roundTripper.RoundTrip(r)
		counter--
	}
	return resp, roundTimeErr
}

// AddMetricsTransportWrapper adds a transport wrapper which wraps a function call around each kubernetes request
func AddFailureRetryWrapper(config *rest.Config) *rest.Config {
	wrap := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if wrap != nil {
			rt = wrap(rt)
		}
		return &failureRetryRoundTripper{
			roundTripper: rt,
		}
	}
	return config
}
