package scm_provider

import (
	"errors"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/codecommit"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type awsCodeCommitTestRepository struct {
	name                     string
	id                       string
	arn                      string
	accountId                string
	defaultBranch            string
	expectedCloneURL         string
	getRepositoryError       error
	getRepositoryNilMetadata bool
	valid                    bool
}

func TestAWSCodeCommitListRepos(t *testing.T) {
	testCases := []struct {
		name                   string
		repositories           []*awsCodeCommitTestRepository
		cloneProtocol          string
		tagFilters             []*v1alpha1.TagFilter
		expectTagFilters       []*resourcegroupstaggingapi.TagFilter
		listRepositoryError    error
		expectOverallError     bool
		expectListAtCodeCommit bool
	}{
		{
			name:          "ListRepos by tag with https",
			cloneProtocol: "https",
			repositories: []*awsCodeCommitTestRepository{
				{
					name:             "repo1",
					id:               "8235624d-d248-4df9-a983-2558b01dbe83",
					arn:              "arn:aws:codecommit:us-east-1:111111111111:repo1",
					defaultBranch:    "main",
					expectedCloneURL: "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo1",
					valid:            true,
				},
			},
			tagFilters: []*v1alpha1.TagFilter{
				{Key: "key1", Value: "value1"},
				{Key: "key1", Value: "value2"},
				{Key: "key2"},
			},
			expectTagFilters: []*resourcegroupstaggingapi.TagFilter{
				{Key: new("key1"), Values: aws.StringSlice([]string{"value1", "value2"})},
				{Key: new("key2")},
			},
			expectOverallError:     false,
			expectListAtCodeCommit: false,
		},
		{
			name:          "ListRepos by tag with https-fips",
			cloneProtocol: "https-fips",
			repositories: []*awsCodeCommitTestRepository{
				{
					name:             "repo1",
					id:               "8235624d-d248-4df9-a983-2558b01dbe83",
					arn:              "arn:aws:codecommit:us-east-1:111111111111:repo1",
					defaultBranch:    "main",
					expectedCloneURL: "https://git-codecommit-fips.us-east-1.amazonaws.com/v1/repos/repo1",
					valid:            true,
				},
			},
			tagFilters: []*v1alpha1.TagFilter{
				{Key: "key1"},
			},
			expectTagFilters: []*resourcegroupstaggingapi.TagFilter{
				{Key: new("key1")},
			},
			expectOverallError:     false,
			expectListAtCodeCommit: false,
		},
		{
			name:          "ListRepos without tag with invalid repo",
			cloneProtocol: "ssh",
			repositories: []*awsCodeCommitTestRepository{
				{
					name:             "repo1",
					id:               "8235624d-d248-4df9-a983-2558b01dbe83",
					arn:              "arn:aws:codecommit:us-east-1:111111111111:repo1",
					defaultBranch:    "main",
					expectedCloneURL: "ssh://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo1",
					valid:            true,
				},
				{
					name:  "repo2",
					id:    "640d5859-d265-4e27-a9fa-e0731eb13ed7",
					arn:   "arn:aws:codecommit:us-east-1:111111111111:repo2",
					valid: false,
				},
				{
					name:                     "repo3-nil-metadata",
					id:                       "24a6ee96-d3a0-4be6-a595-c5e5b1ab1617",
					arn:                      "arn:aws:codecommit:us-east-1:111111111111:repo3-nil-metadata",
					getRepositoryNilMetadata: true,
					valid:                    false,
				},
			},
			expectOverallError:     false,
			expectListAtCodeCommit: true,
		},
		{
			name:          "ListRepos with invalid protocol",
			cloneProtocol: "invalid-protocol",
			repositories: []*awsCodeCommitTestRepository{
				{
					name:          "repo1",
					id:            "8235624d-d248-4df9-a983-2558b01dbe83",
					arn:           "arn:aws:codecommit:us-east-1:111111111111:repo1",
					defaultBranch: "main",
					valid:         true,
				},
			},
			expectOverallError:     true,
			expectListAtCodeCommit: true,
		},
		{
			name:                   "ListRepos error on listRepos",
			cloneProtocol:          "https",
			listRepositoryError:    errors.New("list repo error"),
			expectOverallError:     true,
			expectListAtCodeCommit: true,
		},
		{
			name:          "ListRepos error on getRepo",
			cloneProtocol: "https",
			repositories: []*awsCodeCommitTestRepository{
				{
					name:               "repo1",
					id:                 "8235624d-d248-4df9-a983-2558b01dbe83",
					arn:                "arn:aws:codecommit:us-east-1:111111111111:repo1",
					defaultBranch:      "main",
					getRepositoryError: errors.New("get repo error"),
					valid:              true,
				},
			},
			expectOverallError:     true,
			expectListAtCodeCommit: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			codeCommitClient := mocks.NewAWSCodeCommitClient(t)
			taggingClient := mocks.NewAWSTaggingClient(t)
			ctx := t.Context()
			codecommitRepoNameIdPairs := make([]*codecommit.RepositoryNameIdPair, 0)
			resourceTaggings := make([]*resourcegroupstaggingapi.ResourceTagMapping, 0)
			validRepositories := make([]*awsCodeCommitTestRepository, 0)

			for _, repo := range testCase.repositories {
				repoMetadata := &codecommit.RepositoryMetadata{
					AccountId:      new(repo.accountId),
					Arn:            new(repo.arn),
					CloneUrlHttp:   new("https://git-codecommit.us-east-1.amazonaws.com/v1/repos/" + repo.name),
					CloneUrlSsh:    new("ssh://git-codecommit.us-east-1.amazonaws.com/v1/repos/" + repo.name),
					DefaultBranch:  new(repo.defaultBranch),
					RepositoryId:   new(repo.id),
					RepositoryName: new(repo.name),
				}
				if repo.getRepositoryNilMetadata {
					repoMetadata = nil
				}
				codeCommitClient.EXPECT().GetRepositoryWithContext(mock.Anything, &codecommit.GetRepositoryInput{RepositoryName: new(repo.name)}).
					Return(&codecommit.GetRepositoryOutput{RepositoryMetadata: repoMetadata}, repo.getRepositoryError).Maybe()
				codecommitRepoNameIdPairs = append(codecommitRepoNameIdPairs, &codecommit.RepositoryNameIdPair{
					RepositoryId:   new(repo.id),
					RepositoryName: new(repo.name),
				})
				resourceTaggings = append(resourceTaggings, &resourcegroupstaggingapi.ResourceTagMapping{
					ResourceARN: new(repo.arn),
				})
				if repo.valid {
					validRepositories = append(validRepositories, repo)
				}
			}

			if testCase.expectListAtCodeCommit {
				codeCommitClient.EXPECT().ListRepositoriesWithContext(mock.Anything, &codecommit.ListRepositoriesInput{}).
					Return(&codecommit.ListRepositoriesOutput{
						Repositories: codecommitRepoNameIdPairs,
					}, testCase.listRepositoryError).Maybe()
			} else {
				taggingClient.EXPECT().GetResourcesWithContext(mock.Anything, mock.MatchedBy(equalIgnoringTagFilterOrder(&resourcegroupstaggingapi.GetResourcesInput{
					TagFilters:          testCase.expectTagFilters,
					ResourceTypeFilters: aws.StringSlice([]string{resourceTypeCodeCommitRepository}),
				}))).
					Return(&resourcegroupstaggingapi.GetResourcesOutput{
						ResourceTagMappingList: resourceTaggings,
					}, testCase.listRepositoryError).Maybe()
			}

			provider := &AWSCodeCommitProvider{
				codeCommitClient: codeCommitClient,
				taggingClient:    taggingClient,
				tagFilters:       testCase.tagFilters,
			}
			repos, err := provider.ListRepos(ctx, testCase.cloneProtocol)
			if testCase.expectOverallError {
				assert.Error(t, err)
			} else {
				assert.Len(t, repos, len(validRepositories))
				for i, repo := range repos {
					originRepo := validRepositories[i]
					assert.Equal(t, originRepo.accountId, repo.Organization)
					assert.Equal(t, originRepo.name, repo.Repository)
					assert.Equal(t, originRepo.id, repo.RepositoryId)
					assert.Equal(t, originRepo.defaultBranch, repo.Branch)
					assert.Equal(t, originRepo.expectedCloneURL, repo.URL)
					assert.Empty(t, repo.SHA, "SHA is always empty")
				}
			}
		})
	}
}

