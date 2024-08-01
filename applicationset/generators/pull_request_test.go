package generators

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pullrequest "github.com/argoproj/argo-cd/v2/applicationset/services/pull_request"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestPullRequestGithubGenerateParams(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		selectFunc     func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error)
		expected       []map[string]interface{}
		expectedErr    error
		applicationSet argoprojiov1alpha1.ApplicationSet
	}{
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Branch:       "branch1",
							TargetBranch: "master",
							HeadSHA:      "089d92cbf9ff857a39e6feccd32798ca700fb958",
						},
					},
					nil,
				)
			},
			expected: []map[string]interface{}{
				{
					"number":             "1",
					"branch":             "branch1",
					"branch_slug":        "branch1",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "089d92cbf9ff857a39e6feccd32798ca700fb958",
					"head_short_sha":     "089d92cb",
					"head_short_sha_7":   "089d92c",
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
							Branch:       "feat/areally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
							TargetBranch: "feat/anotherreally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
							HeadSHA:      "9b34ff5bd418e57d58891eb0aa0728043ca1e8be",
						},
					},
					nil,
				)
			},
			expected: []map[string]interface{}{
				{
					"number":             "2",
					"branch":             "feat/areally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
					"branch_slug":        "feat-areally-long-pull-request-name-to-test-argo",
					"target_branch":      "feat/anotherreally+long_pull_request_name_to_test_argo_slugification_and_branch_name_shortening_feature",
					"target_branch_slug": "feat-anotherreally-long-pull-request-name-to-test",
					"head_sha":           "9b34ff5bd418e57d58891eb0aa0728043ca1e8be",
					"head_short_sha":     "9b34ff5b",
					"head_short_sha_7":   "9b34ff5",
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
							Branch:       "a-very-short-sha",
							TargetBranch: "master",
							HeadSHA:      "abcd",
						},
					},
					nil,
				)
			},
			expected: []map[string]interface{}{
				{
					"number":             "1",
					"branch":             "a-very-short-sha",
					"branch_slug":        "a-very-short-sha",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "abcd",
					"head_short_sha":     "abcd",
					"head_short_sha_7":   "abcd",
				},
			},
			expectedErr: nil,
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					nil,
					fmt.Errorf("fake error"),
				)
			},
			expected:    nil,
			expectedErr: fmt.Errorf("error listing repos: fake error"),
		},
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						{
							Number:       1,
							Branch:       "branch1",
							TargetBranch: "master",
							HeadSHA:      "089d92cbf9ff857a39e6feccd32798ca700fb958",
							Labels:       []string{"preview"},
						},
					},
					nil,
				)
			},
			expected: []map[string]interface{}{
				{
					"number":             "1",
					"branch":             "branch1",
					"branch_slug":        "branch1",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "089d92cbf9ff857a39e6feccd32798ca700fb958",
					"head_short_sha":     "089d92cb",
					"head_short_sha_7":   "089d92c",
					"labels":             []string{"preview"},
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
							Branch:       "branch1",
							TargetBranch: "master",
							HeadSHA:      "089d92cbf9ff857a39e6feccd32798ca700fb958",
							Labels:       []string{"preview"},
						},
					},
					nil,
				)
			},
			expected: []map[string]interface{}{
				{
					"number":             "1",
					"branch":             "branch1",
					"branch_slug":        "branch1",
					"target_branch":      "master",
					"target_branch_slug": "master",
					"head_sha":           "089d92cbf9ff857a39e6feccd32798ca700fb958",
					"head_short_sha":     "089d92cb",
					"head_short_sha_7":   "089d92c",
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
	}

	for _, c := range cases {
		gen := PullRequestGenerator{
			selectServiceProviderFunc: c.selectFunc,
		}
		generatorConfig := argoprojiov1alpha1.ApplicationSetGenerator{
			PullRequest: &argoprojiov1alpha1.PullRequestGenerator{},
		}

		got, gotErr := gen.GenerateParams(&generatorConfig, &c.applicationSet)
		assert.Equal(t, c.expectedErr, gotErr)
		assert.ElementsMatch(t, c.expected, got)
	}
}

func TestPullRequestGetSecretRef(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test-secret", Namespace: "test"},
		Data: map[string][]byte{
			"my-token": []byte("secret"),
		},
	}
	gen := &PullRequestGenerator{client: fake.NewClientBuilder().WithObjects(secret).Build()}
	ctx := context.Background()

	cases := []struct {
		name, namespace, token string
		ref                    *argoprojiov1alpha1.SecretRef
		hasError               bool
	}{
		{
			name:      "valid ref",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "test-secret", Key: "my-token"},
			namespace: "test",
			token:     "secret",
			hasError:  false,
		},
		{
			name:      "nil ref",
			ref:       nil,
			namespace: "test",
			token:     "",
			hasError:  false,
		},
		{
			name:      "wrong name",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "other", Key: "my-token"},
			namespace: "test",
			token:     "",
			hasError:  true,
		},
		{
			name:      "wrong key",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "test-secret", Key: "other-token"},
			namespace: "test",
			token:     "",
			hasError:  true,
		},
		{
			name:      "wrong namespace",
			ref:       &argoprojiov1alpha1.SecretRef{SecretName: "test-secret", Key: "my-token"},
			namespace: "other",
			token:     "",
			hasError:  true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			token, err := gen.getSecretRef(ctx, c.ref, c.namespace)
			if c.hasError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			assert.Equal(t, c.token, token)
		})
	}
}

func TestAllowedSCMProviderPullRequest(t *testing.T) {
	cases := []struct {
		name           string
		providerConfig *argoprojiov1alpha1.PullRequestGenerator
		expectedError  error
	}{
		{
			name: "Error Github",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				Github: &argoprojiov1alpha1.PullRequestGeneratorGithub{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
			expectedError: &ErrDisallowedSCMProvider{},
		},
		{
			name: "Error Gitlab",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				GitLab: &argoprojiov1alpha1.PullRequestGeneratorGitLab{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
			expectedError: &ErrDisallowedSCMProvider{},
		},
		{
			name: "Error Gitea",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				Gitea: &argoprojiov1alpha1.PullRequestGeneratorGitea{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
			expectedError: &ErrDisallowedSCMProvider{},
		},
		{
			name: "Error Bitbucket",
			providerConfig: &argoprojiov1alpha1.PullRequestGenerator{
				BitbucketServer: &argoprojiov1alpha1.PullRequestGeneratorBitbucketServer{
					API: "https://myservice.mynamespace.svc.cluster.local",
				},
			},
			expectedError: &ErrDisallowedSCMProvider{},
		},
	}

	for _, testCase := range cases {
		testCaseCopy := testCase

		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()

			pullRequestGenerator := NewPullRequestGenerator(nil, SCMAuthProviders{}, "", []string{
				"github.myorg.com",
				"gitlab.myorg.com",
				"gitea.myorg.com",
				"bitbucket.myorg.com",
				"azuredevops.myorg.com",
			}, true)

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

			_, err := pullRequestGenerator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo)

			assert.Error(t, err, "Must return an error")
			assert.ErrorAs(t, err, testCaseCopy.expectedError)
		})
	}
}

func TestSCMProviderDisabled_PRGenerator(t *testing.T) {
	generator := NewPullRequestGenerator(nil, SCMAuthProviders{}, "", []string{}, false)

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

	_, err := generator.GenerateParams(&applicationSetInfo.Spec.Generators[0], &applicationSetInfo)
	assert.ErrorIs(t, err, ErrSCMProvidersDisabled)
}
