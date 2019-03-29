package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepoCfg(t *testing.T) {

	t.Run("GarbageUrl", func(t *testing.T) {
		_, err := RepoCfgFactory{}.GetRepoCfg("xxx", "", "", "", false)
		assert.EqualError(t, err, "repository not found")
	})

	appTemplates := map[string]string{
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

	t.Run("FindApps", func(t *testing.T) {
		revision, err := repoCfg.ResolveRevision(".", "HEAD")
		assert.NoError(t, err)
		actualAppTemplates, err := repoCfg.FindApps(revision)
		assert.NoError(t, err)
		assert.Equal(t, appTemplates, actualAppTemplates)
	})

	t.Run("GetTemplate", func(t *testing.T) {
		for appPath := range appTemplates {
			t.Run(appPath, func(t *testing.T) {
				revision, err := repoCfg.ResolveRevision(appPath, "HEAD")
				assert.NoError(t, err)
				actualAppPath, appType, err := repoCfg.GetTemplate(appPath, revision)
				assert.NoError(t, err)
				assert.Equal(t, appTemplates[appPath], appType)
				assert.True(t, strings.HasSuffix(actualAppPath, appPath))
			})
		}
	})
}
