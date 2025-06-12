package scm_provider

import (
	"context"
	"errors"
	"fmt"
	pathpkg "path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go/aws/request"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/codecommit"
	"github.com/aws/aws-sdk-go/service/resourcegroupstaggingapi"
	log "github.com/sirupsen/logrus"
	"golang.org/x/exp/maps"
	"k8s.io/utils/strings/slices"

	application "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const (
	resourceTypeCodeCommitRepository = "codecommit:repository"
	prefixGitUrlHttps                = "https://git-codecommit."
	prefixGitUrlHttpsFIPS            = "https://git-codecommit-fips."
)

// AWSCodeCommitClient is a lean facade to the codecommitiface.CodeCommitAPI
// it helps to reduce the mockery generated code.
type AWSCodeCommitClient interface {
	ListRepositoriesWithContext(aws.Context, *codecommit.ListRepositoriesInput, ...request.Option) (*codecommit.ListRepositoriesOutput, error)
	GetRepositoryWithContext(aws.Context, *codecommit.GetRepositoryInput, ...request.Option) (*codecommit.GetRepositoryOutput, error)
	ListBranchesWithContext(aws.Context, *codecommit.ListBranchesInput, ...request.Option) (*codecommit.ListBranchesOutput, error)
	GetFolderWithContext(aws.Context, *codecommit.GetFolderInput, ...request.Option) (*codecommit.GetFolderOutput, error)
}

