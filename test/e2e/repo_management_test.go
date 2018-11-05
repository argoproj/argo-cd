package e2e

import (
	"testing"

	"context"

	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	"github.com/stretchr/testify/assert"
)

func TestRepoManagement(t *testing.T) {
	t.Run("TestAddRemovePublicRepo", func(t *testing.T) {
		repoUrl := "https://github.com/argoproj/argo-cd.git"
		_, err := fixture.RunCli("repo", "add", repoUrl)
		assert.Nil(t, err)
		clientSet, err := fixture.NewApiClientset()
		assert.Nil(t, err)

		conn, repoClient, err := clientSet.NewRepoClient()
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
	})
}
