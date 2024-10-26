package pull_request

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"

	"github.com/microsoft/azure-devops-go-api/azuredevops/core"
	git "github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	azureMock "github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider/azure_devops/git/mocks"
)

func createBoolPtr(x bool) *bool {
	return &x
}

func createStringPtr(x string) *string {
	return &x
}

func createIntPtr(x int) *int {
	return &x
}

func createLabelsPtr(x []core.WebApiTagDefinition) *[]core.WebApiTagDefinition {
	return &x
}

func createUniqueNamePtr(x string) *string {
	return &x
}

type AzureClientFactoryMock struct {
	mock *mock.Mock
}

func (m *AzureClientFactoryMock) GetClient(ctx context.Context) (git.Client, error) {
	args := m.mock.Called(ctx)

	var client git.Client
	c := args.Get(0)
	if c != nil {
		client = c.(git.Client)
	}

	var err error
	if len(args) > 1 {
		if e, ok := args.Get(1).(error); ok {
			err = e
		}
	}

	return client, err
}

func TestListPullRequest(t *testing.T) {
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	pr_id := 123
	pr_title := "feat(123)"
	pr_head_sha := "cd4973d9d14a08ffe6b641a89a68891d6aac8056"
	ctx := context.Background()
	uniqueName := "testName"

	pullRequestMock := []git.GitPullRequest{
		{
			PullRequestId: createIntPtr(pr_id),
			Title:         createStringPtr(pr_title),
			SourceRefName: createStringPtr("refs/heads/feature-branch"),
			TargetRefName: createStringPtr("refs/heads/main"),
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: createStringPtr(pr_head_sha),
			},
			Labels: &[]core.WebApiTagDefinition{},
			Repository: &git.GitRepository{
				Name: createStringPtr(repoName),
			},
			CreatedBy: &webapi.IdentityRef{
				UniqueName: createUniqueNamePtr(uniqueName + "@example.com"),
			},
		},
	}

	args := git.GetPullRequestsByProjectArgs{
		Project:        &teamProject,
		SearchCriteria: &git.GitPullRequestSearchCriteria{},
	}

	gitClientMock := azureMock.Client{}
	clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
	clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, nil)
	gitClientMock.On("GetPullRequestsByProject", ctx, args).Return(&pullRequestMock, nil)

	provider := AzureDevOpsService{
		clientFactory: clientFactoryMock,
		project:       teamProject,
		repo:          repoName,
		labels:        nil,
	}

	list, err := provider.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "feature-branch", list[0].Branch)
	assert.Equal(t, "main", list[0].TargetBranch)
	assert.Equal(t, pr_head_sha, list[0].HeadSHA)
	assert.Equal(t, "feat(123)", list[0].Title)
	assert.Equal(t, pr_id, list[0].Number)
	assert.Equal(t, uniqueName, list[0].Author)
}

func TestConvertLabes(t *testing.T) {
	testCases := []struct {
		name           string
		gotLabels      *[]core.WebApiTagDefinition
		expectedLabels []string
	}{
		{
			name:           "empty labels",
			gotLabels:      createLabelsPtr([]core.WebApiTagDefinition{}),
			expectedLabels: []string{},
		},
		{
			name:           "nil labels",
			gotLabels:      createLabelsPtr(nil),
			expectedLabels: []string{},
		},
		{
			name: "one label",
			gotLabels: createLabelsPtr([]core.WebApiTagDefinition{
				{Name: createStringPtr("label1"), Active: createBoolPtr(true)},
			}),
			expectedLabels: []string{"label1"},
		},
		{
			name: "two label",
			gotLabels: createLabelsPtr([]core.WebApiTagDefinition{
				{Name: createStringPtr("label1"), Active: createBoolPtr(true)},
				{Name: createStringPtr("label2"), Active: createBoolPtr(true)},
			}),
			expectedLabels: []string{"label1", "label2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertLabels(tc.gotLabels)
			assert.Equal(t, tc.expectedLabels, got)
		})
	}
}

func TestContainAzureDevOpsLabels(t *testing.T) {
	testCases := []struct {
		name           string
		expectedLabels []string
		gotLabels      []string
		expectedResult bool
	}{
		{
			name:           "empty labels",
			expectedLabels: []string{},
			gotLabels:      []string{},
			expectedResult: true,
		},
		{
			name:           "no matching labels",
			expectedLabels: []string{"label1", "label2"},
			gotLabels:      []string{"label3", "label4"},
			expectedResult: false,
		},
		{
			name:           "some matching labels",
			expectedLabels: []string{"label1", "label2"},
			gotLabels:      []string{"label1", "label3"},
			expectedResult: false,
		},
		{
			name:           "all matching labels",
			expectedLabels: []string{"label1", "label2"},
			gotLabels:      []string{"label1", "label2"},
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := containAzureDevOpsLabels(tc.expectedLabels, tc.gotLabels)
			assert.Equal(t, tc.expectedResult, got)
		})
	}
}

