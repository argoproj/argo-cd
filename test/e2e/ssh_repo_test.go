package e2e

import (
	"testing"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestCanAccessInsecureSSHRepo(t *testing.T) {
	Given(t).
		SSHInsecureRepoURLAdded(true).
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
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Path("config-map").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded))
}
