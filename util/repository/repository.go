package repository

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
)

var errPermissionDenied = status.Error(codes.PermissionDenied, "permission denied")

// FilterRepositoryByProjectAndURL fetches a single repository which the user has access to. If only one repository can be found which
// matches the same URL, that will be returned (this is for backward compatibility reasons). If multiple repositories
// are matched, a repository is only returned if it matches the app project of the incoming request.
func FilterRepositoryByProjectAndURL(repositories v1alpha1.Repositories, repoURL, appProject string) (*v1alpha1.Repository, error) {
	var foundRepos []*v1alpha1.Repository
	for _, repo := range repositories {
		if git.SameURL(repo.Repo, repoURL) {
			foundRepos = append(foundRepos, repo)
		}
	}

	if len(foundRepos) == 0 {
		return nil, errPermissionDenied
	}

	if len(foundRepos) == 1 && appProject == "" {
		return foundRepos[0], nil
	}

	for _, repo := range foundRepos {
		if repo.Project == appProject {
			return repo, nil
		}
	}

	return nil, fmt.Errorf("repository not found for url %q and project %q", repoURL, appProject)
}
