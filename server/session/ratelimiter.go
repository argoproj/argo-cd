package session

import (
	"sync"

	"github.com/argoproj/argo-cd/util/session"

	"golang.org/x/sync/semaphore"

	util "github.com/argoproj/gitops-engine/pkg/utils/io"
	log "github.com/sirupsen/logrus"
)

var once sync.Once

var instance *semaphore.Weighted

func ObtainLoginSemaphore(maxNumber int) *semaphore.Weighted {
	// Make a thread safe singleton to make sure we always use the same semaphore
	once.Do(func() {
		instance = semaphore.NewWeighted(int64(maxNumber))
	})
	return instance
}

func NewLoginRateLimiter(maxNumber int) func() (util.Closer, error) {
	semaphore := ObtainLoginSemaphore(maxNumber)
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
