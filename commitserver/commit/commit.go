package commit

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/argoproj/pkg/v2/sync"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/gpgsign"
	"github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/io/files"
)

const (
	NoteNamespace = "hydrator.metadata" // NoteNamespace is the custom git notes namespace used by the hydrator to store and retrieve commit-related metadata.
	ManifestYaml  = "manifest.yaml"     // ManifestYaml constant for the manifest yaml
)

// Service is the service that handles commit requests.
type Service struct {
	metricsServer     *metrics.Server
	repoClientFactory RepoClientFactory
	// signingConfig is non-nil when the commit server has been configured to
	// sign hydrated commits. When set, every commit produced by this service
	// is GPG-signed with the configured key, locally verified, and only then
	// pushed — there is no unsigned fallback.
	signingConfig *gpgsign.Config
	// branchLock serializes the read-modify-write of a given target branch so
	// concurrent requests for the same branch don't race each other. Keyed by
	// repo URL + target branch.
	branchLock sync.KeyLock
}

// NewService returns a new instance of the commit service. When signingConfig
// is non-nil, hydrated commits are GPG-signed with the configured key.
func NewService(gitCredsStore git.CredsStore, metricsServer *metrics.Server, signingConfig *gpgsign.Config) *Service {
	return &Service{
		metricsServer:     metricsServer,
		repoClientFactory: NewRepoClientFactory(gitCredsStore, metricsServer),
		signingConfig:     signingConfig,
		branchLock:        sync.NewKeyLock(),
	}
}

// CommitNote represents the structure of the git note associated with a hydrated commit.
// This struct is used to serialize/deserialize commit metadata (such as the dry run SHA)
// stored in the custom note namespace by the hydrator.
type CommitNote struct {
	DrySHA string `json:"drySha"` // SHA of original commit that triggerd the hydrator
}

