package reporter

import (
	"context"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

type serverClient struct {
}

type ServerClient interface {
	GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error)
	GetAppResources(ctx context.Context, a *appv1.Application) (*appv1.ApplicationTree, error)
	Get(ctx context.Context, q *application.ApplicationQuery) (*appv1.Application, error)
	GetResource(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ApplicationResourceResponse, error)
	RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*appv1.RevisionMetadata, error)
}

func NewServerClient() ServerClient {
	return &serverClient{}
}

func (sc *serverClient) GetManifests(ctx context.Context, q *application.ApplicationManifestQuery) (*apiclient.ManifestResponse, error) {
	return nil, nil
}

func (sc *serverClient) GetAppResources(ctx context.Context, a *appv1.Application) (*appv1.ApplicationTree, error) {
	return nil, nil
}

func (sc *serverClient) GetResource(ctx context.Context, q *application.ApplicationResourceRequest) (*application.ApplicationResourceResponse, error) {
	return nil, nil
}

func (sc *serverClient) Get(ctx context.Context, q *application.ApplicationQuery) (*appv1.Application, error) {
	return nil, nil
}

func (sc *serverClient) RevisionMetadata(ctx context.Context, q *application.RevisionMetadataQuery) (*appv1.RevisionMetadata, error) {
	return nil, nil
}
