package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/repos"
)

func TestCreateRepositoryWithProject(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	Given(t).
		When().
		Path(path).
		Project("pasha-project").
		Create().
		Then().
		And(func(r *Repository, err error) {
			assert.Equal(t, r.Repo, path)
			assert.Equal(t, r.Project, "pasha-project")
		})
}

func TestDeleteRepository(t *testing.T) {
	path := "https://github.com/argoproj/argo-cd.git"
	Given(t).
		When().
		Path(path).
		Project("pasha-project").
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
