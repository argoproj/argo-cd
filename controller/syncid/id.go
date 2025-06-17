package syncid

import (
	"fmt"
	"sync/atomic"

	"github.com/argoproj/argo-cd/v3/util/rand"
)

var globalCount uint64

// Generate generates a new ID
func Generate() (string, error) {
	prefix := atomic.AddUint64(&globalCount, 1)
	randSuffix, err := rand.String(5)
	if err != nil {
		return "", fmt.Errorf("failed to generate random suffix: %w", err)
	}
	return fmt.Sprintf("%05d-%s", prefix, randSuffix), nil
}