func TestBuildURL(t *testing.T) {
	testCases := []struct {
		name         string
		url          string
		organization string
		expected     string
	}{
		{
			name:         "Provided default URL and organization",
			url:          "https://dev.azure.com/",
			organization: "myorganization",
			expected:     "https://dev.azure.com/myorganization",
		},
		{
			name:         "Provided default URL and organization without trailing slash",
			url:          "https://dev.azure.com",
			organization: "myorganization",
			expected:     "https://dev.azure.com/myorganization",
		},
		{
			name:         "Provided no URL and organization",
			url:          "",
			organization: "myorganization",
			expected:     "https://dev.azure.com/myorganization",
		},
		{
			name:         "Provided custom URL and organization",
			url:          "https://azuredevops.example.com/",
			organization: "myorganization",
			expected:     "https://azuredevops.example.com/myorganization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildURL(tc.url, tc.organization)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAzureDevOpsPullRequestTLS(t *testing.T) {
	tests := []struct {
		name        string
		tlsInsecure bool
		passCerts   bool
		requireErr  bool
	}{
		{
			name:        "TLS Insecure: true, No Certs",
			tlsInsecure: true,
			passCerts:   false,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: true, With Certs",
			tlsInsecure: true,
			passCerts:   true,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: false, With Certs",
			tlsInsecure: false,
			passCerts:   true,
			requireErr:  false,
		},
		{
			name:        "TLS Insecure: false, No Certs",
			tlsInsecure: false,
			passCerts:   false,
			requireErr:  true,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			// Set up mock response data
			pr_id := 123
			pr_title := "feat(123)"
			pr_head_sha := "cd4973d9d14a08ffe6b641a89a68891d6aac8056"
			uniqueName := "testName"
			teamProject := "project"
			repoName := "repo"

			pullRequestMock := []git.GitPullRequest{
				{
					PullRequestId: createIntPtr(pr_id),
					Title:         createStringPtr(pr_title),
					SourceRefName: createStringPtr("refs/heads/feature-branch"),
					TargetRefName: createStringPtr("refs/heads/main"),
					LastMergeSourceCommit: &git.GitCommitRef{
						CommitId: createStringPtr(pr_head_sha),
					},
					Labels: &[]core.WebApiTagDefinition{},
					Repository: &git.GitRepository{
						Name: createStringPtr(repoName),
					},
					CreatedBy: &webapi.IdentityRef{
						UniqueName: createUniqueNamePtr(uniqueName + "@example.com"),
					},
				},
			}

			// Set up mocks
			gitClientMock := azureMock.Client{}
			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}

			args := git.GetPullRequestsByProjectArgs{
				Project:        &teamProject,
				SearchCriteria: &git.GitPullRequestSearchCriteria{},
			}

			if test.requireErr {
				clientFactoryMock.mock.On("GetClient", mock.Anything).Return(nil, fmt.Errorf("TLS certificate verification failed"))
			} else {
				clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, nil)
				gitClientMock.On("GetPullRequestsByProject", mock.Anything, args).Return(&pullRequestMock, nil)
			}

			// Generate certificates if needed
			var certs []byte
			if test.passCerts {
				privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
				require.NoError(t, err)

				// Create a template for the certificate
				template := &x509.Certificate{
					SerialNumber: big.NewInt(1),
					Subject: pkix.Name{
						Organization: []string{"Test Org"},
					},
					NotBefore:             time.Now(),
					NotAfter:              time.Now().Add(time.Hour * 24),
					KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
					ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
					BasicConstraintsValid: true,
				}
				certBytes, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)

				require.NoError(t, err)

				certs = pem.EncodeToMemory(&pem.Block{
					Type:  "CERTIFICATE",
					Bytes: certBytes,
				})
			}

			// Create service
			svc, err := NewAzureDevOpsService(
				context.Background(),
				"",
				"https://dev.azure.com",
				"org",
				teamProject,
				repoName,
				[]string{},
				"",
				test.tlsInsecure,
				certs,
			)
			require.NoError(t, err)

			// Replace the real client factory with our mock
			azureSvc := svc.(*AzureDevOpsService)
			azureSvc.clientFactory = clientFactoryMock

			// Test the List operation
			list, err := svc.List(context.Background())

			if test.requireErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Len(t, list, 1)
				assert.Equal(t, "feature-branch", list[0].Branch)
				assert.Equal(t, "main", list[0].TargetBranch)
				assert.Equal(t, pr_head_sha, list[0].HeadSHA)
				assert.Equal(t, pr_title, list[0].Title)
				assert.Equal(t, pr_id, list[0].Number)
				assert.Equal(t, uniqueName, list[0].Author)
			}
		})
	}
}
