package scm_provider

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func strp(s string) *string {
	return &s
}

func TestFilterRepoMatch(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
			{
				Repository: "two",
			},
			{
				Repository: "three",
			},
			{
				Repository: "four",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("n|hr"),
		},
	}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "three", repos[1].Repository)
}

func TestFilterLabelMatch(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
			},
			{
				Repository: "two",
				Labels:     []string{"prod-two"},
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			LabelMatch: strp("^prod-.*$"),
		},
	}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
}

func TestFilterPathExists(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
			{
				Repository: "two",
			},
			{
				Repository: "three",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			PathsExist: []string{"two"},
		},
	}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "two", repos[0].Repository)
}

func TestFilterPathDoesntExists(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
			{
				Repository: "two",
			},
			{
				Repository: "three",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			PathsDoNotExist: []string{"two"},
		},
	}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 2)
}
func TestFilterRepoMatchBadRegexp(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("("),
		},
	}
	_, err := ListRepos(context.Background(), provider, filters, "")
	assert.NotNil(t, err)
}

func TestFilterLabelMatchBadRegexp(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			LabelMatch: strp("("),
		},
	}
	_, err := ListRepos(context.Background(), provider, filters, "")
	assert.NotNil(t, err)
}

func TestFilterBranchMatch(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Branch:     "one",
			},
			{
				Repository: "one",
				Branch:     "two",
			},
			{
				Repository: "two",
				Branch:     "one",
			},
			{
				Repository: "three",
				Branch:     "one",
			},
			{
				Repository: "three",
				Branch:     "two",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			BranchMatch: strp("w"),
		},
	}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 2)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[0].Branch)
	assert.Equal(t, "three", repos[1].Repository)
	assert.Equal(t, "two", repos[1].Branch)
}

func TestMultiFilterAnd(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
			},
			{
				Repository: "two",
				Labels:     []string{"prod-two"},
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("w"),
			PathsExist:      []string{"two"},
			LabelMatch:      strp("^prod-.*$"),
		},
	}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "two", repos[0].Repository)
}

func TestMultiFilterOr(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
				Branch:     "devel",
			},
			{
				Repository: "two",
				Labels:     []string{"prod-one", "prod-two", "staging"},
				Branch:     "feature/ABC",
			},
			{
				Repository: "two",
				Labels:     []string{"prod-one", "prod-two", "staging"},
				Branch:     "main",
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
				Branch:     "feature/XYZ",
			},
			{
				Repository: "four",
				Labels:     []string{"dev"},
				Branch:     "feature/IDK",
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{
		{
			RepositoryMatch: strp("e"),
			BranchMatch:     strp("feature/.*"),
		},
		{
			LabelMatch: strp("^prod-.*$"),
		},
	}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 4)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "two", repos[2].Repository)
	assert.Equal(t, "three", repos[3].Repository)
}

func TestNoFilters(t *testing.T) {
	provider := &MockProvider{
		Repos: []*Repository{
			{
				Repository: "one",
				Labels:     []string{"prod-one", "prod-two", "staging"},
			},
			{
				Repository: "two",
				Labels:     []string{"prod-two"},
			},
			{
				Repository: "three",
				Labels:     []string{"staging"},
			},
		},
	}
	filters := []argoprojiov1alpha1.SCMProviderGeneratorFilter{}
	repos, err := ListRepos(context.Background(), provider, filters, "")
	assert.Nil(t, err)
	assert.Len(t, repos, 3)
	assert.Equal(t, "one", repos[0].Repository)
	assert.Equal(t, "two", repos[1].Repository)
	assert.Equal(t, "three", repos[2].Repository)
}
