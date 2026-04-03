package commit

import "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

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
	if repo.GithubAppPrivateKey != "" && repo.GithubAppId != 0 { // Promoter MVP: remove github-app-installation-id check since it is no longer a required field
		return "github-app"
	}
	if repo.GCPServiceAccountKey != "" {
		return "cloud-source-repositories"
	}
	return ""
}
