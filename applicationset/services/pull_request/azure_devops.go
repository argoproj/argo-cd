package pull_request

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
)

const (
	AZURE_DEVOPS_DEFAULT_URL             = "https://dev.azure.com"
	AZURE_DEVOPS_PROJECT_NOT_FOUND_ERROR = "The following project does not exist"
)

type AzureDevOpsClientFactory interface {
	// Returns an Azure Devops Client interface.
	GetClient(ctx context.Context) (git.Client, error)
}

type devopsFactoryImpl struct {
	connection *azuredevops.Connection
}

func (factory *devopsFactoryImpl) GetClient(ctx context.Context) (git.Client, error) {
	gitClient, err := git.NewClient(ctx, factory.connection)
	if err != nil {
		return nil, fmt.Errorf("failed to get new Azure DevOps git client for pull request generator: %w", err)
	}
	return gitClient, nil
}

type AzureDevOpsService struct {
	clientFactory AzureDevOpsClientFactory
	project       string
	repo          string
	labels        []string
}

var (
	_ PullRequestService       = (*AzureDevOpsService)(nil)
	_ AzureDevOpsClientFactory = &devopsFactoryImpl{}
)

func NewAzureDevOpsService(token, url, organization, project, repo string, labels []string) (PullRequestService, error) {
	organizationURL := buildURL(url, organization)

	var connection *azuredevops.Connection
	if token == "" {
		connection = azuredevops.NewAnonymousConnection(organizationURL)
	} else {
		connection = azuredevops.NewPatConnection(organizationURL, token)
	}

	return &AzureDevOpsService{
		clientFactory: &devopsFactoryImpl{connection: connection},
		project:       project,
		repo:          repo,
		labels:        labels,
	}, nil
}

func (a *AzureDevOpsService) List(ctx context.Context) ([]*PullRequest, error) {
	client, err := a.clientFactory.GetClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure DevOps client: %w", err)
	}

	args := git.GetPullRequestsByProjectArgs{
		Project:        &a.project,
		SearchCriteria: &git.GitPullRequestSearchCriteria{},
	}

	pullRequests := []*PullRequest{}

	azurePullRequests, err := client.GetPullRequestsByProject(ctx, args)
	if err != nil {
		// A standard Http 404 error is not returned for Azure DevOps,
		// so checking the error message for a specific pattern.
		// NOTE: Since the repos are filtered later, only existence of the project
		// is relevant for AzureDevOps
		if strings.Contains(err.Error(), AZURE_DEVOPS_PROJECT_NOT_FOUND_ERROR) {
			// return a custom error indicating that the repository is not found,
			// but also return the empty result since the decision to continue or not in this case is made by the caller
			return pullRequests, NewRepositoryNotFoundError(err)
		}
		return nil, fmt.Errorf("failed to get pull requests by project: %w", err)
	}

	for _, pr := range *azurePullRequests {
		if pr.Repository == nil ||
			pr.Repository.Name == nil ||
			pr.PullRequestId == nil ||
			pr.SourceRefName == nil ||
			pr.TargetRefName == nil ||
			pr.LastMergeSourceCommit == nil ||
			pr.LastMergeSourceCommit.CommitId == nil {
			continue
		}

		azureDevOpsLabels := convertLabels(pr.Labels)
		if !containAzureDevOpsLabels(a.labels, azureDevOpsLabels) {
			continue
		}

		if *pr.Repository.Name == a.repo {
			pullRequests = append(pullRequests, &PullRequest{
				Number:       int64(*pr.PullRequestId),
				Title:        *pr.Title,
				Branch:       strings.Replace(*pr.SourceRefName, "refs/heads/", "", 1),
				TargetBranch: strings.Replace(*pr.TargetRefName, "refs/heads/", "", 1),
				HeadSHA:      *pr.LastMergeSourceCommit.CommitId,
				Labels:       azureDevOpsLabels,
				Author:       strings.Split(*pr.CreatedBy.UniqueName, "@")[0], // Get the part before the @ in the email-address
			})
		}
	}

	return pullRequests, nil
}

// convertLabels converts WebApiTagDefinitions to strings
func convertLabels(tags *[]core.WebApiTagDefinition) []string {
	if tags == nil {
		return []string{}
	}
	labelStrings := make([]string, len(*tags))
	for i, label := range *tags {
		labelStrings[i] = *label.Name
	}
	return labelStrings
}

// containAzureDevOpsLabels returns true if gotLabels contains expectedLabels
func containAzureDevOpsLabels(expectedLabels []string, gotLabels []string) bool {
	for _, expected := range expectedLabels {
		found := slices.Contains(gotLabels, expected)
		if !found {
			return false
		}
	}
	return true
}

func buildURL(url, organization string) string {
	if url == "" {
		url = AZURE_DEVOPS_DEFAULT_URL
	}
	separator := ""
	if !strings.HasSuffix(url, "/") {
		separator = "/"
	}
	devOpsURL := fmt.Sprintf("%s%s%s", url, separator, organization)
	return devOpsURL
}
