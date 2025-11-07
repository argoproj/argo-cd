package scm_provider

import (
	"context"
	"errors"
	"fmt"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/codecommit"
	codecommittypes "github.com/aws/aws-sdk-go-v2/service/codecommit/types"
	"github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi"
	taggingtypes "github.com/aws/aws-sdk-go-v2/service/resourcegroupstaggingapi/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	log "github.com/sirupsen/logrus"

	application "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const (
	resourceTypeCodeCommitRepository = "codecommit:repository"
	prefixGitURLHTTPS                = "https://git-codecommit."
	prefixGitURLHTTPSFIPS            = "https://git-codecommit-fips."
)

// AWSCodeCommitClient is a lean facade to the CodeCommit API client
// it helps to reduce the mockery generated code.
type AWSCodeCommitClient interface {
	ListRepositories(context.Context, *codecommit.ListRepositoriesInput, ...func(*codecommit.Options)) (*codecommit.ListRepositoriesOutput, error)
	GetRepository(context.Context, *codecommit.GetRepositoryInput, ...func(*codecommit.Options)) (*codecommit.GetRepositoryOutput, error)
	ListBranches(context.Context, *codecommit.ListBranchesInput, ...func(*codecommit.Options)) (*codecommit.ListBranchesOutput, error)
	GetFolder(context.Context, *codecommit.GetFolderInput, ...func(*codecommit.Options)) (*codecommit.GetFolderOutput, error)
}

// AWSTaggingClient is a lean facade to the Resource Groups Tagging API client
// it helps to reduce the mockery generated code.
type AWSTaggingClient interface {
	GetResources(context.Context, *resourcegroupstaggingapi.GetResourcesInput, ...func(*resourcegroupstaggingapi.Options)) (*resourcegroupstaggingapi.GetResourcesOutput, error)
}

type AWSCodeCommitProvider struct {
	codeCommitClient AWSCodeCommitClient
	taggingClient    AWSTaggingClient
	tagFilters       []*application.TagFilter
	allBranches      bool
}

func NewAWSCodeCommitProvider(ctx context.Context, tagFilters []*application.TagFilter, role string, region string, allBranches bool) (*AWSCodeCommitProvider, error) {
	taggingClient, codeCommitClient, err := createAWSDiscoveryClients(ctx, role, region)
	if err != nil {
		return nil, err
	}
	return &AWSCodeCommitProvider{
		codeCommitClient: codeCommitClient,
		taggingClient:    taggingClient,
		tagFilters:       tagFilters,
		allBranches:      allBranches,
	}, nil
}

func (p *AWSCodeCommitProvider) ListRepos(ctx context.Context, cloneProtocol string) ([]*Repository, error) {
	repos := make([]*Repository, 0)

	repoNames, err := p.listRepoNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list codecommit repository: %w", err)
	}

	for _, repoName := range repoNames {
		repo, err := p.codeCommitClient.GetRepository(ctx, &codecommit.GetRepositoryInput{
			RepositoryName: aws.String(repoName),
		})
		if err != nil {
			// we don't want to skip at this point. It's a valid repo, we don't want to have flapping Application on an AWS outage.
			return nil, fmt.Errorf("failed to get codecommit repository: %w", err)
		}
		if repo == nil || repo.RepositoryMetadata == nil {
			// unlikely to happen, but just in case to protect nil pointer dereferences.
			log.Warnf("codecommit returned invalid response for repository %s, skipped", repoName)
			continue
		}
		if aws.ToString(repo.RepositoryMetadata.DefaultBranch) == "" {
			// if a codecommit repo doesn't have default branch, it's uninitialized. not going to bother with it.
			log.Warnf("repository %s does not have default branch, skipped", repoName)
			continue
		}
		var url string
		switch cloneProtocol {
		// default to SSH if unspecified (i.e. if "").
		case "", "ssh":
			url = aws.ToString(repo.RepositoryMetadata.CloneUrlSsh)
		case "https":
			url = aws.ToString(repo.RepositoryMetadata.CloneUrlHttp)
		case "https-fips":
			url, err = getCodeCommitFIPSEndpoint(aws.ToString(repo.RepositoryMetadata.CloneUrlHttp))
			if err != nil {
				return nil, fmt.Errorf("https-fips is provided but repoUrl can't be transformed to FIPS endpoint: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown clone protocol for codecommit %v", cloneProtocol)
		}
		repos = append(repos, &Repository{
			// there's no "organization" level at codecommit.
			// we are just using AWS accountId for now.
			Organization: aws.ToString(repo.RepositoryMetadata.AccountId),
			Repository:   aws.ToString(repo.RepositoryMetadata.RepositoryName),
			URL:          url,
			Branch:       aws.ToString(repo.RepositoryMetadata.DefaultBranch),
			// we could propagate repo tag keys, but without value not sure if it's any useful.
			Labels:       []string{},
			RepositoryId: aws.ToString(repo.RepositoryMetadata.RepositoryId),
		})
	}

	return repos, nil
}

