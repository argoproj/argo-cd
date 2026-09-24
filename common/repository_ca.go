package common

import (
	"github.com/argoproj/argo-cd/v3/util/env"
)

var mergeRepositoryCAWithSystem = env.ParseBoolFromEnv(EnvMergeRepositoryCAWithSystem, true)

// MergeRepositoryCAWithSystem reports whether repository-specific CAs are merged with the
// system trust store (default true). The repo-server sets this from its command-line flag.
func MergeRepositoryCAWithSystem() bool {
	return mergeRepositoryCAWithSystem
}

// SetMergeRepositoryCAWithSystem configures whether repository CAs are merged with system trust.
func SetMergeRepositoryCAWithSystem(enabled bool) {
	mergeRepositoryCAWithSystem = enabled
}
