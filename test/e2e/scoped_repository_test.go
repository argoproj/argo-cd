package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/project"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
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
			assert.Equal(t, r.Project, "argo-project")

			prjConsequence.And(func(projectResponse *project.DetailedProjectsResponse, err error) {
				assert.Equal(t, len(projectResponse.Repositories), 1)
				assert.Equal(t, projectResponse.Repositories[0].Repo, path)
			})
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
			assert.Equal(t, err.Error(), "repo not found")
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
