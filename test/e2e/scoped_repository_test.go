package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/project"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/account"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	projectFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/project"
	repoFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/repos"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRepositoryWithProject(t *testing.T) {
	prjConsequence := projectFixture.Given(t).
		When().
		Name("argo-project").
		Create().
		Then()

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.GivenWithSameState(t).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, _ error) {
			assert.Equal(t, r.Repo, path)
			assert.Equal(t, "argo-project", r.Project)

			prjConsequence.And(func(projectResponse *project.DetailedProjectsResponse, _ error) {
				assert.Len(t, projectResponse.Repositories, 1)
				assert.Equal(t, projectResponse.Repositories[0].Repo, path)
			})
		})
}

func TestCreateRepositoryNonAdminUserPermissionDenied(t *testing.T) {
	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login()

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.GivenWithSameState(t).
		When().
		Path(path).
		Project("argo-project").
		IgnoreErrors().
		Create().
		Then().
		AndCLIOutput(func(_ string, err error) {
			assert.ErrorContains(t, err, "PermissionDenied desc = permission denied: repositories, create")
		})
}

func TestCreateRepositoryNonAdminUserWithWrongProject(t *testing.T) {
	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "repositories",
				Action:   "*",
				Scope:    "wrong-project/*",
			},
		}, "org-admin")

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.GivenWithSameState(t).
		When().
		Path(path).
		Project("argo-project").
		IgnoreErrors().
		Create().
		Then().
		AndCLIOutput(func(_ string, err error) {
			assert.ErrorContains(t, err, "PermissionDenied desc = permission denied: repositories, create")
		})
}

func TestDeleteRepositoryRbacAllowed(t *testing.T) {
	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "repositories",
				Action:   "create",
				Scope:    "argo-project/*",
			},
			{
				Resource: "repositories",
				Action:   "delete",
				Scope:    "argo-project/*",
			},
			{
				Resource: "repositories",
				Action:   "get",
				Scope:    "argo-project/*",
			},
		}, "org-admin")

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.GivenWithSameState(t).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, _ error) {
			assert.Equal(t, r.Repo, path)
			assert.Equal(t, "argo-project", r.Project)
		}).
		When().
		Delete().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Contains(t, output, "Repository 'https://github.com/argoproj/argo-cd.git' removed")
		})
}

func TestDeleteRepositoryRbacDenied(t *testing.T) {
	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "repositories",
				Action:   "create",
				Scope:    "argo-project/*",
			},
			{
				Resource: "repositories",
				Action:   "delete",
				Scope:    "argo-pr/*",
			},
			{
				Resource: "repositories",
				Action:   "get",
				Scope:    "argo-project/*",
			},
		}, "org-admin")

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.GivenWithSameState(t).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, _ error) {
			assert.Equal(t, r.Repo, path)
			assert.Equal(t, "argo-project", r.Project)
		}).
		When().
		IgnoreErrors().
		Delete().
		Then().
		AndCLIOutput(func(_ string, err error) {
			assert.ErrorContains(t, err, "PermissionDenied desc = permission denied: repositories, delete")
		})
}

func TestDeleteRepository(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.Given(t).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, _ error) {
			assert.Equal(t, r.Repo, path)
		}).
		When().
		Delete().
		Then().
		And(func(_ *Repository, err error) {
			assert.EqualError(t, err, "repo not found")
		})
}

func TestListRepoCLIOutput(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.Given(t).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Equal(t, `Repository 'https://github.com/argoproj/argo-cd.git' added`, output)
		}).
		When().
		List().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Equal(t, `TYPE  NAME  REPO                                     INSECURE  OCI    LFS    CREDS  STATUS      MESSAGE  PROJECT
git         https://github.com/argoproj/argo-cd.git  false     false  false  false  Successful           argo-project`, output)
		})
}

func TestGetRepoCLIOutput(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.Given(t).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Equal(t, `Repository 'https://github.com/argoproj/argo-cd.git' added`, output)
		}).
		When().
		Get().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Equal(t, `TYPE  NAME  REPO                                     INSECURE  OCI    LFS    CREDS  STATUS      MESSAGE  PROJECT
git         https://github.com/argoproj/argo-cd.git  false     false  false  false  Successful           argo-project`, output)
		})
}

func TestCannotAccessProjectScopedRepoWithInvalidCreds(t *testing.T) {
	// Create project A with valid credentials
	projectFixture.Given(t).
		When().
		Name("project-a").
		Create().
		Then()

	Given(t).
		Project("project-a").
		Path(fixture.GuestbookPath).
		And(func() {
			// Add repository with valid credentials to project A
			_, err := fixture.RunCli("repo", "add",
				fixture.RepoURL(fixture.RepoURLTypeHTTPS),
				"--project", "project-a",
				"--username", fixture.GitUsername,
				"--password", fixture.GitPassword,
				"--insecure-skip-server-verification")
			require.NoError(t, err)
		}).
		When().
		CreateApp().
		Then().
		Expect(Success(""))
	// Restart server to test cache behavior
	fixture.RestartAPIServer(t)

	// Create project B explicitly
	projectFixture.Given(t).
		When().
		Name("project-b").
		Create().
		Then()

	// Create project B and try to use the same repo without credentials
	Given(t).
		Project("project-b").
		RepoURLType(fixture.RepoURLTypeHTTPS).
		Path(fixture.GuestbookPath).
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(Error("", "repository not accessible"))
}
