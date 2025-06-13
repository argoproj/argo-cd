package commit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/io/files"
)

// Service is the service that handles commit requests.
type Service struct {
	metricsServer     *metrics.Server
	repoClientFactory RepoClientFactory
}

// NewService returns a new instance of the commit service.
func NewService(gitCredsStore git.CredsStore, metricsServer *metrics.Server) *Service {
	return &Service{
		metricsServer:     metricsServer,
		repoClientFactory: NewRepoClientFactory(gitCredsStore, metricsServer),
	}
}

// CommitHydratedManifests handles a commit request. It clones the repository, checks out the sync branch, checks out
// the target branch, clears the repository contents, writes the manifests to the repository, commits the changes, and
// pushes the changes. It returns the hydrated revision SHA and an error if one occurred.
func (s *Service) CommitHydratedManifests(_ context.Context, r *apiclient.CommitHydratedManifestsRequest) (*apiclient.CommitHydratedManifestsResponse, error) {
	// This method is intentionally short. It's a wrapper around handleCommitRequest that adds metrics and logging.
	// Keep logic here minimal and put most of the logic in handleCommitRequest.
	startTime := time.Now()

	// We validate for a nil repo in handleCommitRequest, but we need to check for a nil repo here to get the repo URL
	// for metrics.
	var repoURL string
	if r.Repo != nil {
		repoURL = r.Repo.Repo
	}

	var err error
	s.metricsServer.IncPendingCommitRequest(repoURL)
	defer func() {
		s.metricsServer.DecPendingCommitRequest(repoURL)
		commitResponseType := metrics.CommitResponseTypeSuccess
		if err != nil {
			commitResponseType = metrics.CommitResponseTypeFailure
		}
		s.metricsServer.IncCommitRequest(repoURL, commitResponseType)
		s.metricsServer.ObserveCommitRequestDuration(repoURL, commitResponseType, time.Since(startTime))
	}()

	logCtx := log.WithFields(log.Fields{"branch": r.TargetBranch, "drySHA": r.DrySha})

	out, sha, err := s.handleCommitRequest(logCtx, r)
	if err != nil {
		logCtx.WithError(err).WithField("output", out).Error("failed to handle commit request")

		// No need to wrap this error, sufficient context is build in handleCommitRequest.
		return &apiclient.CommitHydratedManifestsResponse{}, err
	}

	logCtx.Info("Successfully handled commit request")
	return &apiclient.CommitHydratedManifestsResponse{
		HydratedSha: sha,
	}, nil
}

// handleCommitRequest handles the commit request. It clones the repository, checks out the sync branch, checks out the
// target branch, clears the repository contents, writes the manifests to the repository, commits the changes, and pushes
// the changes. It returns the output of the git commands and an error if one occurred.
func (s *Service) handleCommitRequest(logCtx *log.Entry, r *apiclient.CommitHydratedManifestsRequest) (string, string, error) {
	if r.Repo == nil {
		return "", "", errors.New("repo is required")
	}
	if r.Repo.Repo == "" {
		return "", "", errors.New("repo URL is required")
	}
	if r.TargetBranch == "" {
		return "", "", errors.New("target branch is required")
	}
	if r.SyncBranch == "" {
		return "", "", errors.New("sync branch is required")
	}

	logCtx = logCtx.WithField("repo", r.Repo.Repo)
	logCtx.Debug("Initiating git client")
	gitClient, dirPath, cleanup, err := s.initGitClient(logCtx, r)
	if err != nil {
		return "", "", fmt.Errorf("failed to init git client: %w", err)
	}
	defer cleanup()

	root, err := os.OpenRoot(dirPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to open root dir: %w", err)
	}
	defer io.Close(root)

	logCtx.Debugf("Checking out sync branch %s", r.SyncBranch)
	var out string
	out, err = gitClient.CheckoutOrOrphan(r.SyncBranch, false)
	if err != nil {
		return out, "", fmt.Errorf("failed to checkout sync branch: %w", err)
	}

	logCtx.Debugf("Checking out target branch %s", r.TargetBranch)
	out, err = gitClient.CheckoutOrNew(r.TargetBranch, r.SyncBranch, false)
	if err != nil {
		return out, "", fmt.Errorf("failed to checkout target branch: %w", err)
	}

	logCtx.Debug("Clearing repo contents")
	out, err = gitClient.RemoveContents()
	if err != nil {
		return out, "", fmt.Errorf("failed to clear repo: %w", err)
	}

	logCtx.Debug("Writing manifests")
	err = WriteForPaths(root, r.Repo.Repo, r.DrySha, r.DryCommitMetadata, r.Paths)
	if err != nil {
		return "", "", fmt.Errorf("failed to write manifests: %w", err)
	}

	logCtx.Debug("Committing and pushing changes")
	out, err = gitClient.CommitAndPush(r.TargetBranch, r.CommitMessage)
	if err != nil {
		return out, "", fmt.Errorf("failed to commit and push: %w", err)
	}

	logCtx.Debug("Getting commit SHA")
	sha, err := gitClient.CommitSHA()
	if err != nil {
		return "", "", fmt.Errorf("failed to get commit SHA: %w", err)
	}

	return "", sha, nil
}

