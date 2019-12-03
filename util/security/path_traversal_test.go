package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnforceToCurrentRoot(t *testing.T) {
	cleanDir, err := EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/values.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "/home/argo/helmapp/values.yaml", cleanDir)

	// File is outside current working directory
	_, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/values.yaml")
	assert.Error(t, err)

	// File is outside current working directory
	_, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/../differentapp/values.yaml")
	assert.Error(t, err)

	// Goes back and forth, but still legal
	cleanDir, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/../../argo/helmapp/values.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "/home/argo/helmapp/values.yaml", cleanDir)
}

func TestSubtractRelativeFromAbsolutePath(t *testing.T) {
	for _, test := range []string{"env", "/env", "env/", "/env/", "./env"} {
		subtracted, err := SubtractRelativeFromAbsolutePath("/argocd-example-apps/helm-guestbook/env/guestbook/env", test)
		assert.NoError(t, err)
		assert.Equal(t, "/argocd-example-apps/helm-guestbook/env/guestbook", subtracted)
	}
	for _, test := range []string{"guestbook/env", "/guestbook/env", "guestbook/env/", "/guestbook/env/", "./guestbook/env"} {
		subtracted, err := SubtractRelativeFromAbsolutePath("/argocd-example-apps/helm-guestbook/env/guestbook/env", test)
		assert.NoError(t, err)
		assert.Equal(t, "/argocd-example-apps/helm-guestbook/env", subtracted)
	}
	for _, test := range []string{"", ".", "/", "./"} {
		subtracted, err := SubtractRelativeFromAbsolutePath("/argocd-example-apps/helm-guestbook/env/guestbook/env", test)
		assert.NoError(t, err)
		assert.Equal(t, "/argocd-example-apps/helm-guestbook/env/guestbook/env", subtracted)
	}
	// "Dirty" strings
	for _, test := range []string{"guestbook/foo/../env", "/guestbook//env", "../guestbook/env/", "/../guestbook/env/", "./guestbook/env///"} {
		subtracted, err := SubtractRelativeFromAbsolutePath("/argocd-example-apps/helm-guestbook/env/guestbook/env", test)
		assert.NoError(t, err)
		assert.Equal(t, "/argocd-example-apps/helm-guestbook/env", subtracted)
	}
	// Invalid strings
	for _, test := range []string{"/not/in/path", "../not/in/path", "not/in/path"} {
		_, err := SubtractRelativeFromAbsolutePath("/argocd-example-apps/helm-guestbook/env/guestbook/env", test)
		assert.Error(t, err)
	}
}
