package repository

import (
	"context"
	goio "io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	argopath "github.com/argoproj/argo-cd/v3/util/app/path"
	"github.com/argoproj/argo-cd/v3/util/git"
	"github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/kustomize"

	log "github.com/sirupsen/logrus"
)

func (s *Service) getCacheKeyWithKustomizeComponents(
	revision string,
	repo *v1alpha1.Repository,
	source *v1alpha1.ApplicationSource,
	settings operationSettings,
	gitClient git.Client,
) (string, error) {
	closer, err := s.repoLock.Lock(gitClient.Root(), revision, settings.allowConcurrent, func() (goio.Closer, error) {
		return s.checkoutRevision(gitClient, revision, s.initConstants.SubmoduleEnabled)
	})
	if err != nil {
		return "", err
	}

	defer io.Close(closer)

	appPath, err := argopath.Path(gitClient.Root(), source.Path)
	if err != nil {
		return "", err
	}

	k := kustomize.NewKustomizeApp(gitClient.Root(), appPath, repo.GetGitCreds(s.gitCredsStore), repo.Repo, source.Kustomize.Version, "", "")

	resolveRevisionFunc := func(repoURL, revision string, _ git.Creds) (string, error) {
		cloneRepo := *repo
		cloneRepo.Repo = repoURL
		_, res, err := s.newClientResolveRevision(&cloneRepo, revision)
		return res, err
	}

	return k.GetCacheKeyWithComponents(revision, source.Kustomize, resolveRevisionFunc)
}

func (s *Service) GetChangeRevision(_ context.Context, request *apiclient.ChangeRevisionRequest) (*apiclient.ChangeRevisionResponse, error) {
	logCtx := log.WithFields(log.Fields{"application": request.AppName, "appNamespace": request.Namespace})

	repo := request.GetRepo()
	currentRevision := request.GetCurrentRevision()
	previousRevision := request.GetPreviousRevision()
	refreshPaths := request.GetPaths()

	if repo == nil {
		return nil, status.Error(codes.InvalidArgument, "must pass a valid repo")
	}

	if len(refreshPaths) == 0 {
		return nil, status.Error(codes.InvalidArgument, "must pass a refresh path")
	}

	gitClientOpts := git.WithCache(s.cache, true)
	gitClient, revision, err := s.newClientResolveRevision(repo, currentRevision, gitClientOpts)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to resolve git revision %s: %v", revision, err)
	}

	s.metricsServer.IncPendingRepoRequest(repo.Repo)
	defer s.metricsServer.DecPendingRepoRequest(repo.Repo)

	closer, err := s.repoLock.Lock(gitClient.Root(), revision, true, func() (goio.Closer, error) {
		return s.checkoutRevision(gitClient, revision, false)
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to checkout git repo %s with revision %s: %v", repo.Repo, revision, err)
	}
	defer io.Close(closer)
	revisions, err := gitClient.ListRevisions(previousRevision, revision)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get revisions %s..%s", previousRevision, revision)
	}
	for _, rev := range revisions {
		files, err := gitClient.DiffTree(rev)
		if err != nil {
			continue
		}
		changedFiles := argopath.AppFilesHaveChanged(refreshPaths, files)
		if changedFiles {
			logCtx.Debugf("changes found for application %s in repo %s from revision %s to revision %s", request.AppName, repo.Repo, previousRevision, revision)
			return &apiclient.ChangeRevisionResponse{
				Revision: rev,
			}, nil
		}
	}

	logCtx.Debugf("changes not found for application %s in repo %s from revision %s to revision %s", request.AppName, repo.Repo, previousRevision, revision)
	return &apiclient.ChangeRevisionResponse{}, nil
}
