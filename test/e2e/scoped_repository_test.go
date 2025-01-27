package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/project"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	accountFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/account"
	projectFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/project"
	repoFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/repos"
)

func TestCreateRepositoryWithProject(t *testing.T) {
	prjConsequence := projectFixture.Given(t).
		When().
		Name("argo-project").
		Create().
		Then()

	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.Given(t, true).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, err error) {
			assert.Equal(t, r.Repo, path)
			assert.Equal(t, "argo-project", r.Project)

			prjConsequence.And(func(projectResponse *project.DetailedProjectsResponse, err error) {
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
	repoFixture.Given(t, true).
		When().
		Path(path).
		Project("argo-project").
		IgnoreErrors().
		Create().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Contains(t, err.Error(), "PermissionDenied desc = permission denied: repositories, create")
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
	repoFixture.Given(t, true).
		When().
		Path(path).
		Project("argo-project").
		IgnoreErrors().
		Create().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Contains(t, err.Error(), "PermissionDenied desc = permission denied: repositories, create")
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
	repoFixture.Given(t, true).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, err error) {
			assert.Equal(t, r.Repo, path)
			assert.Equal(t, "argo-project", r.Project)
		}).
		When().
		Delete().
		Then().
		AndCLIOutput(func(output string, err error) {
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
	repoFixture.Given(t, true).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, err error) {
			assert.Equal(t, r.Repo, path)
			assert.Equal(t, "argo-project", r.Project)
		}).
		When().
		IgnoreErrors().
		Delete().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Contains(t, err.Error(), "PermissionDenied desc = permission denied: repositories, delete")
		})
}

func TestDeleteRepository(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.Given(t, false).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		And(func(r *Repository, err error) {
			assert.Equal(t, r.Repo, path)
		}).
		When().
		Delete().
		Then().
		And(func(r *Repository, err error) {
			assert.Equal(t, "repo not found", err.Error())
		})
}

func TestListRepoCLIOutput(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.Given(t, false).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, `Repository 'https://github.com/argoproj/argo-cd.git' added`, output)
		}).
		When().
		List().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, `TYPE  NAME  REPO                                     INSECURE  OCI    LFS    CREDS  STATUS      MESSAGE  PROJECT
git         https://github.com/argoproj/argo-cd.git  false     false  false  false  Successful           argo-project`, output)
		})
}

func TestGetRepoCLIOutput(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	repoFixture.Given(t, false).
		When().
		Path(path).
		Project("argo-project").
		Create().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, `Repository 'https://github.com/argoproj/argo-cd.git' added`, output)
		}).
		When().
		Get().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, `TYPE  NAME  REPO                                     INSECURE  OCI    LFS    CREDS  STATUS      MESSAGE  PROJECT
git         https://github.com/argoproj/argo-cd.git  false     false  false  false  Successful           argo-project`, output)
		})
}
