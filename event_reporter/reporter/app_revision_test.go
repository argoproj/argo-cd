package reporter

import (
	"context"
	"testing"

	"github.com/argoproj/argo-cd/v2/event_reporter/application/mocks"
	"github.com/argoproj/argo-cd/v2/event_reporter/metrics"
	"github.com/argoproj/argo-cd/v2/event_reporter/utils"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/cache"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetRevisionsDetails(t *testing.T) {
	t.Run("should return revisions for single source app", func(t *testing.T) {
		expectedRevision := "expected-revision"
		expectedResult := []*utils.RevisionWithMetadata{{
			Revision: expectedRevision,
			Metadata: &v1alpha1.RevisionMetadata{
				Author:  "Test Author",
				Message: "first commit",
			},
		}}

		app := v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://my-site.com",
					TargetRevision: "HEAD",
					Path:           ".",
				},
			},
		}

		appServiceClient := mocks.NewApplicationClient(t)
		project := app.Spec.GetProject()
		sourceIdx1 := int32(0)

		appServiceClient.On("RevisionMetadata", mock.Anything, &application.RevisionMetadataQuery{
			Name:         &app.Name,
			AppNamespace: &app.Namespace,
			Revision:     &expectedResult[0].Revision,
			Project:      &project,
			SourceIndex:  &sourceIdx1,
		}).Return(expectedResult[0].Metadata, nil)

		reporter := &applicationEventReporter{
			&cache.Cache{},
			&MockCodefreshClient{},
			newAppLister(),
			appServiceClient,
			&metrics.MetricsServer{},
			fakeArgoDb(),
			"0.0.1",
		}

		result, _ := reporter.getRevisionsDetails(context.Background(), &app, []string{expectedRevision})

		assert.Equal(t, expectedResult, result)
	})

	t.Run("should return revisions for multi sourced apps", func(t *testing.T) {
		expectedRevision1 := "expected-revision-1"
		expectedRevision2 := "expected-revision-2"
		expectedResult := []*utils.RevisionWithMetadata{{
			Revision: expectedRevision1,
			Metadata: &v1alpha1.RevisionMetadata{
				Author:  "Repo1 Author",
				Message: "first commit repo 1",
			},
		}, {
			Revision: expectedRevision2,
			Metadata: &v1alpha1.RevisionMetadata{
				Author:  "Repo2 Author",
				Message: "first commit repo 2",
			},
		}}

		app := v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				Sources: []v1alpha1.ApplicationSource{{
					RepoURL:        "https://my-site.com/repo-1",
					TargetRevision: "branch1",
					Path:           ".",
				}, {
					RepoURL:        "https://my-site.com/repo-2",
					TargetRevision: "branch2",
					Path:           ".",
				}},
			},
		}

		project := app.Spec.GetProject()

		appServiceClient := mocks.NewApplicationClient(t)
		sourceIdx1 := int32(0)
		sourceIdx2 := int32(1)
		appServiceClient.On("RevisionMetadata", mock.Anything, &application.RevisionMetadataQuery{
			Name:         &app.Name,
			AppNamespace: &app.Namespace,
			Revision:     &expectedRevision1,
			Project:      &project,
			SourceIndex:  &sourceIdx1,
		}).Return(expectedResult[0].Metadata, nil)
		appServiceClient.On("RevisionMetadata", mock.Anything, &application.RevisionMetadataQuery{
			Name:         &app.Name,
			AppNamespace: &app.Namespace,
			Revision:     &expectedRevision2,
			Project:      &project,
			SourceIndex:  &sourceIdx2,
		}).Return(expectedResult[1].Metadata, nil)

		reporter := &applicationEventReporter{
			&cache.Cache{},
			&MockCodefreshClient{},
			newAppLister(),
			appServiceClient,
			&metrics.MetricsServer{},
			fakeArgoDb(),
			"0.0.1",
		}

		result, _ := reporter.getRevisionsDetails(context.Background(), &app, []string{expectedRevision1, expectedRevision2})

		assert.Equal(t, expectedResult, result)
	})

	t.Run("should return only revision because of helm single source app", func(t *testing.T) {
		expectedRevision := "expected-revision"
		expectedResult := []*utils.RevisionWithMetadata{{
			Revision: expectedRevision,
		}}

		app := v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				Source: &v1alpha1.ApplicationSource{
					RepoURL:        "https://my-site.com",
					TargetRevision: "HEAD",
					Path:           ".",
				},
			},
		}

		appServiceClient := mocks.NewApplicationClient(t)
		project := app.Spec.GetProject()
		sourceIdx1 := int32(0)

		appServiceClient.On("RevisionMetadata", mock.Anything, &application.RevisionMetadataQuery{
			Name:         &app.Name,
			AppNamespace: &app.Namespace,
			Revision:     &expectedResult[0].Revision,
			Project:      &project,
			SourceIndex:  &sourceIdx1,
		}).Return(expectedResult[0].Metadata, nil)

		reporter := &applicationEventReporter{
			&cache.Cache{},
			&MockCodefreshClient{},
			newAppLister(),
			appServiceClient,
			&metrics.MetricsServer{},
			fakeArgoDb(),
			"0.0.1",
		}

		result, _ := reporter.getRevisionsDetails(context.Background(), &app, []string{expectedRevision})

		assert.Equal(t, expectedResult, result)
	})
}
