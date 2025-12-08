package generators

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pullrequest "github.com/argoproj/argo-cd/v3/applicationset/services/pull_request"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestPullRequestGithubGenerateParams(t *testing.T) {
	ctx := t.Context()
	cases := []struct {
		selectFunc                  func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error)
		values                      map[string]string
		expected                    []map[string]any
		expectedErr                 error
		applicationSet              argoprojiov1alpha1.ApplicationSet
		continueOnRepoNotFoundError bool
	}{
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Title:        "title1",
							Branch:       "branch1",
							TargetBranch: "master",
							HeadSHA:      "089d92cbf9ff857a39e6feccd32798ca700fb958",
							Author:       "testName",
						},
					},
					nil,
				)
			},
			expected: []map[string]any{
				{
					"number":             "1",
					"title":              "title1",
					"branch":             "branch1",
					"branch_slug":        "branch1",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "089d92cbf9ff857a39e6feccd32798ca700fb958",
					"head_short_sha":     "089d92cb",
					"head_short_sha_7":   "089d92c",
					"author":             "testName",
				},
			},
			expectedErr: nil,
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       2,
							Title:        "title2",
							Branch:       "feat/areally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
							TargetBranch: "feat/anotherreally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
							HeadSHA:      "9b34ff5bd418e57d58891eb0aa0728043ca1e8be",
							Author:       "testName",
						},
					},
					nil,
				)
			},
			expected: []map[string]any{
				{
					"number":             "2",
					"title":              "title2",
					"branch":             "feat/areally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
					"branch_slug":        "feat-areally-long-pull-request-name-to-test-argo",
					"target_branch":      "feat/anotherreally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
					"target_branch_slug": "feat-anotherreally-long-pull-request-name-to-test",
					"head_sha":           "9b34ff5bd418e57d58891eb0aa0728043ca1e8be",
					"head_short_sha":     "9b34ff5b",
					"head_short_sha_7":   "9b34ff5",
					"author":             "testName",
				},
			},
			expectedErr: nil,
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Title:        "title1",
							Branch:       "a-very-short-sha",
							TargetBranch: "master",
							HeadSHA:      "abcd",
							Author:       "testName",
						},
					},
					nil,
				)
			},
			expected: []map[string]any{
				{
					"number":             "1",
					"title":              "title1",
					"branch":             "a-very-short-sha",
					"branch_slug":        "a-very-short-sha",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "abcd",
					"head_short_sha":     "abcd",
					"head_short_sha_7":   "abcd",
					"author":             "testName",
				},
			},
			expectedErr: nil,
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Title:        "title1",
							Branch:       "my_branch",
							TargetBranch: "master",
							HeadSHA:      "abcd",
							Author:       "testName",
						},
					},
					nil,
				)
			},
			values: map[string]string{
				"foo":       "bar",
				"pr_branch": "{{ branch }}",
			},
			expected: []map[string]any{
				{
					"number":             "1",
					"title":              "title1",
					"branch":             "my_branch",
					"branch_slug":        "my-branch",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "abcd",
					"head_short_sha":     "abcd",
					"head_short_sha_7":   "abcd",
					"author":             "testName",
					"values.foo":         "bar",
					"values.pr_branch":   "my_branch",
				},
			},
			expectedErr: nil,
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					nil,
					errors.New("fake error"),
				)
			},
			expected:    nil,
			expectedErr: errors.New("error listing repos: fake error"),
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					nil,
					pullrequest.NewRepositoryNotFoundError(errors.New("repository not found")),
				)
			},
			expected:                    []map[string]any{},
			expectedErr:                 nil,
			continueOnRepoNotFoundError: true,
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					nil,
					pullrequest.NewRepositoryNotFoundError(errors.New("repository not found")),
				)
			},
			expected:                    nil,
			expectedErr:                 errors.New("error listing repos: repository not found"),
			continueOnRepoNotFoundError: false,
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Title:        "title1",
							Branch:       "branch1",
							TargetBranch: "master",
							HeadSHA:      "089d92cbf9ff857a39e6feccd32798ca700fb958",
							Labels:       []string{"preview"},
							Author:       "testName",
						},
					},
					nil,
				)
			},
			expected: []map[string]any{
				{
					"number":             "1",
					"title":              "title1",
					"branch":             "branch1",
					"branch_slug":        "branch1",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "089d92cbf9ff857a39e6feccd32798ca700fb958",
					"head_short_sha":     "089d92cb",
					"head_short_sha_7":   "089d92c",
					"labels":             []string{"preview"},
					"author":             "testName",
				},
			},
			expectedErr: nil,
			applicationSet: argoprojiov1alpha1.ApplicationSet{
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					// Application set is using Go Template.
					GoTemplate: true,
				},
			},
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Title:        "title1",
							Branch:       "branch1",
							TargetBranch: "master",
							HeadSHA:      "089d92cbf9ff857a39e6feccd32798ca700fb958",
							Labels:       []string{"preview"},
							Author:       "testName",
						},
					},
					nil,
				)
			},
			expected: []map[string]any{
				{
					"number":             "1",
					"title":              "title1",
					"branch":             "branch1",
					"branch_slug":        "branch1",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "089d92cbf9ff857a39e6feccd32798ca700fb958",
					"head_short_sha":     "089d92cb",
					"head_short_sha_7":   "089d92c",
					"author":             "testName",
				},
			},
			expectedErr: nil,
			applicationSet: argoprojiov1alpha1.ApplicationSet{
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					// Application set is using fasttemplate.
					GoTemplate: false,
				},
			},
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Title:        "title1",
							Branch:       "my_branch",
							TargetBranch: "master",
							HeadSHA:      "abcd",
							Author:       "testName",
							Labels:       []string{"preview", "preview:team1"},
						},
					},
					nil,
				)
			},
			values: map[string]string{
				"preview_env": "{{ regexFind \"(team1|team2)\" (.labels | join \",\") }}",
			},
			expected: []map[string]any{
				{
					"number":             "1",
					"title":              "title1",
					"branch":             "my_branch",
					"branch_slug":        "my-branch",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "abcd",
					"head_short_sha":     "abcd",
					"head_short_sha_7":   "abcd",
					"author":             "testName",
					"labels":             []string{"preview", "preview:team1"},
					"values":             map[string]string{"preview_env": "team1"},
				},
			},
			expectedErr: nil,
			applicationSet: argoprojiov1alpha1.ApplicationSet{
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					// Application set is using fasttemplate.
					GoTemplate: true,
				},
			},
		},
	}

	for _, c := range cases {
		gen := PullRequestGenerator{
			selectServiceProviderFunc: c.selectFunc,
		}
		generatorConfig := argoprojiov1alpha1.ApplicationSetGenerator{
			PullRequest: &argoprojiov1alpha1.PullRequestGenerator{
				Values:                      c.values,
				ContinueOnRepoNotFoundError: c.continueOnRepoNotFoundError,
			},
		}

		got, gotErr := gen.GenerateParams(&generatorConfig, &c.applicationSet, nil)
		if c.expectedErr != nil {
			require.EqualError(t, gotErr, c.expectedErr.Error())
		} else {
			require.NoError(t, gotErr)
		}
		assert.ElementsMatch(t, c.expected, got)
	}
}

