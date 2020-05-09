package metrics

import (
	"net/http"
	"regexp"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/rest"
)

type failureRetryRoundTripper struct {
	roundTripper              http.RoundTripper
	failureRetryCount         int
	failureRetryPeriodSeconds int
}

var retryActionPatterns = []*regexp.Regexp{
	regexp.MustCompile("/apis/argoproj.io/.*/applications(/)?.*"),
	regexp.MustCompile("/apis/argoproj.io/.*/appprojects(/)?.*"),
}

func isInterestedInRetry(path string) bool {
	for i := range retryActionPatterns {
		if retryActionPatterns[i].MatchString(path) {
			return true
		}
	}
	return false
}

func shouldRetry(counter int, r *http.Request, response *http.Response, err error) bool {
	if counter <= 0 {
		return false
	}
	if !isInterestedInRetry(r.URL.Path) {
		return false
	}
	if err != nil {
		if apierrors.IsInternalError(err) || apierrors.IsTimeout(err) || apierrors.IsServerTimeout(err) ||
			apierrors.IsTooManyRequests(err) || utilnet.IsProbableEOF(err) || utilnet.IsConnectionReset(err) {
			return true
		}
	}
	if response != nil && (response.StatusCode == 504 || response.StatusCode == 503) {
		return true
	}

	return false
}

func (frt *failureRetryRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, roundTimeErr := frt.roundTripper.RoundTrip(r)
	counter := frt.failureRetryCount
	for shouldRetry(counter, r, resp, roundTimeErr) {
		log.Debug("failureRetryRoundTripper: ", r.URL.Path, " ", r.Method)
		resp, roundTimeErr = frt.roundTripper.RoundTrip(r)
		counter--
		time.Sleep(time.Duration(frt.failureRetryPeriodSeconds) * time.Second)
	}
	return resp, roundTimeErr
}

// AddFailureRetryWrapper adds a transport wrapper which wraps a function call around each kubernetes request
func AddFailureRetryWrapper(config *rest.Config, failureRetryCount int, failureRetryPeriodSeconds int) *rest.Config {
	wrap := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if wrap != nil {
			rt = wrap(rt)
		}
		return &failureRetryRoundTripper{
			roundTripper:              rt,
			failureRetryCount:         failureRetryCount,
			failureRetryPeriodSeconds: failureRetryPeriodSeconds,
		}
	}
	return config
}
