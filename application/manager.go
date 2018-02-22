package application

import (
	"context"

	"io/ioutil"

	"fmt"

	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util/git"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Manager is responsible to retrieve application spec and compare it to actual application state.
type Manager struct {
	gitClient            git.Client
	repoService          repository.RepositoryServiceServer
	statusRefreshTimeout time.Duration
}

// NeedRefreshAppStatus answers if application status needs to be refreshed. Returns true if application never been compared, has changed or comparison result has expired.
func (m *Manager) NeedRefreshAppStatus(app *v1alpha1.Application) bool {
	return app.Status.ComparisonResult.Status == v1alpha1.ComparisonStatusUnknown ||
		!app.Spec.Source.Equals(app.Status.ComparisonResult.ComparedTo) ||
		app.Status.ComparisonResult.ComparedAt.Add(m.statusRefreshTimeout).Before(time.Now())
}

// RefreshAppStatus compares application against actual state in target cluster and returns updated status.
func (m *Manager) RefreshAppStatus(app *v1alpha1.Application) *v1alpha1.ApplicationStatus {
	status, err := m.tryRefreshAppStatus(app)
	if err != nil {
		status = &v1alpha1.ApplicationStatus{
			ComparisonResult: v1alpha1.ComparisonResult{
				Status:                 v1alpha1.ComparisonStatusError,
				ComparisonErrorDetails: fmt.Sprintf("Failed to get application status for application '%s': %+v", app.Name, err),
				ComparedTo:             app.Spec.Source,
				ComparedAt:             metav1.Time{Time: time.Now().UTC()},
			},
		}
	}
	return status
}

func (m *Manager) tryRefreshAppStatus(app *v1alpha1.Application) (*v1alpha1.ApplicationStatus, error) {
	repo, err := m.repoService.Get(context.Background(), &repository.RepoQuery{Repo: app.Spec.Source.RepoURL})
	if err != nil {
		return nil, err
	}

	appRepoPath, err := ioutil.TempDir("", app.Name)
	if err != nil {
		return nil, fmt.Errorf("unable to create temp repository directory for app '%s'", app.Name)
	}

	err = m.gitClient.CloneOrFetch(repo.Repo, repo.Username, repo.Password, appRepoPath)
	if err != nil {
		return nil, err
	}

	err = m.gitClient.Checkout(appRepoPath, app.Spec.Source.TargetRevision)
	if err != nil {
		return nil, err
	}
	comparisonResult, err := m.compareAppState(appRepoPath, app)
	if err != nil {
		return nil, err
	}
	return &v1alpha1.ApplicationStatus{
		ComparisonResult: *comparisonResult,
	}, nil
}

func (m *Manager) compareAppState(appRepoPath string, app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error) {
	// TODO (amatyushentsev): Implement actual comparison
	return &v1alpha1.ComparisonResult{
		Status:     v1alpha1.ComparisonStatusEqual,
		ComparedTo: app.Spec.Source,
		ComparedAt: metav1.Time{Time: time.Now().UTC()},
	}, nil
}

// NewAppManager creates new instance of app.Manager
func NewAppManager(gitClient git.Client, repoService repository.RepositoryServiceServer, statusRefreshTimeout time.Duration) *Manager {
	return &Manager{
		gitClient:            gitClient,
		repoService:          repoService,
		statusRefreshTimeout: statusRefreshTimeout,
	}
}
