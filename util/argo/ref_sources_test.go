package argo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/stretchr/testify/assert"
)

func Test_GetRefSources(t *testing.T) {
	repoPath, err := filepath.Abs("./../..")
	assert.NoError(t, err)

	getMultiSourceAppSpec := func(sources argoappv1.ApplicationSources) *argoappv1.ApplicationSpec {
		return &argoappv1.ApplicationSpec{
			Sources: sources,
		}
	}

	repo := &argoappv1.Repository{Repo: fmt.Sprintf("file://%s", repoPath)}

	t.Run("target ref exists", func(t *testing.T) {
		repoDB := &dbmocks.ArgoDB{}
		repoDB.On("GetRepository", context.Background(), repo.Repo).Return(repo, nil)

		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: fmt.Sprintf("file://%s", repoPath), Ref: "source-1_2"},
			{RepoURL: fmt.Sprintf("file://%s", repoPath)},
		})

		refSources, err := GetRefSources(context.Background(), GetRefSourcesOptions{
			Sources: argoSpec.Sources,
			Db:      repoDB,
		})

		expectedRefSource := argoappv1.RefTargetRevisionMapping{
			"$source-1_2": &argoappv1.RefTarget{
				Repo: *repo,
			},
		}
		assert.NoError(t, err)
		assert.Len(t, refSources, 1)
		assert.Equal(t, expectedRefSource, refSources)
	})

	t.Run("target ref does not exist", func(t *testing.T) {
		repoDB := &dbmocks.ArgoDB{}
		repoDB.On("GetRepository", context.Background(), "file://does-not-exist").Return(nil, errors.New("repo does not exist"))

		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: "file://does-not-exist", Ref: "source1"},
		})

		refSources, err := GetRefSources(context.Background(), GetRefSourcesOptions{
			Sources: argoSpec.Sources,
			Db:      repoDB,
		})

		assert.Error(t, err)
		assert.Empty(t, refSources)
	})

	t.Run("invalid ref", func(t *testing.T) {
		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: "file://does-not-exist", Ref: "%invalid-name%"},
		})

		refSources, err := GetRefSources(context.TODO(), GetRefSourcesOptions{
			Sources: argoSpec.Sources,
			Db:      &dbmocks.ArgoDB{},
		})
		assert.Error(t, err)
		assert.Empty(t, refSources)
	})

	t.Run("duplicate ref keys", func(t *testing.T) {
		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: "file://does-not-exist", Ref: "source1"},
			{RepoURL: "file://does-not-exist", Ref: "source1"},
		})

		refSources, err := GetRefSources(context.TODO(), GetRefSourcesOptions{
			Sources: argoSpec.Sources,
			Db:      &dbmocks.ArgoDB{},
		})

		assert.Error(t, err)
		assert.Empty(t, refSources)
	})
}
