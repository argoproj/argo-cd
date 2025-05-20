package e2e

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositorypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture/repos"
	. "github.com/argoproj/argo-cd/v2/util/errors"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestAddRemovePublicRepo(t *testing.T) {
	app.Given(t).And(func() {
		repoUrl := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoUrl)
		require.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)
		defer argoio.Close(conn)

		repo, err := repoClient.ListRepositories(context.Background(), &repositorypkg.RepoQuery{})

		require.NoError(t, err)
		exists := false
		for i := range repo.Items {
			if repo.Items[i].Repo == repoUrl {
				exists = true
				break
			}
		}
		assert.True(t, exists)

		_, err = fixture.RunCli("repo", "rm", repoUrl)
		require.NoError(t, err)

		repo, err = repoClient.ListRepositories(context.Background(), &repositorypkg.RepoQuery{})
		require.NoError(t, err)
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

func TestGetRepoWithInheritedCreds(t *testing.T) {
	app.Given(t).And(func() {
		// create repo credentials
		FailOnErr(fixture.RunCli("repocreds", "add", fixture.RepoURL(fixture.RepoURLTypeHTTPSOrg), "--github-app-id", fixture.GithubAppID, "--github-app-installation-id", fixture.GithubAppInstallationID, "--github-app-private-key-path", repos.CertKeyPath))

		repoUrl := fixture.RepoURL(fixture.RepoURLTypeHTTPS)

		// Hack: First we need to create repo with valid credentials
		FailOnErr(fixture.RunCli("repo", "add", repoUrl, "--username", fixture.GitUsername, "--password", fixture.GitPassword, "--insecure-skip-server-verification"))

		// Then, we remove username/password so that the repo inherits the credentials from our repocreds
		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)
		defer argoio.Close(conn)

		_, err = repoClient.UpdateRepository(context.Background(), &repositorypkg.RepoUpdateRequest{
			Repo: &v1alpha1.Repository{
				Repo: repoUrl,
			},
		})
		require.NoError(t, err)

		// CLI output should indicate that repo has inherited credentials
		out, err := fixture.RunCli("repo", "get", repoUrl)
		require.NoError(t, err)
		assert.Contains(t, out, "inherited")

		_, err = fixture.RunCli("repo", "rm", repoUrl)
		require.NoError(t, err)
	})
}

func TestUpsertExistingRepo(t *testing.T) {
	app.Given(t).And(func() {
		fixture.SetRepos(settings.RepositoryCredentials{URL: fixture.RepoURL(fixture.RepoURLTypeFile)})
		repoUrl := fixture.RepoURL(fixture.RepoURLTypeFile)
		_, err := fixture.RunCli("repo", "add", repoUrl)
		require.NoError(t, err)

		_, err = fixture.RunCli("repo", "add", repoUrl, "--username", fixture.GitUsername, "--password", fixture.GitPassword)
		require.Error(t, err)

		_, err = fixture.RunCli("repo", "add", repoUrl, "--upsert", "--username", fixture.GitUsername, "--password", fixture.GitPassword)
		require.NoError(t, err)
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
		require.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)
		defer argoio.Close(conn)

		repo, err := repoClient.ListRepositories(context.Background(), &repositorypkg.RepoQuery{})

		require.NoError(t, err)
		exists := false
		for i := range repo.Items {
			if repo.Items[i].Repo == fixture.RepoURL(fixture.RepoURLTypeHelm) {
				exists = true
				break
			}
		}
		assert.True(t, exists)

		_, err = fixture.RunCli("repo", "rm", fixture.RepoURL(fixture.RepoURLTypeHelm))
		require.NoError(t, err)

		repo, err = repoClient.ListRepositories(context.Background(), &repositorypkg.RepoQuery{})
		require.NoError(t, err)
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

		require.NoError(t, err)

		conn, repoClient, err := fixture.ArgoCDClientset.NewRepoClient()
		require.NoError(t, err)

		defer argoio.Close(conn)

		repo, err := repoClient.ListRepositories(context.Background(), &repositorypkg.RepoQuery{})

		require.NoError(t, err)

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
