package configbus

import (
	"context"
	"math"
	"time"

	"github.com/argoproj/argo-cd/v3/util/env"
)

// EnvProvider resolves process environment variables. Unowned field getters
// return ErrNotConfigured via the embedded notConfiguredProvider.
type EnvProvider struct {
	notConfiguredProvider
}

// NewEnvProvider constructs an EnvProvider.
func NewEnvProvider() *EnvProvider {
	return &EnvProvider{}
}

// Ensure EnvProvider implements Provider.
var _ Provider = (*EnvProvider)(nil)

func (p *EnvProvider) GitRequestTimeout(_ context.Context) (time.Duration, error) {
	return env.ParseDurationFromEnv("ARGOCD_GIT_REQUEST_TIMEOUT", 15*time.Second, 0, math.MaxInt64), nil
}
