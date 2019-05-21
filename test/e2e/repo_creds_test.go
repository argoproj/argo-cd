package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/common"
	. "github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
)

const repoUrl = "https://gitlab.com/argo-cd-test/test-apps.git"
const accessToken = "B5sBDeoqAVUouoHkrovy"
const appPath = "child-base"

// make sure you cannot access a private repo without set-up
func TestCannotAddAppFromPrivateRepoWithOutConfig(t *testing.T) {

	fixture.EnsureCleanState()

	output, err := createApp()

	assert.Error(t, err)
	assert.Contains(t, output, "No credentials available for source repository and repository is not publicly accessible")
}

// make sure you can access a private repo, if the repo ise set-up in the CM
func TestCanAddAppFromPrivateRepoWithRepoConfig(t *testing.T) {

	fixture.EnsureCleanState()

	FailOnErr(fixture.RunCli("repo", "add", repoUrl, "--username", "blah", "--password", accessToken))
	FailOnErr(createApp())
}

// make sure you can access a private repo, if the creds are set-up in the CM
func TestCanAddAppFromPrivateRepoWithCredConfig(t *testing.T) {

	fixture.EnsureCleanState()

	secretName := fixture.Name() + "-secret"
	FailOnErr(fixture.Run("", "kubectl", "create", "secret", "generic", secretName, "--from-literal=username=", "--from-literal=password="+accessToken))
	defer func() { FailOnErr(fixture.Run("", "kubectl", "delete", "secret", secretName)) }()

	FailOnErr(fixture.Run("", "kubectl", "patch", "cm", "argocd-cm", "-p", fmt.Sprintf(`{"data": {"repository.credentials": "- passwordSecret:\n    key: password\n    name: %s\n  url: %s\n  usernameSecret:\n    key: username\n    name: %s\n"}}`, secretName, repoUrl, secretName)))

	FailOnErr(createApp())
}

func createApp() (string, error) {
	return fixture.RunCli("app", "create", fixture.Name(),
		"--repo", repoUrl,
		"--path", appPath,
		"--dest-server", KubernetesInternalAPIServerAddr,
		"--dest-namespace", fixture.DeploymentNamespace(),
	)
}