func TestAllowedSCMProviderPullRequest(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name           string
		providerConfig *argoprojiov1alpha1.PullRequestGenerator
	}{
		{
			name: "Error Github",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				Github: &argoprojiov1alpha1.PullRequestGeneratorGithub{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
		{
			name: "Error Gitlab",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				GitLab: &argoprojiov1alpha1.PullRequestGeneratorGitLab{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
		{
			name: "Error Gitea",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				Gitea: &argoprojiov1alpha1.PullRequestGeneratorGitea{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
		{
			name: "Error Bitbucket",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				BitbucketServer: &argoprojiov1alpha1.PullRequestGeneratorBitbucketServer{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			pullRequestGenerator := NewPullRequestGenerator(nil, NewSCMConfig("", []string{
				"github.myorg.com",
				"gitlab.myorg.com",
				"gitea.myorg.com",
				"bitbucket.myorg.com",
				"azuredevops.myorg.com",
			}, true, true, nil, true))

			applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "set",
				},
				Spec: argoprojiov1alpha1.ApplicationSetSpec{
					Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
						PullRequest: testCaseCopy.providerConfig,
					}},
				},
			}

			_, err := pullRequestGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, nil)

			require.Error(t, err, "Must return an error")
			var expectedError ErrDisallowedSCMProvider
			assert.ErrorAs(t, err, &expectedError)
		})
	}
}

