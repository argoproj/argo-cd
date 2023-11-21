package reporter

import (
	"context"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

type serverClient struct {
	applicationServiceClient applicationpkg.ApplicationServiceClient
}

type ServerClient interface {
	GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error)
	GetAppResources(ctx context.Context, a *appv1.Application) (*appv1.ApplicationTree, error)
	Get(ctx context.Context, q *application.ApplicationQuery) (*appv1.Application, error)
	GetResource(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ApplicationResourceResponse, error)
	RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*appv1.RevisionMetadata, error)
}

func NewServerClient(applicationServiceClient applicationpkg.ApplicationServiceClient) ServerClient {
	return &serverClient{
		applicationServiceClient: applicationServiceClient,
	}
}

func (sc *serverClient) GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error) {
	return sc.applicationServiceClient.GetManifests(ctx, q)
}

func (sc *serverClient) GetAppResources(ctx context.Context, a *appv1.Application) (*appv1.ApplicationTree, error) {
	return nil, nil
}

func (sc *serverClient) GetResource(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ApplicationResourceResponse, error) {
	return sc.GetResource(ctx, q)
}

func (sc *serverClient) Get(ctx context.Context, q *application.ApplicationQuery) (*appv1.Application, error) {
	return sc.Get(ctx, q)
}

func (sc *serverClient) RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*appv1.RevisionMetadata, error) {
	return sc.RevisionMetadata(ctx, q)
}