func TestAWSCodeCommitRepoHasPath(t *testing.T) {
	organization := "111111111111"
	repoName := "repo1"
	branch := "main"

	testCases := []struct {
		name                  string
		path                  string
		expectedGetFolderPath string
		getFolderOutput       *codecommit.GetFolderOutput
		getFolderError        error
		expectOverallError    bool
		expectedResult        bool
	}{
		{
			name:                  "RepoHasPath on regular file",
			path:                  "lib/config.yaml",
			expectedGetFolderPath: "/lib",
			getFolderOutput: &codecommit.GetFolderOutput{
				Files: []*codecommit.File{
					{RelativePath: new("config.yaml")},
				},
			},
			expectOverallError: false,
			expectedResult:     true,
		},
		{
			name:                  "RepoHasPath on folder",
			path:                  "lib/config",
			expectedGetFolderPath: "/lib",
			getFolderOutput: &codecommit.GetFolderOutput{
				SubFolders: []*codecommit.Folder{
					{RelativePath: new("config")},
				},
			},
			expectOverallError: false,
			expectedResult:     true,
		},
		{
			name:                  "RepoHasPath on submodules",
			path:                  "/lib/submodule/",
			expectedGetFolderPath: "/lib",
			getFolderOutput: &codecommit.GetFolderOutput{
				SubModules: []*codecommit.SubModule{
					{RelativePath: new("submodule")},
				},
			},
			expectOverallError: false,
			expectedResult:     true,
		},
		{
			name:                  "RepoHasPath on symlink",
			path:                  "./lib/service.json",
			expectedGetFolderPath: "/lib",
			getFolderOutput: &codecommit.GetFolderOutput{
				SymbolicLinks: []*codecommit.SymbolicLink{
					{RelativePath: new("service.json")},
				},
			},
			expectOverallError: false,
			expectedResult:     true,
		},
		{
			name:                  "RepoHasPath when no match",
			path:                  "no-match.json",
			expectedGetFolderPath: "/",
			getFolderOutput: &codecommit.GetFolderOutput{
				Files: []*codecommit.File{
					{RelativePath: new("config.yaml")},
				},
				SubFolders: []*codecommit.Folder{
					{RelativePath: new("config")},
				},
				SubModules: []*codecommit.SubModule{
					{RelativePath: new("submodule")},
				},
				SymbolicLinks: []*codecommit.SymbolicLink{
					{RelativePath: new("service.json")},
				},
			},
			expectOverallError: false,
			expectedResult:     false,
		},
		{
			name:                  "RepoHasPath when parent folder not found",
			path:                  "lib/submodule",
			expectedGetFolderPath: "/lib",
			getFolderError:        &codecommit.FolderDoesNotExistException{},
			expectOverallError:    false,
		},
		{
			name:                  "RepoHasPath when unknown error",
			path:                  "lib/submodule",
			expectedGetFolderPath: "/lib",
			getFolderError:        errors.New("unknown error"),
			expectOverallError:    true,
		},
		{
			name:               "RepoHasPath on root folder - './'",
			path:               "./",
			expectOverallError: false,
			expectedResult:     true,
		},
		{
			name:               "RepoHasPath on root folder - '/'",
			path:               "/",
			expectOverallError: false,
			expectedResult:     true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			codeCommitClient := mocks.NewAWSCodeCommitClient(t)
			taggingClient := mocks.NewAWSTaggingClient(t)
			ctx := t.Context()
			if testCase.expectedGetFolderPath != "" {
				codeCommitClient.EXPECT().GetFolderWithContext(mock.Anything, &codecommit.GetFolderInput{
					CommitSpecifier: new(branch),
					FolderPath:      new(testCase.expectedGetFolderPath),
					RepositoryName:  new(repoName),
				}).
					Return(testCase.getFolderOutput, testCase.getFolderError).Maybe()
			}
			provider := &AWSCodeCommitProvider{
				codeCommitClient: codeCommitClient,
				taggingClient:    taggingClient,
			}
			actual, err := provider.RepoHasPath(ctx, &Repository{
				Organization: organization,
				Repository:   repoName,
				Branch:       branch,
			}, testCase.path)
			if testCase.expectOverallError {
				assert.Error(t, err)
			} else {
				assert.Equal(t, testCase.expectedResult, actual)
			}
		})
	}
}

