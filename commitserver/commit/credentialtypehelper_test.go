package commit

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func TestRepository_GetCredentialType(t *testing.T) {
	tests := []struct {
		name string
		repo *v1alpha1.Repository
		want string
	}{
		{
			name: "Empty Repository",
			repo: nil,
			want: "",
		},
		{
			name: "HTTPS Repository",
			repo: &v1alpha1.Repository{
				Repo:     "foo",
				Password: "some-password",
			},
			want: "https",
		},
		{
			name: "SSH Repository",
			repo: &v1alpha1.Repository{
				Repo:          "foo",
				SSHPrivateKey: "some-key",
			},
			want: "ssh",
		},
		{
			name: "GitHub App Repository",
			repo: &v1alpha1.Repository{
				Repo:                    "foo",
				GithubAppPrivateKey:     "some-key",
				GithubAppId:             1,
				GithubAppInstallationId: 1,
			},
			want: "github-app",
		},
		{
			name: "Google Cloud Repository",
			repo: &v1alpha1.Repository{
				Repo:                 "foo",
				GCPServiceAccountKey: "some-key",
			},
			want: "cloud-source-repositories",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getCredentialType(tt.repo); got != tt.want {
				t.Errorf("Repository.GetCredentialType() = %v, want %v", got, tt.want)
			}
		})
	}
}
