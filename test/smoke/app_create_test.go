package smoke

import (
	"testing"

	"github.com/argoproj/argo-cd/util/errors"
	"gotest.tools/assert"
)

func TestCreateApp(t *testing.T) {
	expected := "application 'guestbook' created\n"
	output, err := RunCmd("argocd", "app", "create", "guestbook", "--repo", "https://github.com/argoproj/argocd-example-apps.git", "--path", "guestbook", "--dest-server", "https://kubernetes.default.svc", "--dest-namespace", "default")
	errors.CheckError(err)
	assert.Equal(t, expected, string(output))
}
