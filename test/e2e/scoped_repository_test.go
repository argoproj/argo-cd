package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/project"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	accountFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/account"
	projectFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/project"
	repoFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/repos"
)

func TestCreateRepositoryWithProject(t *testing.T) {
	ctx := projectFixture.Given(t).Name("argo-project")
	prjConsequence := ctx.
		When().
		Create().
		Then()

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.GivenWithSameState(ctx).
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
	ctx := accountFixture.Given(t)
	ctx.Name("test").
		When().
		Create().
		Login()

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.GivenWithSameState(ctx).
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
	ctx := accountFixture.Given(t)
	ctx.Name("test").
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
	repoFixture.GivenWithSameState(ctx).
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
	ctx := accountFixture.Given(t)
	ctx.Name("test").
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
	repoFixture.GivenWithSameState(ctx).
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
	ctx := accountFixture.Given(t)
	ctx.Name("test").
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
	repoFixture.GivenWithSameState(ctx).
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

func TestCreateRepoWithSameURLInTwoProjects(t *testing.T) {
	ctx := projectFixture.Given(t)
	ctx.Name("project-one").
		When().
		Create().
		Then()

	projectFixture.Given(t).
		Name("project-two").
		When().
		Create().
		Then()

	path := "https://github.com/argoproj/argo-cd.git"

	// Create repository in first project
	repoFixture.GivenWithSameState(ctx).
		When().
		Path(path).
		Project("project-one").
		Create().
		Then().
		And(func(r *Repository, _ error) {
			assert.Equal(t, r.Repo, path)
		})

	// Create repository with same URL in second project
	repoFixture.GivenWithSameState(ctx).
		When().
		Path(path).
		Project("project-two").
		Create().
		Then().
		And(func(r *Repository, _ error) {
			assert.Equal(t, r.Repo, path)
		})
}
