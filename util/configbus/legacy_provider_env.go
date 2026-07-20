package configbus

import (
	"math"
	"time"

	"github.com/argoproj/argo-cd/v3/util/env"
)

func (p *LegacyProvider) GitRequestTimeout() (time.Duration, error) {
	return env.ParseDurationFromEnv("ARGOCD_GIT_REQUEST_TIMEOUT", 15*time.Second, 0, math.MaxInt64), nil
}
