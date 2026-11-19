package scm_provider

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"

	"github.com/argoproj/argo-cd/v3/applicationset/services/internal/scmmanager"

	scmm "github.com/scm-manager/goscm"
)

type ScmManagerProvider struct {
	client      *scmmanager.Client
	allBranches bool
}

const FilterLimit = 9999

var _ SCMProviderService = &ScmManagerProvider{}

func NewScmManagerProvider(_ context.Context, token, apiURL string, allBranches, insecure bool, scmRootCAPath string, caCerts []byte) (*ScmManagerProvider, error) {
	return &ScmManagerProvider{
		client:      scmmanager.NewClient(apiURL, token, insecure, scmRootCAPath, caCerts),
		allBranches: allBranches,
	}, nil
}

func (g *ScmManagerProvider) listBranches(ctx context.Context, namespace, name string) ([]scmm.Branch, error) {
	var branches scmm.BranchContainer
	path := scmmanager.RepositoryPath(namespace, name) + "/branches/"
	if err := g.client.GetJSON(ctx, path, nil, &branches); err != nil {
		return nil, err
	}
	return branches.Embedded.Branches, nil
}

func (g *ScmManagerProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	branches, err := g.listBranches(ctx, repo.Organization, repo.Repository)
	if err != nil {
		return nil, err
	}
	repos := make([]*Repository, 0, len(branches))
	for _, branch := range branches {
		if !g.allBranches && !branch.DefaultBranch {
			continue
		}
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			Branch:       branch.Name,
			URL:          repo.URL,
			SHA:          branch.Revision,
			Labels:       []string{},
			RepositoryId: repo.Organization + "/" + repo.Repository,
		})
		if !g.allBranches {
			break
		}
	}
	if !g.allBranches && len(repos) == 0 {
		if len(branches) == 0 {
			return nil, scmm.ErrEmptyRepository
		}
		return nil, scmm.ErrNoDefaultBranchFound
	}
	return repos, nil
}

func (g *ScmManagerProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	repos := []*Repository{}
	query := url.Values{"pageSize": {strconv.Itoa(FilterLimit)}, "archived": {"true"}}
	scmmRepos, err := scmmanager.ListPages(ctx, g.client, "/api/v2/repositories", query, func(page scmm.RepoContainer) ([]scmm.Repository, int, int) {
		return page.Embedded.Repositories, page.Page, page.PageTotal
	})
	if err != nil {
		return nil, err
	}
	for _, scmmRepo := range scmmRepos {
		var url string
		switch cloneProtocol {
		// Default to SSH if unspecified (i.e. if ""). SSH Plugin needs to be installed
		case "", "ssh":
			url = getProtocolUrlByName(scmmRepo.Links.ProtocolUrl, "ssh")
		case "https":
			url = getProtocolUrlByName(scmmRepo.Links.ProtocolUrl, "http")
		default:
			return nil, fmt.Errorf("unknown clone protocol %v", cloneProtocol)
		}

		if url == "" {
			return nil, errors.New("could not find valid repository protocol url")
		}

		branches, err := g.listBranches(ctx, scmmRepo.Namespace, scmmRepo.Name)
		if err != nil {
			return nil, err
		}
		for _, branch := range branches {
			if branch.DefaultBranch {
				repos = append(repos, &Repository{
					Organization: scmmRepo.Namespace,
					Repository:   scmmRepo.Name,
					Branch:       branch.Name,
					URL:          url,
					RepositoryId: scmmRepo.Namespace + "/" + scmmRepo.Name,
				})
				break
			}
		}
	}
	return repos, nil
}

func getProtocolUrlByName(urls []scmm.ProtocolUrl, name string) string {
	for _, url := range urls {
		if url.Name == name {
			return url.Href
		}
	}
	return ""
}

func (g *ScmManagerProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	endpoint := scmmanager.RepositoryPath(repo.Organization, repo.Repository) + "/content/" + url.PathEscape(repo.Branch) + "/" + url.PathEscape(path)
	return g.client.HasContent(ctx, endpoint)
}
