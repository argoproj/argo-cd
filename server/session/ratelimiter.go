package session

import (
	"sync"

	"github.com/argoproj/argo-cd/util/session"

	util "github.com/argoproj/gitops-engine/pkg/utils/io"
	log "github.com/sirupsen/logrus"
)

var once sync.Once

const lockKey = "login:lock"

type LoginProgressStorage struct {
	progresses map[string]int
	mux        *sync.Mutex
}

var (
	instance LoginProgressStorage
)

func (s *LoginProgressStorage) increase(key string, maxNumber int) error {
	s.mux.Lock()
	if inProgressCount := s.progresses[key] + 1; inProgressCount > maxNumber {
		log.Warnf("Exceeded number of concurrent login requests")
		s.mux.Unlock()
		return session.InvalidLoginErr
	}
	s.progresses[key]++
	s.mux.Unlock()
	return nil
}

func (s *LoginProgressStorage) decrease(key string) error {
	s.mux.Lock()
	s.progresses[key]--
	s.mux.Unlock()
	return nil
}

// NewLoginRateLimiter creates a function which enforces max number of concurrent login requests.
// Function returns closer that should be closed when logging request has completed or error if number
// of incomplete requests exceeded max number.
func NewLoginRateLimiter(storage LoginProgressStorage, maxNumber int) func() (util.Closer, error) {
	return func() (util.Closer, error) {
		err := storage.increase(lockKey, maxNumber)
		if err != nil {
			return nil, err
		}
		return util.NewCloser(func() error {
			return storage.decrease(lockKey)
		}), nil
	}
}

func ObtainStorage() LoginProgressStorage {
	// Make a thread safe singleton to make sure we always use the same storage
	once.Do(func() {
		instance = LoginProgressStorage{progresses: make(map[string]int)}
	})
	return instance
}
