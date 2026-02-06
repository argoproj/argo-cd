package scm_provider

import (
	"context"
	"fmt"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	sourcecraft "github.com/aalexzy/sourcecraft-sdk"
)

type SourceCraftProvider struct {
	client           *sourcecraft.Client
	organizationSlug string
	allBranches      bool
}

var _ SCMProviderService = &SourceCraftProvider{}

func NewSourceCraftProvider(organizationSlug, token, url string, allBranches, insecure bool) (*SourceCraftProvider, error) {
	if token == "" {
		token = os.Getenv("SOURCECRAFT_TOKEN")
	}
	client, err := sourcecraft.NewClient(url, sourcecraft.SetToken(token), sourcecraft.WithHTTPClient(insecure))
	if err != nil {
		return nil, fmt.Errorf("error creating a new souorcecraft client: %w", err)
	}
	return &SourceCraftProvider{
		client:           client,
		organizationSlug: organizationSlug,
		allBranches:      allBranches,
	}, nil
}

func (g *SourceCraftProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	if !g.allBranches {
		branch, status, err := g.client.GetRepoBranch(ctx, g.organizationSlug, repo.Repository, repo.Branch)
		if status.StatusCode == http.StatusNotFound {
			return nil, fmt.Errorf("got 404 while getting default branch %q for repo %q - check your repo config: %w", repo.Branch, repo.Repository, err)
		}
		if branch == nil {
			return nil, fmt.Errorf("got nil branch while getting default branch %q for repo %q - check your repo config", repo.Branch, repo.Repository)
		}
		if err != nil {
			return nil, err
		}
		return []*Repository{
			{
				Organization: repo.Organization,
				Repository:   repo.Repository,
				Branch:       repo.Branch,
				URL:          repo.URL,
				SHA:          branch.Commit.Hash,
				Labels:       repo.Labels,
				RepositoryId: repo.RepositoryId,
			},
		}, nil
	}
	repos := []*Repository{}
	opts := sourcecraft.ListRepoBranchesOptions{}
	branchesResp, _, err := g.client.ListRepoBranches(ctx, g.organizationSlug, repo.Repository, opts)
	if err != nil {
		return nil, err
	}
	for _, branch := range branchesResp.Branches {
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			Branch:       branch.Name,
			URL:          repo.URL,
			SHA:          branch.Commit.Hash,
			Labels:       repo.Labels,
			RepositoryId: repo.RepositoryId,
		})
	}
	return repos, nil
}

func (g *SourceCraftProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	repos := []*Repository{}
	repoOpts := sourcecraft.ListOrgReposOptions{}
	reposResp, _, err := g.client.ListOrgRepos(ctx, g.organizationSlug, repoOpts)
	if err != nil {
		return nil, err
	}
	for _, repo := range reposResp.Repositories {
		if repo.CloneUrl == nil {
			log.Errorf("error repo clone url is nil '%v'", repo.Slug)
			continue
		}
		var url string
		switch cloneProtocol {
		// Default to SSH if unspecified (i.e. if "").
		case "", "ssh":
			url = repo.CloneUrl.Ssh
		case "https":
			url = repo.CloneUrl.Https
		default:
			return nil, fmt.Errorf("unknown clone protocol for SourceCraft %v", cloneProtocol)
		}
		labelOpts := sourcecraft.ListLabelsOptions{}
		labelsResp, _, err := g.client.ListRepoLabels(ctx, g.organizationSlug, repo.Slug, labelOpts)
		if err != nil {
			return nil, err
		}
		labels := []string{}
		for _, label := range labelsResp.Labels {
			labels = append(labels, label.Name)
		}
		repos = append(repos, &Repository{
			Organization: g.organizationSlug,
			Repository:   repo.Slug,
			Branch:       repo.DefaultBranch,
			URL:          url,
			Labels:       labels,
			RepositoryId: repo.Id,
		})
	}
	return repos, nil
}

func (g *SourceCraftProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	opts := sourcecraft.ListRepoFileTreeOptions{}
	treeResp, resp, err := g.client.ListRepoFileTree(ctx, repo.Organization, repo.Repository, repo.Branch, path, opts)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if len(treeResp.Trees) == 0 {
		return false, nil
	}
	return true, nil
}