func TestSCMProviderDisabled_PRGenerator(t *testing.T) {
	generator := NewPullRequestGenerator(nil, NewSCMConfig("", []string{}, false, true, nil, true))

	applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "set",
		},
		Spec: argoprojiov1alpha1.ApplicationSetSpec{
			Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
				PullRequest: &argoprojiov1alpha1.PullRequestGenerator{
					Github: &argoprojiov1alpha1.PullRequestGeneratorGithub{
						API: "https://myservice.mynamespace.svc.cluster.local",
					},
				},
			}},
		},
	}

	_, err := generator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, nil)
	assert.ErrorIs(t, err, ErrSCMProvidersDisabled)
}

func TestAzureDevOpsServiceSelection(t *testing.T) {
	t.Parallel()

	t.Run("AzureDevOps with WorkloadIdentity", func(t *testing.T) {
		t.Parallel()
		client := fake.NewClientBuilder().Build()
		generator := NewPullRequestGenerator(client, NewSCMConfig("", []string{}, true, true, nil, true))

		applicationSetInfo := &argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset",
				Namespace: "test-namespace",
			},
		}

		generatorConfig := &argoprojiov1alpha1.PullRequestGenerator{
			AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
				API:                 "https://dev.azure.com",
				Organization:        "test-org",
				Project:             "test-project",
				Repo:                "test-repo",
				UseWorkloadIdentity: true,
				Labels:              []string{"test-label"},
			},
		}

		ctx := context.Background()
		service, err := generator.(*PullRequestGenerator).selectServiceProvider(ctx, generatorConfig, applicationSetInfo)

		require.NoError(t, err)
		assert.NotNil(t, service)

		// Verify it's an AzureDevOpsService
		azureService, ok := service.(*pullrequest.AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.NotNil(t, azureService)
	})

	t.Run("AzureDevOps with Token", func(t *testing.T) {
		t.Parallel()
		// Create a secret for the token
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "azure-token",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "scm-creds",
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-azure-devops-token"),
			},
		}

		client := fake.NewClientBuilder().WithObjects(secret).Build()
		generator := NewPullRequestGenerator(client, NewSCMConfig("", []string{}, true, true, nil, true))

		applicationSetInfo := &argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset",
				Namespace: "test-namespace",
			},
		}

		generatorConfig := &argoprojiov1alpha1.PullRequestGenerator{
			AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
				API:                 "https://dev.azure.com",
				Organization:        "test-org",
				Project:             "test-project",
				Repo:                "test-repo",
				UseWorkloadIdentity: false,
				TokenRef: &argoprojiov1alpha1.SecretRef{
					SecretName: "azure-token",
					Key:        "token",
				},
				Labels: []string{"test-label"},
			},
		}

		ctx := context.Background()
		service, err := generator.(*PullRequestGenerator).selectServiceProvider(ctx, generatorConfig, applicationSetInfo)

		require.NoError(t, err)
		assert.NotNil(t, service)

		// Verify it's an AzureDevOpsService
		azureService, ok := service.(*pullrequest.AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.NotNil(t, azureService)
	})

	t.Run("AzureDevOps with custom API URL", func(t *testing.T) {
		t.Parallel()
		// Create a secret for the token
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "azure-token",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "scm-creds",
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-azure-devops-token"),
			},
		}

		client := fake.NewClientBuilder().WithObjects(secret).Build()
		generator := NewPullRequestGenerator(client, NewSCMConfig("", []string{}, true, true, nil, true))

		applicationSetInfo := &argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset",
				Namespace: "test-namespace",
			},
		}

		generatorConfig := &argoprojiov1alpha1.PullRequestGenerator{
			AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
				API:                 "https://custom.azure.com",
				Organization:        "custom-org",
				Project:             "custom-project",
				Repo:                "custom-repo",
				UseWorkloadIdentity: false,
				TokenRef: &argoprojiov1alpha1.SecretRef{
					SecretName: "azure-token",
					Key:        "token",
				},
			},
		}

		ctx := context.Background()
		service, err := generator.(*PullRequestGenerator).selectServiceProvider(ctx, generatorConfig, applicationSetInfo)

		require.NoError(t, err)
		assert.NotNil(t, service)

		// Verify it's an AzureDevOpsService
		azureService, ok := service.(*pullrequest.AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.NotNil(t, azureService)
	})

	t.Run("AzureDevOps token secret not found", func(t *testing.T) {
		t.Parallel()
		client := fake.NewClientBuilder().Build()
		generator := NewPullRequestGenerator(client, NewSCMConfig("", []string{}, true, true, nil, true))

		applicationSetInfo := &argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset",
				Namespace: "test-namespace",
			},
		}

		generatorConfig := &argoprojiov1alpha1.PullRequestGenerator{
			AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
				API:                 "https://dev.azure.com",
				Organization:        "test-org",
				Project:             "test-project",
				Repo:                "test-repo",
				UseWorkloadIdentity: false,
				TokenRef: &argoprojiov1alpha1.SecretRef{
					SecretName: "missing-secret",
					Key:        "token",
				},
			},
		}

		ctx := context.Background()
		service, err := generator.(*PullRequestGenerator).selectServiceProvider(ctx, generatorConfig, applicationSetInfo)

		require.Error(t, err)
		assert.Nil(t, service)
		assert.Contains(t, err.Error(), "error fetching Secret token")
	})

	t.Run("AzureDevOps empty token uses anonymous", func(t *testing.T) {
		t.Parallel()
		// Create a secret with an empty token
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "azure-token-empty",
				Namespace: "test-namespace",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "scm-creds",
				},
			},
			Data: map[string][]byte{
				"token": []byte(""),
			},
		}

		client := fake.NewClientBuilder().WithObjects(secret).Build()
		generator := NewPullRequestGenerator(client, NewSCMConfig("", []string{}, true, true, nil, true))

		applicationSetInfo := &argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset",
				Namespace: "test-namespace",
			},
		}

		generatorConfig := &argoprojiov1alpha1.PullRequestGenerator{
			AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
				API:                 "https://dev.azure.com",
				Organization:        "test-org",
				Project:             "test-project",
				Repo:                "test-repo",
				UseWorkloadIdentity: false,
				TokenRef: &argoprojiov1alpha1.SecretRef{
					SecretName: "azure-token-empty",
					Key:        "token",
				},
			},
		}

		ctx := context.Background()
		service, err := generator.(*PullRequestGenerator).selectServiceProvider(ctx, generatorConfig, applicationSetInfo)

		require.NoError(t, err)
		assert.NotNil(t, service)

		// Verify it's an AzureDevOpsService
		azureService, ok := service.(*pullrequest.AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.NotNil(t, azureService)
	})

	t.Run("AzureDevOps with SCM provider disabled", func(t *testing.T) {
		t.Parallel()
		client := fake.NewClientBuilder().Build()
		generator := NewPullRequestGenerator(client, NewSCMConfig("", []string{}, false, true, nil, true))

		applicationSetInfo := &argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-appset",
				Namespace: "test-namespace",
			},
		}

		generatorConfig := &argoprojiov1alpha1.PullRequestGenerator{
			AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
				API:                 "https://dev.azure.com",
				Organization:        "test-org",
				Project:             "test-project",
				Repo:                "test-repo",
				UseWorkloadIdentity: true,
			},
		}

		ctx := context.Background()
		service, err := generator.(*PullRequestGenerator).selectServiceProvider(ctx, generatorConfig, applicationSetInfo)

		require.Error(t, err)
		assert.Nil(t, service)
		assert.ErrorIs(t, err, ErrSCMProvidersDisabled)
	})
}

