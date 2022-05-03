package scm_provider

import (
	"context"
	"fmt"
	"net/url"
	"os"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	azureGit "github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

const AZURE_DEVOPS_DEFAULT_URL = "https://dev.azure.com"

type AzureDevopsClientFactory interface {
	// Returns an Azure Devops Client interface.
	GetClient(ctx context.Context) (azureGit.Client, error)
}

type devopsFactoryImpl struct {
	connection *azuredevops.Connection
}

func (factory *devopsFactoryImpl) GetClient(ctx context.Context) (azureGit.Client, error) {
	gitClient, err := azureGit.NewClient(ctx, factory.connection)
	if err != nil {
		return nil, err
	}
	return gitClient, nil
}

// Contains Azure Devops REST API implementation of SCMProviderService.
// See https://docs.microsoft.com/en-us/rest/api/azure/devops

type AzureDevopsProvider struct {
	organization  string
	teamProject   string
	accessToken   string
	clientFactory AzureDevopsClientFactory
}

var _ SCMProviderService = &AzureDevopsProvider{}
var _ AzureDevopsClientFactory = &devopsFactoryImpl{}

func NewAzureDevopsProvider(ctx context.Context, accessToken string, org string, project string) (*AzureDevopsProvider, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("No access token provided")
	}

	baseUrl, err := getBaseUrl()
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("%s/%s", *baseUrl, org)
	connection := azuredevops.NewPatConnection(url, accessToken)
	return &AzureDevopsProvider{organization: org, teamProject: project, accessToken: accessToken, clientFactory: &devopsFactoryImpl{connection: connection}}, nil
}

func (g *AzureDevopsProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	gitClient, err := g.clientFactory.GetClient(ctx)
	if err != nil {
		return nil, err
	}
	getRepoArgs := azureGit.GetRepositoriesArgs{Project: &g.teamProject}
	azureRepos, err := gitClient.GetRepositories(ctx, getRepoArgs)

	if err != nil {
		return nil, err
	}
	repos := []*Repository{}
	for _, azureRepo := range *azureRepos {
		if azureRepo.Name == nil || azureRepo.DefaultBranch == nil || azureRepo.RemoteUrl == nil || azureRepo.Id == nil {
			continue
		}
		repos = append(repos, &Repository{
			Organization: g.organization,
			Repository:   *azureRepo.Name,
			URL:          *azureRepo.RemoteUrl,
			Branch:       *azureRepo.DefaultBranch,
			Labels:       []string{},
			RepositoryId: *azureRepo.Id,
		})
	}

	return repos, nil
}

func (g *AzureDevopsProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	gitClient, err := g.clientFactory.GetClient(ctx)
	if err != nil {
		return false, err
	}

	idAsString := fmt.Sprintf("%v", repo.RepositoryId)
	branchName := repo.Branch
	getItemArgs := azureGit.GetItemArgs{RepositoryId: &idAsString, Project: &g.teamProject, Path: &path, VersionDescriptor: &azureGit.GitVersionDescriptor{Version: &branchName}}
	_, err = gitClient.GetItem(ctx, getItemArgs)

	if err != nil {
		if wrappedError, isWrappedError := err.(azuredevops.WrappedError); isWrappedError {
			if *wrappedError.TypeKey == "GitItemNotFoundException" {
				return false, nil
			}
		}
		return false, err
	}

	return true, nil
}

func (g *AzureDevopsProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	gitClient, err := g.clientFactory.GetClient(ctx)
	if err != nil {
		return nil, err
	}

	getBranchesRequest := azureGit.GetBranchesArgs{RepositoryId: &repo.Repository, Project: &g.teamProject}
	branches, err := gitClient.GetBranches(ctx, getBranchesRequest)
	if err != nil {
		return []*Repository{}, nil //Repo might be locked/unavailable, branch authz; all sorts of reasons why this would fail. Just return empty result.
	}
	repos := []*Repository{}
	for _, azureBranch := range *branches {
		repos = append(repos, &Repository{
			Branch:       *azureBranch.Name,
			SHA:          shortCommitSHA(*azureBranch.Commit.CommitId),
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Labels:       []string{},
			RepositoryId: repo.RepositoryId,
		})
	}

	return repos, nil
}

func shortCommitSHA(commitId string) string {
	if len(commitId) < 8 {
		return commitId
	}
	r := commitId[0:8]
	return r
}

func getBaseUrl() (*string, error) {
	baseUrl := os.Getenv("AZURE_DEVOPS_BASE_URL")
	if baseUrl == "" {
		baseUrl = AZURE_DEVOPS_DEFAULT_URL
	}
	urlCheck, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	ret := urlCheck.String()
	return &ret, nil
}
