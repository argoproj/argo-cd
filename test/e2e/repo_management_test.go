package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	"github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/test/e2e/fixture/repos"
	argoio "github.com/argoproj/argo-cd/util/io"
	"github.com/argoproj/argo-cd/util/settings"
)

func TestAddRemovePublicRepo(t *testing.T) {
	app.Given(t).And(func() {
		repoUrl := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoUrl)
		assert.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		assert.NoError(t, err)
		defer argoio.Close(conn)

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
	})
}

func TestUpsertExistingRepo(t *testing.T) {
	app.Given(t).And(func() {
		fixture.SetRepos(settings.RepositoryCredentials{URL: fixture.RepoURL(fixture.RepoURLTypeFile)})
		repoUrl := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoUrl)
		assert.NoError(t, err)

		_, err = fixture.RunCli("repo", "add", repoUrl, "--username", fixture.GitUsername, "--password", fixture.GitPassword)
		assert.Error(t, err)

		_, err = fixture.RunCli("repo", "add", repoUrl, "--upsert", "--username", fixture.GitUsername, "--password", fixture.GitPassword)
		assert.NoError(t, err)
	})
}

func TestAddRemoveHelmRepo(t *testing.T) {
	app.Given(t).CustomCACertAdded().And(func() {
		_, err := fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHelm),
			"--name", "testrepo",
			"--type", "helm",
			"--username", fixture.GitUsername,
			"--password", fixture.GitPassword,
			"--tls-client-cert-path", repos.CertPath,
			"--tls-client-cert-key-path", repos.CertKeyPath)
		assert.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		assert.NoError(t, err)
		defer argoio.Close(conn)

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
	})

}

func TestAddHelmRepoInsecureSkipVerify(t *testing.T) {
	app.Given(t).And(func() {
		_, err := fixture.RunCli("repo", "add", fixture.RepoURL(fixture.RepoURLTypeHelm),
			"--name", "testrepo",
			"--type", "helm",
			"--username", fixture.GitUsername,
			"--password", fixture.GitPassword,
			"--insecure-skip-server-verification",
			"--tls-client-cert-path", repos.CertPath,
			"--tls-client-cert-key-path", repos.CertKeyPath)

		if !assert.NoError(t, err) {
			return
		}

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		if !assert.NoError(t, err) {
			return
		}

		defer argoio.Close(conn)

		repo, err := repoClient.List(context.Background(), &repositorypkg.RepoQuery{})

		if !assert.NoError(t, err) {
			return
		}

		exists := false
		for i := range repo.Items {
			if repo.Items[i].Repo == fixture.RepoURL(fixture.RepoURLTypeHelm) {
				exists = true
				break
			}
		}
		assert.True(t, exists)
	})

}
