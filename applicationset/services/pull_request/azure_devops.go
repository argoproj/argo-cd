package pull_request

import (
	"context"
	"fmt"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	gitcreds "github.com/argoproj/argo-cd/v3/util/git"
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

// devopsWorkloadIdentityFactory handles token refresh automatically for workload identity authentication
type devopsWorkloadIdentityFactory struct {
	organizationURL string
	creds           gitcreds.AzureWorkloadIdentityCreds
}

func (factory *devopsWorkloadIdentityFactory) GetClient(ctx context.Context) (git.Client, error) {
	// Get a fresh token (uses cached token if still valid, refreshes if expired)
	accessToken, err := factory.creds.GetAzureDevOpsAccessToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure DevOps access token using workload identity: %w", err)
	}

	// Use the SDK method to create the connection - this handles all the details properly
	connection := azuredevops.NewPatConnection(factory.organizationURL, accessToken)

	gitClient, err := git.NewClient(ctx, connection)
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
	_ AzureDevOpsClientFactory = &devopsWorkloadIdentityFactory{}
	_ AuthProvider             = &PatAuthProvider{}
	_ AuthProvider             = &WorkloadIdentityAuthProvider{}
	_ AuthProvider             = &AnonymousAuthProvider{}
)

// AuthProvider defines the interface for Azure DevOps authentication methods
type AuthProvider interface {
	CreateClientFactory(organizationURL string) AzureDevOpsClientFactory
}

// PatAuthProvider handles Personal Access Token authentication
type PatAuthProvider struct {
	token string
}

func NewPatAuthProvider(token string) *PatAuthProvider {
	return &PatAuthProvider{token: token}
}

func (p *PatAuthProvider) CreateClientFactory(organizationURL string) AzureDevOpsClientFactory {
	var connection *azuredevops.Connection
	if p.token == "" {
		connection = azuredevops.NewAnonymousConnection(organizationURL)
	} else {
		connection = azuredevops.NewPatConnection(organizationURL, p.token)
	}
	return &devopsFactoryImpl{connection: connection}
}

// AnonymousAuthProvider handles anonymous authentication
type AnonymousAuthProvider struct{}

func NewAnonymousAuthProvider() *AnonymousAuthProvider {
	return &AnonymousAuthProvider{}
}

func (a *AnonymousAuthProvider) CreateClientFactory(organizationURL string) AzureDevOpsClientFactory {
	connection := azuredevops.NewAnonymousConnection(organizationURL)
	return &devopsFactoryImpl{connection: connection}
}

// WorkloadIdentityAuthProvider handles Azure Workload Identity authentication
type WorkloadIdentityAuthProvider struct{}

func NewWorkloadIdentityAuthProvider() *WorkloadIdentityAuthProvider {
	return &WorkloadIdentityAuthProvider{}
}

func (w *WorkloadIdentityAuthProvider) CreateClientFactory(organizationURL string) AzureDevOpsClientFactory {
	tokenProvider := workloadidentity.NewWorkloadIdentityTokenProvider()
	creds := gitcreds.NewAzureWorkloadIdentityCreds(gitcreds.NoopCredsStore{}, tokenProvider)
	return &devopsWorkloadIdentityFactory{
		organizationURL: organizationURL,
		creds:           creds,
	}
}

func NewAzureDevOpsService(token string, providerConfig *argoprojiov1alpha1.PullRequestGeneratorAzureDevOps) (PullRequestService, error) {
	if token == "" {
		return NewAzureDevOpsServiceWithAuthProvider(providerConfig, NewAnonymousAuthProvider())
	}
	return NewAzureDevOpsServiceWithAuthProvider(providerConfig, NewPatAuthProvider(token))
}

// NewAzureDevOpsServiceWithAuthProvider creates a new Azure DevOps service with the specified authentication provider
func NewAzureDevOpsServiceWithAuthProvider(providerConfig *argoprojiov1alpha1.PullRequestGeneratorAzureDevOps, authProvider AuthProvider) (PullRequestService, error) {
	organizationURL := buildURL(providerConfig.API, providerConfig.Organization)

	clientFactory := authProvider.CreateClientFactory(organizationURL)

	return &AzureDevOpsService{
		clientFactory: clientFactory,
		project:       providerConfig.Project,
		repo:          providerConfig.Repo,
		labels:        providerConfig.Labels,
	}, nil
}

// NewAzureDevOpsServiceWithWorkloadIdentity creates a new Azure DevOps service using Azure Workload Identity
func NewAzureDevOpsServiceWithWorkloadIdentity(providerConfig *argoprojiov1alpha1.PullRequestGeneratorAzureDevOps) (PullRequestService, error) {
	return NewAzureDevOpsServiceWithAuthProvider(providerConfig, NewWorkloadIdentityAuthProvider())
}

// NewAzureDevOpsServiceAnonymous creates a new Azure DevOps service using anonymous authentication
func NewAzureDevOpsServiceAnonymous(providerConfig *argoprojiov1alpha1.PullRequestGeneratorAzureDevOps) (PullRequestService, error) {
	return NewAzureDevOpsServiceWithAuthProvider(providerConfig, NewAnonymousAuthProvider())
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
