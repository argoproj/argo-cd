package application

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/app/path"
	ioutil "github.com/argoproj/argo-cd/v3/util/io"
)

const (
	projectEntity     = "project"
	sourceEntity      = "source"
	destinationEntity = "destination"
)

func (s *Server) GetChangeRevision(ctx context.Context, in *application.ChangeRevisionRequest) (*application.ChangeRevisionResponse, error) {
	app, err := s.appLister.Applications(in.GetNamespace()).Get(in.GetAppName())
	if err != nil {
		return nil, err
	}

	val, ok := app.Annotations[appv1.AnnotationKeyManifestGeneratePaths]
	if !ok || val == "" {
		return nil, status.Errorf(codes.FailedPrecondition, "manifest generation paths not set")
	}

	repo, err := s.db.GetRepository(ctx, app.Spec.GetSource().RepoURL, app.Spec.Project)
	if err != nil {
		return nil, fmt.Errorf("error getting repository: %w", err)
	}

	closer, client, err := s.repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error creating repo server client: %w", err)
	}
	defer ioutil.Close(closer)

	response, err := client.GetChangeRevision(ctx, &apiclient.ChangeRevisionRequest{
		AppName:          in.GetAppName(),
		Namespace:        in.GetNamespace(),
		CurrentRevision:  in.GetCurrentRevision(),
		PreviousRevision: in.GetPreviousRevision(),
		Paths:            path.GetAppRefreshPaths(app),
		Repo:             repo,
	})
	if err != nil {
		return nil, fmt.Errorf("error getting change revision: %w", err)
	}

	return &application.ChangeRevisionResponse{
		Revision: ptr.To(response.Revision),
	}, nil
}