func TestAWSCodeCommitGetBranches(t *testing.T) {
	name := "repo1"
	id := "1a64adc4-2fb5-4abd-afe7-127984ba83c0"
	defaultBranch := "main"
	organization := "111111111111"
	cloneURL := "https://git-codecommit.us-east-1.amazonaws.com/v1/repos/repo1"

	testCases := []struct {
		name               string
		branches           []string
		apiError           error
		expectOverallError bool
		allBranches        bool
	}{
		{
			name:        "GetBranches all branches",
			branches:    []string{"main", "feature/codecommit", "chore/go-upgrade"},
			allBranches: true,
		},
		{
			name:        "GetBranches default branch only",
			allBranches: false,
		},
		{
			name:        "GetBranches default branch only",
			allBranches: false,
		},
		{
			name:               "GetBranches all branches on api error",
			apiError:           errors.New("api error"),
			expectOverallError: true,
			allBranches:        true,
		},
		{
			name:               "GetBranches default branch on api error",
			apiError:           errors.New("api error"),
			expectOverallError: true,
			allBranches:        false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			codeCommitClient := mocks.NewAWSCodeCommitClient(t)
			taggingClient := mocks.NewAWSTaggingClient(t)
			ctx := t.Context()
			if testCase.allBranches {
				codeCommitClient.EXPECT().ListBranchesWithContext(mock.Anything, &codecommit.ListBranchesInput{
					RepositoryName: new(name),
				}).
					Return(&codecommit.ListBranchesOutput{Branches: aws.StringSlice(testCase.branches)}, testCase.apiError).Maybe()
			} else {
				codeCommitClient.EXPECT().GetRepositoryWithContext(mock.Anything, &codecommit.GetRepositoryInput{RepositoryName: new(name)}).
					Return(&codecommit.GetRepositoryOutput{RepositoryMetadata: &codecommit.RepositoryMetadata{
						AccountId:     new(organization),
						DefaultBranch: new(defaultBranch),
					}}, testCase.apiError).Maybe()
			}
			provider := &AWSCodeCommitProvider{
				codeCommitClient: codeCommitClient,
				taggingClient:    taggingClient,
				allBranches:      testCase.allBranches,
			}
			actual, err := provider.GetBranches(ctx, &Repository{
				Organization: organization,
				Repository:   name,
				URL:          cloneURL,
				RepositoryId: id,
			})
			if testCase.expectOverallError {
				assert.Error(t, err)
			} else {
				assertCopiedProperties := func(repo *Repository) {
					assert.Equal(t, id, repo.RepositoryId)
					assert.Equal(t, name, repo.Repository)
					assert.Equal(t, cloneURL, repo.URL)
					assert.Equal(t, organization, repo.Organization)
					assert.Empty(t, repo.SHA)
				}
				actualBranches := make([]string, 0)
				for _, repo := range actual {
					assertCopiedProperties(repo)
					actualBranches = append(actualBranches, repo.Branch)
				}
				if testCase.allBranches {
					assert.ElementsMatch(t, testCase.branches, actualBranches)
				} else {
					assert.ElementsMatch(t, []string{defaultBranch}, actualBranches)
				}
			}
		})
	}
}

// equalIgnoringTagFilterOrder provides an argumentMatcher function that can be used to compare equality of GetResourcesInput ignoring the tagFilter ordering.
func equalIgnoringTagFilterOrder(expected *resourcegroupstaggingapi.GetResourcesInput) func(*resourcegroupstaggingapi.GetResourcesInput) bool {
	return func(actual *resourcegroupstaggingapi.GetResourcesInput) bool {
		sort.Slice(actual.TagFilters, func(i, j int) bool {
			return *actual.TagFilters[i].Key < *actual.TagFilters[j].Key
		})
		return cmp.Equal(expected, actual)
	}
}
