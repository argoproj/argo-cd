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
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestPullRequestGithubGenerateParams(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		selectFunc  func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error)
		expected    []map[string]string
		expectedErr error
	}{
		{
			selectFunc: func(context.Context, *argoprojiov1alpha1.PullRequestGenerator, *argoprojiov1alpha1.ApplicationSet) (pullrequest.PullRequestService, error) {
				return pullrequest.NewFakeService(
					ctx,
					[]*pullrequest.PullRequest{
						&pullrequest.PullRequest{
							Number:  1,
							Branch:  "branch1",
							HeadSHA: "089d92cbf9ff857a39e6feccd32798ca700fb958",
						},
					},
					nil,
				)
			},
			expected: []map[string]string{
				{
					"number":   "1",
					"branch":   "branch1",
					"head_sha": "089d92cbf9ff857a39e6feccd32798ca700fb958",
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
	}

	for _, c := range cases {
		gen := PullRequestGenerator{
			selectServiceProviderFunc: c.selectFunc,
		}
		generatorConfig := argoprojiov1alpha1.ApplicationSetGenerator{
			PullRequest: &argoprojiov1alpha1.PullRequestGenerator{},
		}
		got, gotErr := gen.GenerateParams(&generatorConfig, nil)
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
