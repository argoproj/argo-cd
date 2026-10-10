//go:build !linux

package reaper

import "context"

// StartIfPID1 is a no-op on non-Linux platforms: the CMP sidecar (the only
// place argocd-cmp-server runs as PID 1) is Linux-only.
func StartIfPID1(_ context.Context) bool {
	return false
}
