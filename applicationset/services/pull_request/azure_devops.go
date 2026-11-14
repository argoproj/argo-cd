package pull_request

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"

	argogit "github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
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

// workloadIdentityFactoryImpl creates Azure DevOps clients authenticated with a Microsoft Entra ID
// bearer token obtained through Azure Workload Identity.
type workloadIdentityFactoryImpl struct {
	organizationURL string
	// getToken returns a valid Entra ID access token for the Azure DevOps resource.
	getToken func() (string, error)
}

func (factory *workloadIdentityFactoryImpl) GetClient(ctx context.Context) (git.Client, error) {
	token, err := factory.getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure DevOps access token using Azure Workload Identity: %w", err)
	}
	// A new connection is created on every call: the SDK copies the authorization header into the
	// clients it caches per connection, so reusing a connection would keep using an expired token.
	gitClient, err := git.NewClient(ctx, newBearerConnection(factory.organizationURL, token))
	if err != nil {
		return nil, fmt.Errorf("failed to get new Azure DevOps git client for pull request generator: %w", err)
	}
	return gitClient, nil
}

// newBearerConnection returns an Azure DevOps connection that authenticates with an OAuth bearer token.
func newBearerConnection(organizationURL, token string) *azuredevops.Connection {
	connection := azuredevops.NewAnonymousConnection(organizationURL)
	connection.AuthorizationString = "Bearer " + token
	return connection
}

type AzureDevOpsService struct {
	clientFactory  AzureDevOpsClientFactory
	project        string
	repo           string
	labels         []string
	excludedLabels []string
}

var (
	_ PullRequestService       = (*AzureDevOpsService)(nil)
	_ AzureDevOpsClientFactory = &devopsFactoryImpl{}
	_ AzureDevOpsClientFactory = &workloadIdentityFactoryImpl{}
)

func NewAzureDevOpsService(token, url, organization, project, repo string, labels []string, excludedLabels []string) (PullRequestService, error) {
	organizationURL := buildURL(url, organization)

	var connection *azuredevops.Connection
	if token == "" {
		connection = azuredevops.NewAnonymousConnection(organizationURL)
	} else {
		connection = azuredevops.NewPatConnection(organizationURL, token)
	}

	return &AzureDevOpsService{
		clientFactory:  &devopsFactoryImpl{connection: connection},
		project:        project,
		repo:           repo,
		labels:         labels,
		excludedLabels: excludedLabels,
	}, nil
}

// NewAzureDevOpsServiceWithWorkloadIdentity creates a PullRequestService that authenticates to Azure DevOps
// with the Azure Workload Identity of the running pod. Tokens are requested for the Azure DevOps resource and
// cached until shortly before they expire, the same way the repo-server does for Azure Repos.
func NewAzureDevOpsServiceWithWorkloadIdentity(tokenProvider workloadidentity.TokenProvider, url, organization, project, repo string, labels []string, excludedLabels []string) (PullRequestService, error) {
	creds := argogit.NewAzureWorkloadIdentityCreds(argogit.NoopCredsStore{}, tokenProvider)
	return &AzureDevOpsService{
		clientFactory: &workloadIdentityFactoryImpl{
			organizationURL: buildURL(url, organization),
			getToken:        creds.GetAzureDevOpsAccessToken,
		},
		project:        project,
		repo:           repo,
		labels:         labels,
		excludedLabels: excludedLabels,
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
		if containsAnyExcludedLabels(a.excludedLabels, azureDevOpsLabels) {
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
