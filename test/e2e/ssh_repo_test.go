package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/test/fixture/test_repos"
)

func TestCanAccessSSHRepo(t *testing.T) {
	Given(t).
		And(repos.AddHTTPSRepo).
		Repo(test_repos.SSHTestRepo.URL).
		Path("config-map").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded))
}
