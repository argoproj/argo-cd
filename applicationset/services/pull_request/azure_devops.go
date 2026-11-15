package pull_request

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"

	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
)

const (
	AZURE_DEVOPS_DEFAULT_URL             = "https://dev.azure.com"
	AZURE_DEVOPS_PROJECT_NOT_FOUND_ERROR = "The following project does not exist"

	// azureDevOpsTokenScope is the scope for the well-known Microsoft Entra ID resource of Azure DevOps.
	azureDevOpsTokenScope = "499b84ac-1321-427f-aa17-267ca6975798/.default"
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

// workloadIdentityClientFactory creates Azure DevOps clients authenticated with a Microsoft Entra ID
// bearer token obtained through Azure Workload Identity.
type workloadIdentityClientFactory struct {
	organizationURL string
	// getToken returns a valid Entra ID access token for the Azure DevOps resource.
	getToken func() (string, error)
}

func (factory *workloadIdentityClientFactory) GetClient(_ context.Context) (git.Client, error) {
	token, err := factory.getToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get Azure DevOps access token using Azure Workload Identity: %w", err)
	}
	return newPinnedGitClient(newBearerConnection(factory.organizationURL, token)), nil
}

// newPinnedGitClient returns an Azure DevOps git client that sends every request to the connection's base URL.
//
// git.NewClient first asks the server for the Git resource area and then sends all further requests, including
// the authorization header, to the LocationUrl the server returns, without checking its scheme or host. The
// workload identity token is a credential of the controller, so this client skips that discovery and builds the
// client on the already validated organization URL. Its HTTP client also refuses redirects, so that the token is
// never forwarded to another URL. A new connection and client are created on every call, because the SDK copies
// the authorization header into the client and would otherwise keep using an expired token.
func newPinnedGitClient(connection *azuredevops.Connection) git.Client {
	httpClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	client := azuredevops.NewClientWithOptions(connection, connection.BaseUrl, azuredevops.WithHTTPClient(httpClient))
	return &git.ClientImpl{Client: *client}
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
	_ AzureDevOpsClientFactory = &workloadIdentityClientFactory{}
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
// with a Microsoft Entra ID token obtained from tokenProvider, which is expected to use the Azure Workload
// Identity of the running pod. Token caching is left to tokenProvider, so callers should reuse one provider
// instead of creating a new one per service.
//
// Because the token is a credential of the controller rather than of the ApplicationSet author, it is only ever
// sent over HTTPS: an api URL with any other scheme is rejected before a token is requested.
func NewAzureDevOpsServiceWithWorkloadIdentity(tokenProvider workloadidentity.TokenProvider, apiURL, organization, project, repo string, labels []string, excludedLabels []string) (PullRequestService, error) {
	if tokenProvider == nil {
		return nil, errors.New("azure workload identity token provider must not be nil")
	}
	// Validate the api URL itself, before the organization is appended, so that the organization can never
	// end up being interpreted as the host.
	if apiURL != "" {
		parsedURL, err := url.Parse(apiURL)
		if err != nil {
			return nil, fmt.Errorf("invalid Azure DevOps api URL %q: %w", apiURL, err)
		}
		if !strings.EqualFold(parsedURL.Scheme, "https") || parsedURL.Host == "" {
			return nil, fmt.Errorf("useAzureWorkloadIdentity requires an https:// Azure DevOps api URL, got %q", apiURL)
		}
	}
	organizationURL := buildURL(apiURL, organization)
	return &AzureDevOpsService{
		clientFactory: &workloadIdentityClientFactory{
			organizationURL: organizationURL,
			getToken: func() (string, error) {
				token, err := tokenProvider.GetToken(azureDevOpsTokenScope)
				if err != nil {
					return "", err
				}
				return token.AccessToken, nil
			},
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
