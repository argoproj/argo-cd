package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepo(t *testing.T) {

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

	repo, err := RepoFactory{}.GetRepo("https://github.com/argoproj/argocd-example-apps.git", "", "", "", false)
	assert.NoError(t, err)

	t.Run("FindApps", func(t *testing.T) {
		revision, err := repo.ResolveRevision(".", "HEAD")
		assert.NoError(t, err)
		actualAppTemplates, err := repo.FindApps(revision)
		assert.NoError(t, err)
		assert.Equal(t, appTemplates, actualAppTemplates)
	})

	t.Run("GetTemplate", func(t *testing.T) {
		for appPath := range appTemplates {
			t.Run(appPath, func(t *testing.T) {
				revision, err := repo.ResolveRevision(appPath, "HEAD")
				assert.NoError(t, err)
				actualAppPath, appType, err := repo.GetTemplate(appPath, revision)
				assert.NoError(t, err)
				assert.Equal(t, appTemplates[appPath], appType)
				assert.True(t, strings.HasSuffix(actualAppPath, appPath))
			})
		}
	})

	t.Run("GetTemplateUnresolvedRevision", func(t *testing.T) {
		_, _, err := repo.GetTemplate("", "HEAD")
		assert.EqualError(t, err, "invalid resolved revision \"HEAD\", must be resolved")
		_, _, err = repo.GetTemplate("", "")
		assert.EqualError(t, err, "invalid resolved revision \"\", must be resolved")
	})
}
