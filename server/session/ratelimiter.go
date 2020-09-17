package session

import (
	"github.com/argoproj/argo-cd/util/session"

	"golang.org/x/sync/semaphore"

	util "github.com/argoproj/gitops-engine/pkg/utils/io"
	log "github.com/sirupsen/logrus"
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
