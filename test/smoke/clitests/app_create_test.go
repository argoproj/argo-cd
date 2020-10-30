package clitests

import (
	"testing"

	"github.com/argoproj/argo-cd/test/smoke/helper"
	"github.com/argoproj/argo-cd/util/errors"
	"gotest.tools/assert"
)

func TestCreateApp(t *testing.T) {

	output, err := helper.RunCmd("argocd", "app", "create", "guestbook", "--repo", "https://github.com/argoproj/argocd-example-apps.git", "--path", "guestbook", "--dest-server", "https://kubernetes.default.svc", "--dest-namespace", "default")
	errors.CheckError(err)

	assert.Equal(t, "application 'guestbook' created", output)
}