// initGitClient initializes a git client for the given repository and returns the client, the path to the directory where
// the repository is cloned, a cleanup function that should be called when the directory is no longer needed, and an error
// if one occurred.
func (s *Service) initGitClient(logCtx *log.Entry, r *apiclient.CommitHydratedManifestsRequest) (git.Client, string, func(), error) {
	dirPath, err := files.CreateTempDir("/tmp/_commit-service")
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	// Call cleanupOrLog in this function if an error occurs to ensure the temp dir is cleaned up.
	cleanupOrLog := func() {
		err := os.RemoveAll(dirPath)
		if err != nil {
			logCtx.WithError(err).Error("failed to cleanup temp dir")
		}
	}

	gitClient, err := s.repoClientFactory.NewClient(r.Repo, dirPath)
	if err != nil {
		cleanupOrLog()
		return nil, "", nil, fmt.Errorf("failed to create git client: %w", err)
	}

	logCtx.Debugf("Initializing repo %s", r.Repo.Repo)
	err = gitClient.Init()
	if err != nil {
		cleanupOrLog()
		return nil, "", nil, fmt.Errorf("failed to init git client: %w", err)
	}

	logCtx.Debugf("Fetching repo %s", r.Repo.Repo)
	err = gitClient.Fetch("")
	if err != nil {
		cleanupOrLog()
		return nil, "", nil, fmt.Errorf("failed to clone repo: %w", err)
	}

	// FIXME: make it work for GHE
	// logCtx.Debugf("Getting user info for repo credentials")
	// gitCreds := r.Repo.GetGitCreds(s.gitCredsStore)
	// startTime := time.Now()
	// authorName, authorEmail, err := gitCreds.GetUserInfo(ctx)
	// s.metricsServer.ObserveUserInfoRequestDuration(r.Repo.Repo, getCredentialType(r.Repo), time.Since(startTime))
	// if err != nil {
	//	 cleanupOrLog()
	//	 return nil, "", nil, fmt.Errorf("failed to get github app info: %w", err)
	// }
	var authorName, authorEmail string

	if authorName == "" {
		authorName = "Argo CD"
	}
	if authorEmail == "" {
		logCtx.Warnf("Author email not available, using 'argo-cd@example.com'.")
		authorEmail = "argo-cd@example.com"
	}

	logCtx.Debugf("Setting author %s <%s>", authorName, authorEmail)
	_, err = gitClient.SetAuthor(authorName, authorEmail)
	if err != nil {
		cleanupOrLog()
		return nil, "", nil, fmt.Errorf("failed to set author: %w", err)
	}

	return gitClient, dirPath, cleanupOrLog, nil
}

type hydratorMetadataFile struct {
	RepoURL    string                       `json:"repoURL,omitempty"`
	DrySHA     string                       `json:"drySha,omitempty"`
	Commands   []string                     `json:"commands,omitempty"`
	Author     string                       `json:"author,omitempty"`
	Date       string                       `json:"date,omitempty"`
	Message    string                       `json:"message,omitempty"`
	References []v1alpha1.RevisionReference `json:"references,omitempty"`
}

// TODO: make this configurable via ConfigMap.
var manifestHydrationReadmeTemplate = `# Manifest Hydration

To hydrate the manifests in this repository, run the following commands:

` + "```shell" + `
git clone {{ .RepoURL }}
# cd into the cloned directory
git checkout {{ .DrySHA }}
{{ range $command := .Commands -}}
{{ $command }}
{{ end -}}` + "```" + `
{{ if .References -}}

## References

{{ range $ref := .References -}}
{{ if $ref.Commit -}}
* [{{ $ref.Commit.SHA | mustRegexFind "[0-9a-f]+" | trunc 7 }}]({{ $ref.Commit.RepoURL }}): {{ $ref.Commit.Subject }} ({{ $ref.Commit.Author.Name }} <{{ $ref.Commit.Author.Email }}>)
{{ end -}}
{{ end -}}
{{ end -}}`
