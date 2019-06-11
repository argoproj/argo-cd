package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/test/e2e/fixture/repos"
	"github.com/argoproj/argo-cd/test/fixture/test_repos"
)

// make sure we can echo back the Git creds
func TestCustomToolWithGitCreds(t *testing.T) {
	Given(t).
		// path does not matter, we ignore it
		ConfigManagementPlugin(
			ConfigManagementPlugin{
				Name: Name(),
				Generate: Command{
					Command: []string{"sh", "-c"},
					Args:    []string{`echo "{\"kind\": \"ConfigMap\", \"apiVersion\": \"v1\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GitAskpass\": \"$GIT_ASKPASS\", \"GitUsername\": \"$GIT_USERNAME\", \"GitPassword\": \"$GIT_PASSWORD\"}}}"`},
				},
			},
		).
		// add the private repo
		And(repos.AddHTTPSRepo).
		Repo(test_repos.HTTPSTestRepo.URL).
		Path("child-base").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitAskpass}")
			assert.NoError(t, err)
			assert.Equal(t, "git-ask-pass.sh", output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitUsername}")
			assert.NoError(t, err)
			assert.Equal(t, test_repos.HTTPSTestRepo.Username, output)
		}).
		And(func(app *Application) {
			output, err := Run("", "kubectl", "-n", DeploymentNamespace(), "get", "cm", Name(), "-o", "jsonpath={.metadata.annotations.GitPassword}")
			assert.NoError(t, err)
			assert.Equal(t, test_repos.HTTPSTestRepo.Password, output)
		})
}
