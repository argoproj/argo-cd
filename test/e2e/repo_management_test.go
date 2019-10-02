package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/util"
)

func TestAddRemovePublicRepo(t *testing.T) {
	repoUrl := "https://github.com/argoproj/argocd-example-apps.git"
	_, err := fixture.RunCli("repo", "add", repoUrl)
	assert.NoError(t, err)

	conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
	assert.NoError(t, err)
	defer util.Close(conn)

	repo, err := repoClient.List(context.Background(), &repositorypkg.RepoQuery{})

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
	assert.NoError(t, err)

	repo, err = repoClient.List(context.Background(), &repositorypkg.RepoQuery{})
	assert.NoError(t, err)
	exists = false
	for i := range repo.Items {
		if repo.Items[i].Repo == repoUrl {
			exists = true
			break
		}
	}
	assert.False(t, exists)
}

func TestAddRemoveHelmRepo(t *testing.T) {
	_, err := fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHelm), "--name", "testrepo", "--type", "helm")
	assert.NoError(t, err)

	conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
	assert.NoError(t, err)
	defer util.Close(conn)

	repo, err := repoClient.List(context.Background(), &repositorypkg.RepoQuery{})

	assert.NoError(t, err)
	exists := false
	for i := range repo.Items {
		if repo.Items[i].Repo == fixture.RepoURL(fixture.RepoURLTypeHelm) {
			exists = true
			break
		}
	}
	assert.True(t, exists)

	_, err = fixture.RunCli("repo", "rm", fixture.RepoURL(fixture.RepoURLTypeHelm))
	assert.NoError(t, err)

	repo, err = repoClient.List(context.Background(), &repositorypkg.RepoQuery{})
	assert.NoError(t, err)
	exists = false
	for i := range repo.Items {
		if repo.Items[i].Repo == fixture.RepoURL(fixture.RepoURLTypeHelm) {
			exists = true
			break
		}
	}
	assert.False(t, exists)
}
