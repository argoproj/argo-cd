package session

import (
	util "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/session"

	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
)

func NewLoginRateLimiter(maxNumber int) func() (util.Closer, error) {
	semaphore := semaphore.NewWeighted(int64(maxNumber))
	return func() (util.Closer, error) {
		if !semaphore.TryAcquire(1) {
			log.Warnf("Exceeded number of concurrent login requests")
			return nil, session.InvalidLoginErr
		}
		return util.NewCloser(func() error {
			defer semaphore.Release(1)
			return nil
		}), nil
	}
}
