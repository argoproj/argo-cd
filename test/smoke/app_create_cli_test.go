package smoke

import (
	"testing"

	. "github.com/argoproj/argo-cd/test/e2e/fixture"

	"github.com/argoproj/pkg/errors"
	"gotest.tools/assert"
)

func TestCreateApp(t *testing.T) {

	// Instructions on testing this locally:
	// argocd account update-password --current-password <current argocd password> --new-password password
	// This is to let RunCli work as expected per https://github.com/argoproj/argo-cd/blob/master/test/e2e/fixture/fixture.go
	// Also, update the argocd binary location on RunCliWithStdin function under test/e2e/fixture/fixture.go
	output, err := RunCli("app", "create", "guestbook", "--repo", "https://github.com/argoproj/argocd-example-apps.git", "--path", "guestbook", "--dest-server", "https://kubernetes.default.svc", "--dest-namespace", "default")
	errors.CheckError(err)

	assert.Equal(t, "application 'guestbook' created", output)

}
