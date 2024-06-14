package github_app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/applicationset/services/github_app_auth"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type ArgocdRepositoryMock struct {
	mock *mock.Mock
}

func (a ArgocdRepositoryMock) GetRepoCredsBySecretName(ctx context.Context, secretName string) (*v1alpha1.RepoCreds, error) {
	args := a.mock.Called(ctx, secretName)

	return args.Get(0).(*v1alpha1.RepoCreds), args.Error(1)
}

func Test_repoAsCredentials_GetAuth(t *testing.T) {
	tests := []struct {
		name    string
		repo    v1alpha1.RepoCreds
		want    *github_app_auth.Authentication
		wantErr bool
	}{
		{name: "missing", wantErr: true},
		{name: "found", repo: v1alpha1.RepoCreds{
			GithubAppId:             123,
			GithubAppInstallationId: 456,
			GithubAppPrivateKey:     "private key",
		}, want: &github_app_auth.Authentication{
			Id:                123,
			InstallationId:    456,
			EnterpriseBaseURL: "",
			PrivateKey:        "private key",
		}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mock.Mock{}
			m.On("GetRepoCredsBySecretName", mock.Anything, mock.Anything).Return(&tt.repo, nil)
			creds := NewAuthCredentials(ArgocdRepositoryMock{mock: &m})

			auth, err := creds.GetAuthSecret(context.Background(), "https://github.com/foo")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, auth)
		})
	}
}
