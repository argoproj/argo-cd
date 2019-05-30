package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	v1 "k8s.io/api/core/v1"
)

func TestHelmHooksAreNotCreated(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"helm.sh/hook": "pre-install"}}]`).
		Create().
		Sync().
		Then().
		Expect(NotPod(func(p v1.Pod) bool {
			return p.Name == "hook"
		}))
}

func TestHelmCrdInstallIsCreated(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"helm.sh/hook": "crd-install"}}]`).
		Create().
		Sync().
		Then().
		Expect(Pod(func(p v1.Pod) bool {
			return p.Name == "hook"
		}))
}
