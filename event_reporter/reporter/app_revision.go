package reporter

import (
	"context"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func (s *applicationEventReporter) getApplicationRevisionDetails(ctx context.Context, a *appv1.Application, revision string) (*appv1.RevisionMetadata, error) {
	project := a.Spec.GetProject()
	return s.applicationServiceClient.RevisionMetadata(ctx, &application.RevisionMetadataQuery{
		Name:         &a.Name,
		AppNamespace: &a.Namespace,
		Revision:     &revision,
		Project:      &project,
	})
}
