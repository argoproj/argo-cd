package service

import (
	"context"

	"github.com/argoproj/argo-cd/v3/util/notification/expression/shared"

	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

type Service interface {
	GetCommitMetadata(ctx context.Context, repoURL string, commitSHA string, project string) (*shared.CommitMetadata, error)
	GetAppDetails(ctx context.Context, app *v1alpha1.Application) (*shared.AppDetail, error)
	GetCommitAuthorsBetween(ctx context.Context, repoURL string, fromRevision string, toRevision string, project string) ([]string, error)
}

func NewArgoCDService(clientset kubernetes.Interface, namespace string, repoClientset apiclient.Clientset) (*argoCDService, error) {
	ctx, cancel := context.WithCancel(context.Background())
	settingsMgr := settings.NewSettingsManager(ctx, clientset, namespace)
	closer, repoClient, err := repoClientset.NewRepoServerClient()
	if err != nil {
		cancel()
		return nil, err
	}

	dispose := func() {
		cancel()
		if err := closer.Close(); err != nil {
			log.Warnf("Failed to close repo server connection: %v", err)
		}
	}
	return &argoCDService{settingsMgr: settingsMgr, namespace: namespace, repoServerClient: repoClient, dispose: dispose}, nil
}

type argoCDService struct {
	clientset        kubernetes.Interface
	namespace        string
	settingsMgr      *settings.SettingsManager
	repoServerClient apiclient.RepoServerServiceClient
	dispose          func()
}

func (svc *argoCDService) GetCommitMetadata(ctx context.Context, repoURL string, commitSHA string, project string) (*shared.CommitMetadata, error) {
	argocdDB := db.NewDB(svc.namespace, svc.settingsMgr, svc.clientset)
	repo, err := argocdDB.GetRepository(ctx, repoURL, project)
	if err != nil {
		return nil, err
	}
	metadata, err := svc.repoServerClient.GetRevisionMetadata(ctx, &apiclient.RepoServerRevisionMetadataRequest{
		Repo:     repo,
		Revision: commitSHA,
	})
	if err != nil {
		return nil, err
	}
	return &shared.CommitMetadata{
		Message: metadata.Message,
		Author:  metadata.Author,
		Date:    metadata.Date.Time,
		Tags:    metadata.Tags,
	}, nil
}

func (svc *argoCDService) GetAppDetails(ctx context.Context, app *v1alpha1.Application) (*shared.AppDetail, error) {
	appSource := app.Spec.GetSourcePtrByIndex(0)

	argocdDB := db.NewDB(svc.namespace, svc.settingsMgr, svc.clientset)
	repo, err := argocdDB.GetRepository(ctx, appSource.RepoURL, app.Spec.Project)
	if err != nil {
		return nil, err
	}
	helmRepos, err := argocdDB.ListHelmRepositories(ctx)
	if err != nil {
		return nil, err
	}
	kustomizeOptions, err := svc.settingsMgr.GetKustomizeSettings()
	if err != nil {
		return nil, err
	}
	helmOptions, err := svc.settingsMgr.GetHelmSettings()
	if err != nil {
		return nil, err
	}
	appDetail, err := svc.repoServerClient.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		AppName:          app.Name,
		Repo:             repo,
		Source:           appSource,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeOptions,
		HelmOptions:      helmOptions,
	})
	if err != nil {
		return nil, err
	}
	var has *shared.CustomHelmAppSpec
	if appDetail.Helm != nil {
		has = &shared.CustomHelmAppSpec{
			HelmAppSpec: apiclient.HelmAppSpec{
				Name:           appDetail.Helm.Name,
				ValueFiles:     appDetail.Helm.ValueFiles,
				Parameters:     appDetail.Helm.Parameters,
				Values:         appDetail.Helm.Values,
				FileParameters: appDetail.Helm.FileParameters,
			},
			HelmParameterOverrides: appSource.Helm.Parameters,
		}
	}
	return &shared.AppDetail{
		Type:      appDetail.Type,
		Helm:      has,
		Kustomize: appDetail.Kustomize,
		Directory: appDetail.Directory,
	}, nil
}

func (svc *argoCDService) GetCommitAuthorsBetween(ctx context.Context, repoURL string, fromRevision string, toRevision string, project string) ([]string, error) {
	// Validate inputs
	if fromRevision == "" || toRevision == "" {
		return []string{}, nil
	}
	if fromRevision == toRevision {
		return []string{}, nil
	}

	argocdDB := db.NewDB(svc.namespace, svc.settingsMgr, svc.clientset)
	repo, err := argocdDB.GetRepository(ctx, repoURL, project)
	if err != nil {
		// Return empty on error to avoid breaking notifications
		log.Debugf("failed to get repository %s in project %s: %v, returning empty authors", repoURL, project, err)
		return []string{}, nil
	}
	response, err := svc.repoServerClient.GetCommitAuthorsBetween(ctx, &apiclient.RepoServerCommitAuthorsRequest{
		Repo:         repo,
		FromRevision: fromRevision,
		ToRevision:   toRevision,
	})
	if err != nil {
		// Return empty on error to avoid breaking notifications
		log.Debugf("failed to get commit authors between %s and %s: %v, returning empty authors", fromRevision, toRevision, err)
		return []string{}, nil
	}
	if response == nil {
		return []string{}, nil
	}
	return response.Authors, nil
}

func (svc *argoCDService) Close() {
	svc.dispose()
}
