package kube

import (
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilnet "k8s.io/apimachinery/pkg/util/net"
	"k8s.io/client-go/rest"
)

type failureRetryRoundTripper struct {
	roundTripper                   http.RoundTripper
	failureRetryCount              int
	failureRetryPeriodMilliSeconds int
}

// nolint:unparam
func shouldRetry(counter int, r *http.Request, response *http.Response, err error) bool {
	if counter <= 0 {
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
		time.Sleep(time.Duration(frt.failureRetryPeriodMilliSeconds) * time.Millisecond)
		resp, roundTimeErr = frt.roundTripper.RoundTrip(r)
		counter--
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
			roundTripper:                   rt,
			failureRetryCount:              failureRetryCount,
			failureRetryPeriodMilliSeconds: failureRetryPeriodSeconds,
		}
	}
	return config
}