func (p *AWSCodeCommitProvider) RepoHasPath(ctx context.Context, repo *Repository, path string) (bool, error) {
	// we use GetFolder instead of GetFile here because GetFile always downloads the full blob which has scalability problem.
	// GetFolder is slightly less concerning.

	path = toAbsolutePath(path)
	// shortcut: if it's root folder ('/'), we always return true.
	if path == "/" {
		return true, nil
	}
	// here we are sure it's not root folder, strip the suffix for easier comparison.
	path = strings.TrimSuffix(path, "/")

	// we always get the parent folder, so we could support both submodule, file, symlink and folder cases.
	parentPath := pathpkg.Dir(path)
	basePath := pathpkg.Base(path)

	input := &codecommit.GetFolderInput{
		CommitSpecifier: aws.String(repo.Branch),
		FolderPath:      aws.String(parentPath),
		RepositoryName:  aws.String(repo.Repository),
	}
	output, err := p.codeCommitClient.GetFolder(ctx, input)
	if err != nil {
		if hasAwsError(err) {
			return false, nil
		}
		// unhandled exception, propagate out
		return false, err
	}

	// anything that matches.
	for _, submodule := range output.SubModules {
		if basePath == aws.ToString(submodule.RelativePath) {
			return true, nil
		}
	}
	for _, subpath := range output.SubFolders {
		if basePath == aws.ToString(subpath.RelativePath) {
			return true, nil
		}
	}
	for _, subpath := range output.Files {
		if basePath == aws.ToString(subpath.RelativePath) {
			return true, nil
		}
	}
	for _, subpath := range output.SymbolicLinks {
		if basePath == aws.ToString(subpath.RelativePath) {
			return true, nil
		}
	}
	return false, nil
}

func (p *AWSCodeCommitProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	repos := make([]*Repository, 0)
	if !p.allBranches {
		output, err := p.codeCommitClient.GetRepository(ctx, &codecommit.GetRepositoryInput{
			RepositoryName: aws.String(repo.Repository),
		})
		if err != nil {
			return nil, err
		}
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Branch:       aws.ToString(output.RepositoryMetadata.DefaultBranch),
			RepositoryId: repo.RepositoryId,
			Labels:       repo.Labels,
			// getting SHA of the branch requires a separate GetBranch call.
			// too expensive. for now, we just don't support it.
			// SHA:          "",
		})
	} else {
		input := &codecommit.ListBranchesInput{
			RepositoryName: aws.String(repo.Repository),
		}
		for {
			output, err := p.codeCommitClient.ListBranches(ctx, input)
			if err != nil {
				return nil, err
			}
			for _, branch := range output.Branches {
				repos = append(repos, &Repository{
					Organization: repo.Organization,
					Repository:   repo.Repository,
					URL:          repo.URL,
					Branch:       branch,
					RepositoryId: repo.RepositoryId,
					Labels:       repo.Labels,
					// getting SHA of the branch requires a separate GetBranch call.
					// too expensive. for now, we just don't support it.
					// SHA:          "",
				})
			}
			input.NextToken = output.NextToken
			if aws.ToString(output.NextToken) == "" {
				break
			}
		}
	}

	return repos, nil
}

func (p *AWSCodeCommitProvider) listRepoNames(ctx context.Context) ([]string, error) {
	tagFilters := p.getTagFilters()
	repoNames := make([]string, 0)
	var err error

	if len(tagFilters) < 1 {
		log.Debugf("no tag filer, calling codecommit api to list repos")
		listReposInput := &codecommit.ListRepositoriesInput{}
		var output *codecommit.ListRepositoriesOutput
		for {
			output, err = p.codeCommitClient.ListRepositories(ctx, listReposInput)
			if err != nil {
				break
			}
			for _, repo := range output.Repositories {
				repoNames = append(repoNames, aws.ToString(repo.RepositoryName))
			}
			listReposInput.NextToken = output.NextToken
			if aws.ToString(output.NextToken) == "" {
				break
			}
		}
	} else {
		log.Debugf("tag filer is specified, calling tagging api to list repos")
		discoveryInput := &resourcegroupstaggingapi.GetResourcesInput{
			ResourceTypeFilters: []string{resourceTypeCodeCommitRepository},
			TagFilters:          tagFilters,
		}
		var output *resourcegroupstaggingapi.GetResourcesOutput
		for {
			output, err = p.taggingClient.GetResources(ctx, discoveryInput)
			if err != nil {
				break
			}
			for _, resource := range output.ResourceTagMappingList {
				repoArn := aws.ToString(resource.ResourceARN)
				log.Debugf("discovered codecommit repo with arn %s", repoArn)
				repoName, extractErr := getCodeCommitRepoName(repoArn)
				if extractErr != nil {
					log.Warnf("discovered codecommit repoArn %s cannot be parsed due to %v", repoArn, err)
					continue
				}
				repoNames = append(repoNames, repoName)
			}
			discoveryInput.PaginationToken = output.PaginationToken
			if aws.ToString(output.PaginationToken) == "" {
				break
			}
		}
	}
	return repoNames, err
}

