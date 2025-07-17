package pull_request

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	core "github.com/microsoft/azure-devops-go-api/azuredevops/core"
	git "github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

const AZURE_DEVOPS_DEFAULT_URL = "https://dev.azure.com"

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

func NewAzureDevOpsService(ctx context.Context, token, url, organization, project, repo string, labels []string) (PullRequestService, error) {
	organizationUrl := buildURL(url, organization)

	var connection *azuredevops.Connection
	if token == "" {
		connection = azuredevops.NewAnonymousConnection(organizationUrl)
	} else {
		connection = azuredevops.NewPatConnection(organizationUrl, token)
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

	azurePullRequests, err := client.GetPullRequestsByProject(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull requests by project: %w", err)
	}

	pullRequests := []*PullRequest{}

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
				Number:       *pr.PullRequestId,
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
		found := false
		for _, got := range gotLabels {
			if expected == got {
				found = true
				break
			}
		}
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