func TestAzureDevOpsAllowedSCMProvider(t *testing.T) {
	t.Parallel()

	t.Run("Error AzureDevOps not in allowed list", func(t *testing.T) {
		t.Parallel()
		client := fake.NewClientBuilder().Build()
		pullRequestGenerator := NewPullRequestGenerator(client, NewSCMConfig("", []string{
			"github.myorg.com",
			"gitlab.myorg.com",
		}, true, true, nil, true))

		applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "set",
			},
			Spec: argoprojiov1alpha1.ApplicationSetSpec{
				Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
					PullRequest: &argoprojiov1alpha1.PullRequestGenerator{
						AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
							API: "https://dev.azure.com",
						},
					},
				}},
			},
		}

		_, err := pullRequestGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, nil)

		require.Error(t, err, "Must return an error")
		var expectedError ErrDisallowedSCMProvider
		assert.ErrorAs(t, err, &expectedError)
	})

	t.Run("AzureDevOps in allowed list", func(t *testing.T) {
		t.Parallel()
		// Create a secret for the token
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "azure-token",
				Namespace: "default",
				Labels: map[string]string{
					"argocd.argoproj.io/secret-type": "scm-creds",
				},
			},
			Data: map[string][]byte{
				"token": []byte("test-token"),
			},
		}

		client := fake.NewClientBuilder().WithObjects(secret).Build()
		pullRequestGenerator := NewPullRequestGenerator(client, NewSCMConfig("", []string{
			"dev.azure.com",
			"github.myorg.com",
		}, true, true, nil, true))

		applicationSetInfo := argoprojiov1alpha1.ApplicationSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "set",
				Namespace: "default",
			},
			Spec: argoprojiov1alpha1.ApplicationSetSpec{
				Generators: []argoprojiov1alpha1.ApplicationSetGenerator{{
					PullRequest: &argoprojiov1alpha1.PullRequestGenerator{
						AzureDevOps: &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
							API:          "https://dev.azure.com",
							Organization: "test-org",
							Project:      "test-project",
							Repo:         "test-repo",
							TokenRef: &argoprojiov1alpha1.SecretRef{
								SecretName: "azure-token",
								Key:        "token",
							},
						},
					},
				}},
			},
		}

		// Mock the selectServiceProviderFunc to avoid actual Azure DevOps API calls
		generator := pullRequestGenerator.(*PullRequestGenerator)
		generator.selectServiceProviderFunc = func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
			return pullrequest.NewFakeService(
				context.Background(),
				[]*pullrequest.PullRequest{
					{
						Number:       1,
						Title:        "test-pr",
						Branch:       "feature-branch",
						TargetBranch: "main",
						HeadSHA:      "abc123",
						Author:       "testuser",
					},
				},
				nil,
			)
		}

		params, err := pullRequestGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo, nil)

		require.NoError(t, err)
		assert.Len(t, params, 1)
		assert.Equal(t, "1", params[0]["number"])
		assert.Equal(t, "test-pr", params[0]["title"])
	})
}