// AWSTaggingClient is a lean facade to the resourcegroupstaggingapiiface.ResourceGroupsTaggingAPIAPI
// it helps to reduce the mockery generated code.
type AWSTaggingClient interface {
	GetResourcesWithContext(aws.Context, *resourcegroupstaggingapi.GetResourcesInput, ...request.Option) (*resourcegroupstaggingapi.GetResourcesOutput, error)
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
		repo, err := p.codeCommitClient.GetRepositoryWithContext(ctx, &codecommit.GetRepositoryInput{
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
		if aws.StringValue(repo.RepositoryMetadata.DefaultBranch) == "" {
			// if a codecommit repo doesn't have default branch, it's uninitialized. not going to bother with it.
			log.Warnf("repository %s does not have default branch, skipped", repoName)
			continue
		}
		var url string
		switch cloneProtocol {
		// default to SSH if unspecified (i.e. if "").
		case "", "ssh":
			url = aws.StringValue(repo.RepositoryMetadata.CloneUrlSsh)
		case "https":
			url = aws.StringValue(repo.RepositoryMetadata.CloneUrlHttp)
		case "https-fips":
			url, err = getCodeCommitFIPSEndpoint(aws.StringValue(repo.RepositoryMetadata.CloneUrlHttp))
			if err != nil {
				return nil, fmt.Errorf("https-fips is provided but repoUrl can't be transformed to FIPS endpoint: %w", err)
			}
		default:
			return nil, fmt.Errorf("unknown clone protocol for codecommit %v", cloneProtocol)
		}
		repos = append(repos, &Repository{
			// there's no "organization" level at codecommit.
			// we are just using AWS accountId for now.
			Organization: aws.StringValue(repo.RepositoryMetadata.AccountId),
			Repository:   aws.StringValue(repo.RepositoryMetadata.RepositoryName),
			URL:          url,
			Branch:       aws.StringValue(repo.RepositoryMetadata.DefaultBranch),
			// we could propagate repo tag keys, but without value not sure if it's any useful.
			Labels:       []string{},
			RepositoryId: aws.StringValue(repo.RepositoryMetadata.RepositoryId),
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
	output, err := p.codeCommitClient.GetFolderWithContext(ctx, input)
	if err != nil {
		if hasAwsError(err,
			codecommit.ErrCodeRepositoryDoesNotExistException,
			codecommit.ErrCodeCommitDoesNotExistException,
			codecommit.ErrCodeFolderDoesNotExistException,
		) {
			return false, nil
		}
		// unhandled exception, propagate out
		return false, err
	}

	// anything that matches.
	for _, submodule := range output.SubModules {
		if basePath == aws.StringValue(submodule.RelativePath) {
			return true, nil
		}
	}
	for _, subpath := range output.SubFolders {
		if basePath == aws.StringValue(subpath.RelativePath) {
			return true, nil
		}
	}
	for _, subpath := range output.Files {
		if basePath == aws.StringValue(subpath.RelativePath) {
			return true, nil
		}
	}
	for _, subpath := range output.SymbolicLinks {
		if basePath == aws.StringValue(subpath.RelativePath) {
			return true, nil
		}
	}
	return false, nil
}

func (p *AWSCodeCommitProvider) GetBranches(ctx context.Context, repo *Repository) ([]*Repository, error) {
	repos := make([]*Repository, 0)
	if !p.allBranches {
		output, err := p.codeCommitClient.GetRepositoryWithContext(ctx, &codecommit.GetRepositoryInput{
			RepositoryName: aws.String(repo.Repository),
		})
		if err != nil {
			return nil, err
		}
		repos = append(repos, &Repository{
			Organization: repo.Organization,
			Repository:   repo.Repository,
			URL:          repo.URL,
			Branch:       aws.StringValue(output.RepositoryMetadata.DefaultBranch),
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
			output, err := p.codeCommitClient.ListBranchesWithContext(ctx, input)
			if err != nil {
				return nil, err
			}
			for _, branch := range output.Branches {
				repos = append(repos, &Repository{
					Organization: repo.Organization,
					Repository:   repo.Repository,
					URL:          repo.URL,
					Branch:       aws.StringValue(branch),
					RepositoryId: repo.RepositoryId,
					Labels:       repo.Labels,
					// getting SHA of the branch requires a separate GetBranch call.
					// too expensive. for now, we just don't support it.
					// SHA:          "",
				})
			}
			input.NextToken = output.NextToken
			if aws.StringValue(output.NextToken) == "" {
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
			output, err = p.codeCommitClient.ListRepositoriesWithContext(ctx, listReposInput)
			if err != nil {
				break
			}
			for _, repo := range output.Repositories {
				repoNames = append(repoNames, aws.StringValue(repo.RepositoryName))
			}
			listReposInput.NextToken = output.NextToken
			if aws.StringValue(output.NextToken) == "" {
				break
			}
		}
	} else {
		log.Debugf("tag filer is specified, calling tagging api to list repos")
		discoveryInput := &resourcegroupstaggingapi.GetResourcesInput{
			ResourceTypeFilters: aws.StringSlice([]string{resourceTypeCodeCommitRepository}),
			TagFilters:          tagFilters,
		}
		var output *resourcegroupstaggingapi.GetResourcesOutput
		for {
			output, err = p.taggingClient.GetResourcesWithContext(ctx, discoveryInput)
			if err != nil {
				break
			}
			for _, resource := range output.ResourceTagMappingList {
				repoArn := aws.StringValue(resource.ResourceARN)
				log.Debugf("discovered codecommit repo with arn %s", repoArn)
				repoName, extractErr := getCodeCommitRepoName(repoArn)
				if extractErr != nil {
					log.Warnf("discovered codecommit repoArn %s cannot be parsed due to %v", repoArn, err)
					continue
				}
				repoNames = append(repoNames, repoName)
			}
			discoveryInput.PaginationToken = output.PaginationToken
			if aws.StringValue(output.PaginationToken) == "" {
				break
			}
		}
	}
	return repoNames, err
}

func (p *AWSCodeCommitProvider) getTagFilters() []*resourcegroupstaggingapi.TagFilter {
	filters := make(map[string]*resourcegroupstaggingapi.TagFilter)
	for _, tagFilter := range p.tagFilters {
		filter, hasKey := filters[tagFilter.Key]
		if !hasKey {
			filter = &resourcegroupstaggingapi.TagFilter{
				Key: aws.String(tagFilter.Key),
			}
			filters[tagFilter.Key] = filter
		}
		if tagFilter.Value != "" {
			filter.Values = append(filter.Values, aws.String(tagFilter.Value))
		}
	}
	return maps.Values(filters)
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
func getCodeCommitFIPSEndpoint(repoUrl string) (string, error) {
	if strings.HasPrefix(repoUrl, prefixGitUrlHttpsFIPS) {
		log.Debugf("provided repoUrl %s is already a fips endpoint", repoUrl)
		return repoUrl, nil
	}
	if !strings.HasPrefix(repoUrl, prefixGitUrlHttps) {
		return "", fmt.Errorf("the provided https endpoint isn't recognized, cannot be transformed to FIPS endpoint: %s", repoUrl)
	}
	// we already have the prefix, so we guarantee to replace exactly the prefix only.
	return strings.Replace(repoUrl, prefixGitUrlHttps, prefixGitUrlHttpsFIPS, 1), nil
}

func hasAwsError(err error, codes ...string) bool {
	var awsErr awserr.Error
	if errors.As(err, &awsErr) {
		return slices.Contains(codes, awsErr.Code())
	}
	return false
}

// toAbsolutePath transforms a path input to absolute path, as required by AWS CodeCommit
// see https://docs.aws.amazon.com/codecommit/latest/APIReference/API_GetFolder.html
func toAbsolutePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.ToSlash(filepath.Join("/", path))
}

func createAWSDiscoveryClients(_ context.Context, role string, region string) (*resourcegroupstaggingapi.ResourceGroupsTaggingAPI, *codecommit.CodeCommit, error) {
	podSession, err := session.NewSession()
	if err != nil {
		return nil, nil, fmt.Errorf("error creating new AWS pod session: %w", err)
	}
	discoverySession := podSession
	// assume role if provided - this allows cross account CodeCommit repo discovery.
	if role != "" {
		log.Debugf("role %s is provided for AWS CodeCommit discovery", role)
		assumeRoleCreds := stscreds.NewCredentials(podSession, role)
		discoverySession, err = session.NewSession(&aws.Config{
			Credentials: assumeRoleCreds,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("error creating new AWS discovery session: %w", err)
		}
	} else {
		log.Debugf("role is not provided for AWS CodeCommit discovery, using pod role")
	}
	// use region explicitly if provided  - this allows cross region CodeCommit repo discovery.
	if region != "" {
		log.Debugf("region %s is provided for AWS CodeCommit discovery", region)
		discoverySession = discoverySession.Copy(&aws.Config{
			Region: aws.String(region),
		})
	} else {
		log.Debugf("region is not provided for AWS CodeCommit discovery, using pod region")
	}

	taggingClient := resourcegroupstaggingapi.New(discoverySession)
	codeCommitClient := codecommit.New(discoverySession)

	return taggingClient, codeCommitClient, nil
}