func (p *AWSCodeCommitProvider) getTagFilters() []taggingtypes.TagFilter {
	filters := make(map[string]*taggingtypes.TagFilter)
	for _, tagFilter := range p.tagFilters {
		filter, hasKey := filters[tagFilter.Key]
		if !hasKey {
			filter = &taggingtypes.TagFilter{
				Key: aws.String(tagFilter.Key),
			}
			filters[tagFilter.Key] = filter
		}
		if tagFilter.Value != "" {
			filter.Values = append(filter.Values, tagFilter.Value)
		}
	}
	result := make([]taggingtypes.TagFilter, 0, len(filters))
	for _, filter := range filters {
		result = append(result, *filter)
	}
	return result
}

func getCodeCommitRepoName(repoArn string) (string, error) {
	parsedArn, err := arn.Parse(repoArn)
	if err != nil {
		return "", fmt.Errorf("failed to parse codecommit repository ARN: %w", err)
	}
	// see: https://docs.aws.amazon.com/codecommit/latest/userguide/auth-and-access-control-permissions-reference.html
	// arn:aws:codecommit:region:account-id:repository-name
	return parsedArn.Resource, nil
}

// getCodeCommitFIPSEndpoint transforms provided https:// codecommit URL to a FIPS-compliant endpoint.
// note that the specified region must support FIPS, otherwise the returned URL won't be reachable
// see: https://docs.aws.amazon.com/codecommit/latest/userguide/regions.html#regions-git
func getCodeCommitFIPSEndpoint(repoURL string) (string, error) {
	if strings.HasPrefix(repoURL, prefixGitURLHTTPSFIPS) {
		log.Debugf("provided repoUrl %s is already a fips endpoint", repoURL)
		return repoURL, nil
	}
	if !strings.HasPrefix(repoURL, prefixGitURLHTTPS) {
		return "", fmt.Errorf("the provided https endpoint isn't recognized, cannot be transformed to FIPS endpoint: %s", repoURL)
	}
	// we already have the prefix, so we guarantee to replace exactly the prefix only.
	return strings.Replace(repoURL, prefixGitURLHTTPS, prefixGitURLHTTPSFIPS, 1), nil
}

func hasAwsError(err error) bool {
	// Check for common CodeCommit exceptions using SDK v2 typed errors
	var repoNotFound *codecommittypes.RepositoryDoesNotExistException
	var commitNotFound *codecommittypes.CommitDoesNotExistException
	var folderNotFound *codecommittypes.FolderDoesNotExistException

	return errors.As(err, &repoNotFound) ||
		errors.As(err, &commitNotFound) ||
		errors.As(err, &folderNotFound)
}

// toAbsolutePath transforms a path input to absolute path, as required by AWS CodeCommit
// see https://docs.aws.amazon.com/codecommit/latest/APIReference/API_GetFolder.html
func toAbsolutePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.ToSlash(filepath.Join("/", path)) //nolint:gocritic // Prepend slash to have an absolute path
}

func createAWSDiscoveryClients(ctx context.Context, role string, region string) (*resourcegroupstaggingapi.Client, *codecommit.Client, error) {
	// Build config options
	var configOpts []func(*config.LoadOptions) error

	// Add region if provided
	if region != "" {
		log.Debugf("region %s is provided for AWS CodeCommit discovery", region)
		configOpts = append(configOpts, config.WithRegion(region))
	} else {
		log.Debugf("region is not provided for AWS CodeCommit discovery, using pod region")
	}

	// Load base config
	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("error loading AWS config: %w", err)
	}

	// Assume role if provided - this allows cross account CodeCommit repo discovery
	if role != "" {
		log.Debugf("role %s is provided for AWS CodeCommit discovery", role)
		stsClient := sts.NewFromConfig(cfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, role)
		cfg.Credentials = aws.NewCredentialsCache(creds)
	} else {
		log.Debugf("role is not provided for AWS CodeCommit discovery, using pod role")
	}

	taggingClient := resourcegroupstaggingapi.NewFromConfig(cfg)
	codeCommitClient := codecommit.NewFromConfig(cfg)

	return taggingClient, codeCommitClient, nil
}
