package configbus

import (
	"math"
	"time"

	"github.com/argoproj/argo-cd/v3/util/env"
)

// GitRequestTimeout returns ARGOCD_GIT_REQUEST_TIMEOUT (default 15s).
func (p *Provider) GitRequestTimeout() time.Duration {
	return env.ParseDurationFromEnv("ARGOCD_GIT_REQUEST_TIMEOUT", 15*time.Second, 0, math.MaxInt64)
}
