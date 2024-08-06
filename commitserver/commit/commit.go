package commit

import (
	"context"
	"fmt"
	"github.com/argoproj/argo-cd/v2/util/io/files"
	"os"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/commitserver/metrics"
	"github.com/argoproj/argo-cd/v2/util/git"
)

type Service struct {
	gitCredsStore git.CredsStore
	metricsServer *metrics.Server
}

func NewService(gitCredsStore git.CredsStore, metricsServer *metrics.Server) *Service {
	return &Service{gitCredsStore: gitCredsStore, metricsServer: metricsServer}
}

func (s *Service) CommitHydratedManifests(ctx context.Context, r *apiclient.CommitHydratedManifestsRequest) (*apiclient.CommitHydratedManifestsResponse, error) {
	// This method is intentionally short. It's a wrapper around handleCommitRequest that adds metrics and logging.
	// Keep logic here minimal and put most of the logic in handleCommitRequest.

	startTime := time.Now()
	s.metricsServer.IncPendingCommitRequest(r.Repo.Repo)
	defer s.metricsServer.DecPendingCommitRequest(r.Repo.Repo)

	logCtx := log.WithFields(log.Fields{"repo": r.Repo.Repo, "branch": r.TargetBranch, "drySHA": r.DrySha})

	out, err := s.handleCommitRequest(ctx, logCtx, r)
	if err != nil {
		logCtx.WithError(err).WithField("output", out).Error("failed to handle commit request")
		s.metricsServer.IncCommitRequest(r.Repo.Repo, metrics.CommitRequestTypeFailure)
		s.metricsServer.ObserveCommitRequestDuration(r.Repo.Repo, metrics.CommitRequestTypeFailure, time.Since(startTime))

		// No need to wrap this error, sufficient context is build in handleCommitRequest.
		return &apiclient.CommitHydratedManifestsResponse{}, err
	}

	logCtx.Info("Successfully handled commit request")
	s.metricsServer.IncCommitRequest(r.Repo.Repo, metrics.CommitRequestTypeSuccess)
	s.metricsServer.ObserveCommitRequestDuration(r.Repo.Repo, metrics.CommitRequestTypeSuccess, time.Since(startTime))
	return &apiclient.CommitHydratedManifestsResponse{}, nil
}

// handleCommitRequest handles the commit request. It clones the repository, checks out the sync branch, checks out the
// target branch, clears the repository contents, writes the manifests to the repository, commits the changes, and pushes
// the changes. It returns the output of the git commands and an error if one occurred.
func (s *Service) handleCommitRequest(ctx context.Context, logCtx *log.Entry, r *apiclient.CommitHydratedManifestsRequest) (string, error) {
	logCtx.Debug("Initiating git client")
	gitClient, dirPath, cleanup, err := s.initGitClient(ctx, logCtx, r)
	if err != nil {
		return "", fmt.Errorf("failed to init git client: %w", err)
	}
	defer cleanup()

	// Checkout the sync branch
	logCtx.Debugf("Checking out sync branch %s", r.SyncBranch)
	var out string
	out, err = gitClient.CheckoutOrOrphan(r.SyncBranch, false)
	if err != nil {
		return out, fmt.Errorf("failed to checkout sync branch: %w", err)
	}

	// Checkout the target branch
	logCtx.Debugf("Checking out target branch %s", r.TargetBranch)
	out, err = gitClient.CheckoutOrNew(r.TargetBranch, r.SyncBranch, false)
	if err != nil {
		return out, fmt.Errorf("failed to checkout target branch: %w", err)
	}

	// Clear the repo contents using git rm
	logCtx.Debug("Clearing repo contents")
	out, err = gitClient.RemoveContents()
	if err != nil {
		return out, fmt.Errorf("failed to clear repo: %w", err)
	}

	// Write the manifests to the temp dir
	logCtx.Debugf("Writing manifests")
	err = WriteForPaths(dirPath, r.Repo.Repo, r.DrySha, r.Paths)
	if err != nil {
		return "", fmt.Errorf("failed to write manifests: %w", err)
	}

	// Commit the changes
	logCtx.Debugf("Committing and pushing changes")
	out, err = gitClient.CommitAndPush(r.TargetBranch, r.CommitMessage)
	if err != nil {
		return out, fmt.Errorf("failed to commit and push: %w", err)
	}

	return "", nil
}

// initGitClient initializes a git client for the given repository and returns the client, the path to the directory where
// the repository is cloned, a cleanup function that should be called when the directory is no longer needed, and an error
// if one occurred.
func (s *Service) initGitClient(ctx context.Context, logCtx *log.Entry, r *apiclient.CommitHydratedManifestsRequest) (git.Client, string, func(), error) {
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

	gitCreds := r.Repo.GetGitCreds(s.gitCredsStore)
	opts := git.WithEventHandlers(metrics.NewGitClientEventHandlers(s.metricsServer))
	gitClient, err := git.NewClientExt(r.Repo.Repo, dirPath, gitCreds, r.Repo.IsInsecure(), r.Repo.IsLFSEnabled(), r.Repo.Proxy, opts)
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

	logCtx.Debugf("Getting user info for repo credentials")
	// TODO: Produce metrics on getting user info, since it'll generally hit APIs. Make sure to label by _which_ API is
	//       being hit.
	authorName, authorEmail, err := gitCreds.GetUserInfo(ctx)
	if err != nil {
		cleanupOrLog()
		return nil, "", nil, fmt.Errorf("failed to get github app info: %w", err)
	}

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
	RepoURL  string   `json:"repoURL"`
	DrySHA   string   `json:"drySha"`
	Commands []string `json:"commands"`
}

// TODO: make this configurable via ConfigMap.
var manifestHydrationReadmeTemplate = `
# Manifest Hydration

To hydrate the manifests in this repository, run the following commands:

` + "```shell\n" + `
git clone {{ .RepoURL }}
# cd into the cloned directory
git checkout {{ .DrySHA }}
{{ range $command := .Commands -}}
{{ $command }}
{{ end -}}` + "```"
