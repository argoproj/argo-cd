package generators

import (
	"context"
	environment "github.com/argoproj/argo-cd/v2/applicationset/services/gitlab_environments"
	"testing"

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
)

func TestGitlabEnvironmentGenerateParams(t *testing.T) {
	ctx := context.Background()
	gitlabEnvCases := []struct {
		getSvcProviderFunc func(context.Context, *argoprojiov1alpha1.GitlabEnvironmentGenerator, *argoprojiov1alpha1.ApplicationSet) (environment.EnvironmentService, error)
		expected           []map[string]interface{}
		expectedErr        error
		applicationSet     argoprojiov1alpha1.ApplicationSet
	}{
		{
			getSvcProviderFunc: func(context.Context, *argoprojiov1alpha1.GitlabEnvironmentGenerator, *argoprojiov1alpha1.ApplicationSet) (environment.EnvironmentService, error) {
				return environment.NewFakeService(
					ctx,
					[]*environment.Environment{
						{
							ID:          678,
							Name:        "review/add-new-line",
							ExternalURL: "www.review-great-application.com",
							State:       "available",
							Slug:        "review-fix-foo-dfjre3",
							Tier:        "standard",
						}},
					nil,
				)
			},
			expected: []map[string]interface{}{
				{
					"id":               678,
					"name":             "review/add-new-line",
					"external_url":     "www.review-great-application.com",
					"state":            "available",
					"environment_slug": "review-fix-foo-dfjre3",
					"tier":             "standard",
				},
			},
			expectedErr: nil,
		},
	}

	for _, c := range gitlabEnvCases {
		gitlabEnvGenerator := GitlabEnvironmentGenerator{
			getServiceProviderFunc: c.getSvcProviderFunc,
		}
		config := argoprojiov1alpha1.ApplicationSetGenerator{
			GitlabEnvironment: &argoprojiov1alpha1.GitlabEnvironmentGenerator{},
		}
		got, gotErr := gitlabEnvGenerator.GenerateParams(&config, &c.applicationSet)

		assert.Equal(t, c.expectedErr, gotErr)
		assert.ElementsMatch(t, c.expected, got)
	}
}
