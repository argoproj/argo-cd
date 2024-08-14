package scm_provider

import (
	"context"
	"errors"
	"fmt"
	netUrl "net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	azureGit "github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

const AZURE_DEVOPS_DEFAULT_URL = "https://dev.azure.com"

type azureDevOpsErrorTypeKeyValuesType struct {
	GitRepositoryNotFound string
	GitItemNotFound       string
}

var AzureDevOpsErrorsTypeKeyValues = azureDevOpsErrorTypeKeyValuesType{
	GitRepositoryNotFound: "GitRepositoryNotFoundException",
	GitItemNotFound:       "GitItemNotFoundException",
}

type AzureDevOpsClientFactory interface {
	// Returns an Azure Devops Client interface.
	GetClient(ctx context.Context) (azureGit.Client, error)
}

type devopsFactoryImpl struct {
	connection *azuredevops.Connection
}

func (factory *devopsFactoryImpl) GetClient(ctx context.Context) (azureGit.Client, error) {
	gitClient, err := azureGit.NewClient(ctx, factory.connection)
	if err != nil {
		return nil, fmt.Errorf("failed to get new Azure DevOps git client for SCM generator: %w", err)
	}
	return gitClient, nil
}

// Contains Azure Devops REST API implementation of SCMProviderService.
// See https://docs.microsoft.com/en-us/rest/api/azure/devops

type AzureDevOpsProvider struct {
	organization  string
	teamProject   string
	accessToken   string
	clientFactory AzureDevOpsClientFactory
	allBranches   bool
}

var (
	_ SCMProviderService       = &AzureDevOpsProvider{}
	_ AzureDevOpsClientFactory = &devopsFactoryImpl{}
)

func NewAzureDevOpsProvider(ctx context.Context, accessToken string, org string, url string, project string, allBranches bool) (*AzureDevOpsProvider, error) {
	if accessToken == "" {
		return nil, fmt.Errorf("no access token provided")
	}

	devOpsURL, err := getValidDevOpsURL(url, org)
	if err != nil {
		return nil, err
	}

	connection := azuredevops.NewPatConnection(devOpsURL, accessToken)

	return &AzureDevOpsProvider{organization: org, teamProject: project, accessToken: accessToken, clientFactory: &devopsFactoryImpl{connection: connection}, allBranches: allBranches}, nil
}

func (g *AzureDevOpsProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	gitClient, err := g.clientFactory.GetClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure DevOps client: %w", err)
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

func (g *AzureDevOpsProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	gitClient, err := g.clientFactory.GetClient(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get Azure DevOps client: %w", err)
	}

	var repoId string
	if uuid, isUuid := repo.RepositoryId.(uuid.UUID); isUuid { // most likely an UUID, but do type-safe check anyway. Do %v fallback if not expected type.
		repoId = uuid.String()
	} else {
		repoId = fmt.Sprintf("%v", repo.RepositoryId)
	}

	branchName := repo.Branch
	getItemArgs := azureGit.GetItemArgs{RepositoryId: &repoId, Project: &g.teamProject, Path: &path, VersionDescriptor: &azureGit.GitVersionDescriptor{Version: &branchName}}
	_, err = gitClient.GetItem(ctx, getItemArgs)
	if err != nil {
		var wrappedError azuredevops.WrappedError
		if errors.As(err, &wrappedError) && wrappedError.TypeKey != nil {
			if *wrappedError.TypeKey == AzureDevOpsErrorsTypeKeyValues.GitItemNotFound {
				return false, nil
			}
		}

		return false, fmt.Errorf("failed to check for path existence in Azure DevOps: %w", err)
	}

	return true, nil
}

func (g *AzureDevOpsProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	gitClient, err := g.clientFactory.GetClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure DevOps client: %w", err)
	}

	repos := []*Repository{}

	if !g.allBranches {
		defaultBranchName := strings.Replace(repo.Branch, "refs/heads/", "", 1) // Azure DevOps returns default branch info like 'refs/heads/main', but does not support branch lookup of this format.
		getBranchArgs := azureGit.GetBranchArgs{RepositoryId: &repo.Repository, Project: &g.teamProject, Name: &defaultBranchName}
		branchResult, err := gitClient.GetBranch(ctx, getBranchArgs)
		if err != nil {
			var wrappedError azuredevops.WrappedError
			if errors.As(err, &wrappedError) && wrappedError.TypeKey != nil {
				if *wrappedError.TypeKey == AzureDevOpsErrorsTypeKeyValues.GitRepositoryNotFound {
					return repos, nil
				}
			}
			return nil, fmt.Errorf("could not get default branch %v (%v) from repository %v: %w", defaultBranchName, repo.Branch, repo.Repository, err)
		}

		if branchResult.Name == nil || branchResult.Commit == nil {
			return nil, fmt.Errorf("invalid branch result after requesting branch %v from repository %v", repo.Branch, repo.Repository)
		}

		repos = append(repos, &Repository{
			Branch:       *branchResult.Name,
			SHA:          *branchResult.Commit.CommitId,
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Labels:       []string{},
			RepositoryId: repo.RepositoryId,
		})

		return repos, nil
	}

	getBranchesRequest := azureGit.GetBranchesArgs{RepositoryId: &repo.Repository, Project: &g.teamProject}
	branches, err := gitClient.GetBranches(ctx, getBranchesRequest)
	if err != nil {
		var wrappedError azuredevops.WrappedError
		if errors.As(err, &wrappedError) && wrappedError.TypeKey != nil {
			if *wrappedError.TypeKey == AzureDevOpsErrorsTypeKeyValues.GitRepositoryNotFound {
				return repos, nil
			}
		}
		return nil, fmt.Errorf("failed getting branches from repository %v, project %v: %w", repo.Repository, g.teamProject, err)
	}

	if branches == nil {
		return nil, fmt.Errorf("got empty branch result from repository %v, project %v: %w", repo.Repository, g.teamProject, err)
	}

	for _, azureBranch := range *branches {
		repos = append(repos, &Repository{
			Branch:       *azureBranch.Name,
			SHA:          *azureBranch.Commit.CommitId,
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Labels:       []string{},
			RepositoryId: repo.RepositoryId,
		})
	}

	return repos, nil
}

func getValidDevOpsURL(url string, org string) (string, error) {
	if url == "" {
		url = AZURE_DEVOPS_DEFAULT_URL
	}
	separator := ""
	if !strings.HasSuffix(url, "/") {
		separator = "/"
	}

	devOpsURL := fmt.Sprintf("%s%s%s", url, separator, org)

	urlCheck, err := netUrl.ParseRequestURI(devOpsURL)
	if err != nil {
		return "", fmt.Errorf("got an invalid URL for the Azure SCM generator: %w", err)
	}

	ret := urlCheck.String()
	return ret, nil
}
