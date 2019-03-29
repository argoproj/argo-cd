package git

import (
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/util/repos/api"

	"github.com/stretchr/testify/assert"
)

func TestRepoCfg(t *testing.T) {

	t.Run("GarbageUrl", func(t *testing.T) {
		_, err := RepoCfgFactory{}.GetRepoCfg("xxx", "", "", "", false)
		assert.EqualError(t, err, "repository not found")
	})

	appCfgs := map[api.AppPath]api.AppType{
		"blue-green":              "helm",
		"helm-dependency":         "helm",
		"helm-guestbook":          "helm",
		"ksonnet-guestbook":       "ksonnet",
		"kustomize-guestbook":     "kustomize",
		"plugins/kustomized-helm": "helm",
		"pre-post-sync":           "kustomize",
		"sock-shop":               "kustomize",
	}

	repoCfg, err := RepoCfgFactory{}.GetRepoCfg("https://github.com/argoproj/argocd-example-apps", "", "", "", false)
	assert.NoError(t, err)

	t.Run("FindAppCfgs", func(t *testing.T) {
		revision, err := repoCfg.ResolveRevision(".", "HEAD")
		assert.NoError(t, err)
		actualAppCfgs, err := repoCfg.FindAppCfgs(revision)
		assert.NoError(t, err)
		assert.Equal(t, appCfgs, actualAppCfgs)
	})

	t.Run("GetAppCfg", func(t *testing.T) {
		for appPath := range appCfgs {
			t.Run(appPath, func(t *testing.T) {
				revision, err := repoCfg.ResolveRevision(appPath, "HEAD")
				assert.NoError(t, err)
				actualAppPath, appType, err := repoCfg.GetAppCfg(appPath, revision)
				assert.NoError(t, err)
				assert.Equal(t, appCfgs[appPath], appType)
				assert.True(t, strings.HasSuffix(actualAppPath, appPath))
			})
		}
	})
}