// CommitHydratedManifests handles a commit request. It clones the repository, checks out the sync branch, checks out
// the target branch, clears the repository contents, writes the manifests to the repository, commits the changes, and
// pushes the changes. It returns the hydrated revision SHA and an error if one occurred.
func (s *Service) CommitHydratedManifests(ctx context.Context, r *apiclient.CommitHydratedManifestsRequest) (*apiclient.CommitHydratedManifestsResponse, error) {
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

	out, sha, err := s.handleCommitRequest(ctx, logCtx, r)
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
func (s *Service) handleCommitRequest(ctx context.Context, logCtx *log.Entry, r *apiclient.CommitHydratedManifestsRequest) (string, string, error) {
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

	// Serialize same-branch requests so they don't clone the same base and then
	// race on the push (the loser hits a non-fast-forward rejection after a
	// wasted sign cycle). Per-replica only; cross-replica safety still relies on
	// git rejecting non-fast-forward pushes. The NUL separator can't appear in
	// either field, so distinct (repo, branch) pairs never share a key.
	branchKey := r.Repo.Repo + "\x00" + r.TargetBranch
	s.branchLock.Lock(branchKey)
	defer s.branchLock.Unlock(branchKey)

	logCtx = logCtx.WithField("repo", r.Repo.Repo)
	logCtx.Debug("Initiating git client")
	gitClient, dirPath, cleanup, err := s.initGitClient(ctx, logCtx, r)
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
	out, err = gitClient.CheckoutOrOrphan(ctx, r.SyncBranch, false)
	if err != nil {
		return out, "", fmt.Errorf("failed to checkout sync branch: %w", err)
	}

	logCtx.Debugf("Checking out target branch %s", r.TargetBranch)
	out, err = gitClient.CheckoutOrNew(ctx, r.TargetBranch, r.SyncBranch, false)
	if err != nil {
		return out, "", fmt.Errorf("failed to checkout target branch: %w", err)
	}

	hydratedSha, err := gitClient.CommitSHA(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get commit SHA: %w", err)
	}

	/* git note changes
	1. Get the git note
	2. If found, short-circuit, log a warn and return
	3. If not, get the last manifest from git  for every path, compare it with the hydrated manifest
	3a. If manifest has no changes, continue.. no need to commit it
	3b. Else, hydrate the manifest.
	3c. Push the updated note
	*/
	isHydrated, err := IsHydrated(ctx, gitClient, r.DrySha, hydratedSha)
	if err != nil {
		return "", "", fmt.Errorf("failed to get notes from git %w", err)
	}
	// short-circuit if already hydrated
	if isHydrated {
		logCtx.Debugf("this dry sha %s is already hydrated", r.DrySha)
		return "", hydratedSha, nil
	}

	logCtx.Debug("Writing manifests")
	shouldCommit, err := WriteForPaths(ctx, root, r.Repo.Repo, r.DrySha, r.DryCommitMetadata, r.Paths, gitClient, r.ReadmeMessage)
	if err != nil {
		return "", "", fmt.Errorf("failed to write manifests: %w", err)
	}
	if !shouldCommit {
		// Manifests did not change, so we don't need to create a new commit.
		// Add a git note to track that this dry SHA has been processed, and return the existing hydrated SHA.
		logCtx.Debug("Adding commit note")
		err = AddNote(ctx, gitClient, r.DrySha, hydratedSha)
		if err != nil {
			return "", "", fmt.Errorf("failed to add commit note: %w", err)
		}
		return "", hydratedSha, nil
	}
	logCtx.Debug("Committing changes")
	signingKeyID := ""
	if s.signingConfig != nil {
		signingKeyID = s.signingConfig.KeyID
	}
	out, err = gitClient.Commit(r.CommitMessage, signingKeyID)
	if err != nil {
		if s.signingConfig != nil {
			s.metricsServer.IncSigningFailure(r.Repo.Repo, metrics.SigningFailureReasonCommit)
		}
		return out, "", fmt.Errorf("failed to commit: %w", err)
	}

	if s.signingConfig != nil {
		// Verify locally that the freshly-created commit is signed by the
		// expected key before pushing. If anything looks off, we fail here —
		// we must never push an unsigned (or wrongly-signed) hydrated commit.
		status, keyID, sigErr := gitClient.HeadSignatureStatus()
		if sigErr != nil {
			s.metricsServer.IncSigningFailure(r.Repo.Repo, metrics.SigningFailureReasonVerify)
			return out, "", fmt.Errorf("failed to verify signature of hydrated commit: %w", sigErr)
		}
		if status != git.SignatureStatusGood && status != git.SignatureStatusGoodUnknownTrust {
			s.metricsServer.IncSigningFailure(r.Repo.Repo, metrics.SigningFailureReasonBadStatus)
			return out, "", fmt.Errorf("hydrated commit signature status %q is not acceptable (want %q or %q)", status, git.SignatureStatusGood, git.SignatureStatusGoodUnknownTrust)
		}
		if !s.signingConfig.MatchesSigningKey(keyID) {
			s.metricsServer.IncSigningFailure(r.Repo.Repo, metrics.SigningFailureReasonWrongKey)
			return out, "", fmt.Errorf("hydrated commit signed by key %q, expected one of %v", keyID, s.signingConfig.SigningKeyIDs)
		}
		logCtx.WithField("signingKey", keyID).Debug("Hydrated commit signature verified")
	}

	logCtx.Debug("Pushing changes")
	if out, err = gitClient.Push(r.TargetBranch); err != nil {
		return out, "", fmt.Errorf("failed to push: %w", err)
	}

	logCtx.Debug("Getting commit SHA")
	sha, err := gitClient.CommitSHA(ctx)
	if err != nil {
		return "", "", fmt.Errorf("failed to get commit SHA: %w", err)
	}
	// add the commit note
	logCtx.Debug("Adding commit note")
	err = AddNote(ctx, gitClient, r.DrySha, sha)
	if err != nil {
		return "", "", fmt.Errorf("failed to add commit note: %w", err)
	}
	return "", sha, nil
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
	err = gitClient.Fetch(ctx, "", 0)
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
	// Use author name and email from request, defaulting to "Argo CD" if not provided
	authorName := r.AuthorName
	if authorName == "" {
		authorName = "Argo CD"
	}
	authorEmail := r.AuthorEmail
	if authorEmail == "" {
		authorEmail = "argo-cd@example.com"
	}

	logCtx.Debugf("Author config: request name='%s', request email='%s', final name='%s', final email='%s'",
		r.AuthorName, r.AuthorEmail, authorName, authorEmail)

	logCtx.Debugf("Setting author %s <%s>", authorName, authorEmail)
	_, err = gitClient.SetAuthor(ctx, authorName, authorEmail)
	if err != nil {
		cleanupOrLog()
		return nil, "", nil, fmt.Errorf("failed to set author: %w", err)
	}

	return gitClient, dirPath, cleanupOrLog, nil
}
