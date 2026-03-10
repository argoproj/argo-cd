package service

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/notification/expression/shared"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

type Service interface {
	GetCommitMetadata(ctx context.Context, repoURL string, commitSHA string, project string) (*shared.CommitMetadata, error)
	GetAppDetails(ctx context.Context, app *v1alpha1.Application, sourceIndex int) (*shared.AppDetail, error)
	GetAppProject(ctx context.Context, projectName string, namespace string) (*unstructured.Unstructured, error)
}

func NewArgoCDService(clientset kubernetes.Interface, dynamicClient dynamic.Interface, namespace string, repoClientset apiclient.Clientset) (*argoCDService, error) {
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
	return &argoCDService{clientset: clientset, dynamicClient: dynamicClient, settingsMgr: settingsMgr, namespace: namespace, repoServerClient: repoClient, dispose: dispose}, nil
}

type argoCDService struct {
	clientset        kubernetes.Interface
	dynamicClient    dynamic.Interface
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

func (svc *argoCDService) GetAppDetails(ctx context.Context, app *v1alpha1.Application, sourceIndex int) (*shared.AppDetail, error) {
	sources := app.Spec.GetSources()
	if len(sources) == 0 {
		return nil, fmt.Errorf("application has no sources")
	}
	if sourceIndex < 0 || sourceIndex >= len(sources) {
		return nil, fmt.Errorf("source index %d out of range (application has %d sources)", sourceIndex, len(sources))
	}
	appSource := app.Spec.GetSourcePtrByIndex(sourceIndex)
	if appSource == nil {
		return nil, fmt.Errorf("application source at index %d is nil", sourceIndex)
	}

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

	var refSources v1alpha1.RefTargetRevisionMapping
	if app.Spec.HasMultipleSources() {
		// Pass empty revisions slice so each ref source uses its spec.TargetRevision.
		// Unlike sync operations, we have no resolved revisions to override with.
		refSources, err = argo.GetRefSources(ctx, app.Spec.Sources, app.Spec.Project, argocdDB.GetRepository, []string{})
		if err != nil {
			return nil, fmt.Errorf("failed to get ref sources: %w", err)
		}
	}

	appDetail, err := svc.repoServerClient.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		AppName:          app.Name,
		Repo:             repo,
		Source:           appSource,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeOptions,
		HelmOptions:      helmOptions,
		RefSources:       refSources,
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

func (svc *argoCDService) GetAppProject(ctx context.Context, projectName string, namespace string) (*unstructured.Unstructured, error) {
	if projectName == "" {
		projectName = "default"
	}

	resource := v1alpha1.AppProjectSchemaGroupVersionKind.GroupVersion().WithResource(application.AppProjectPlural)
	obj, err := svc.dynamicClient.Resource(resource).Namespace(namespace).Get(ctx, projectName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot get application project %w", err)
	}

	return obj, nil
}

func (svc *argoCDService) Close() {
	svc.dispose()
}
