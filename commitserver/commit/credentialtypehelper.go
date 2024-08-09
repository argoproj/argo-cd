package commit

import "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

// getCredentialType returns the type of credential used by the repository.
func getCredentialType(repo *v1alpha1.Repository) string {
	if repo == nil {
		return ""
	}
	if repo.Password != "" {
		return "https"
	}
	if repo.SSHPrivateKey != "" {
		return "ssh"
	}
	if repo.GithubAppPrivateKey != "" && repo.GithubAppId != 0 && repo.GithubAppInstallationId != 0 {
		return "github-app"
	}
	if repo.GCPServiceAccountKey != "" {
		return "cloud-source-repositories"
	}
	return ""
}
