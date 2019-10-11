package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/test/e2e/fixture"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestCanAccessInsecureSSHRepo(t *testing.T) {
	Given(t).
		SSHInsecureRepoURLAdded().
		RepoURLType(fixture.RepoURLTypeSSH).
		Path("config-map").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded))
}

func TestCanAccessSSHRepo(t *testing.T) {
	Given(t).
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded().
		RepoURLType(fixture.RepoURLTypeSSH).
		Path("config-map").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded))
}
