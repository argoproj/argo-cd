package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
)

func TestAddRemovePublicRepo(t *testing.T) {
	t.Run("TestAddRemovePublicRepo", func(t *testing.T) {
		repoUrl := "https://github.com/argoproj/argocd-example-apps.git"
		_, err := fixture.RunCli("repo", "add", repoUrl)
		assert.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		assert.Nil(t, err)
		defer util.Close(conn)

		repo, err := repoClient.List(context.Background(), &repository.RepoQuery{})

		assert.Nil(t, err)
		exists := false
		for i := range repo.Items {
			if repo.Items[i].Repo == repoUrl {
				exists = true
				break
			}
		}
		assert.True(t, exists)

		_, err = fixture.RunCli("repo", "rm", repoUrl)
		assert.Nil(t, err)

		repo, err = repoClient.List(context.Background(), &repository.RepoQuery{})
		assert.Nil(t, err)
		exists = false
		for i := range repo.Items {
			if repo.Items[i].Repo == repoUrl {
				exists = true
				break
			}
		}
		assert.False(t, exists)
		repo, err = repoClient.List(context.Background(), &repository.RepoQuery{})
		assert.Nil(t, err)
		exists = false
		for i := range repo.Items {
			if repo.Items[i].Repo == repoUrl {
				exists = true
				break
			}
		}
		assert.False(t, exists)
	})

	t.Run("TestAddRemoveHelmRepo", func(t *testing.T) {
		repoUrl := "https://kubernetes-charts.storage.googleapis.com"
		repoType := "helm"
		repoName := "stable"

		_, err := fixture.RunCli("repo", "add", repoUrl, "--type", repoType, "--name", repoName)
		assert.NoError(t, err)

		listing, err := fixture.RunCli("repo", "list")
		assert.NoError(t, err)

		assert.Contains(t, listing, repoUrl)
		assert.Contains(t, listing, repoName)
		assert.Contains(t, listing, repoType)

		_, err = fixture.RunCli("repo", "rm", repoUrl)
		assert.NoError(t, err)
	})
}
