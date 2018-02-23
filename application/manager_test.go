package application_test

import (
	"testing"

	"time"

	"context"

	"github.com/argoproj/argo-cd/application"
	appMocks "github.com/argoproj/argo-cd/application/mocks"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/server/repository"
	repoMocks "github.com/argoproj/argo-cd/server/repository/mock"
	gitMocks "github.com/argoproj/argo-cd/util/git/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestManager(t *testing.T) {

	refreshTimeout := time.Second * 10
	appSource := v1alpha1.ApplicationSource{
		Environment:    "prod/us-west-2",
		Path:           "apps/elk",
		TargetRevision: "master",
		RepoURL:        "http://my-git-repo.git",
	}
	gitClientMock := gitMocks.Client{}
	appComparatorMock := appMocks.AppComparator{}
	repoServiceMock := repoMocks.RepositoryServiceServer{}
	manager := application.NewAppManager(&gitClientMock, &repoServiceMock, &appComparatorMock, refreshTimeout)

	t.Run("NeedRefreshAppStatus", func(t *testing.T) {
		t.Run("TestReturnsTrueIfAppWasNotCompared", func(t *testing.T) {
			needRefresh := manager.NeedRefreshAppStatus(&v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{Source: appSource},
				Status: v1alpha1.ApplicationStatus{
					ComparisonResult: v1alpha1.ComparisonResult{Status: v1alpha1.ComparisonStatusUnknown},
				},
			})
			assert.True(t, needRefresh)
		})

		t.Run("TestReturnsFalseIfAppWasComparedBeforeRefreshTimeoutExpires", func(t *testing.T) {
			needRefresh := manager.NeedRefreshAppStatus(&v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{Source: appSource},
				Status: v1alpha1.ApplicationStatus{
					ComparisonResult: v1alpha1.ComparisonResult{Status: v1alpha1.ComparisonStatusEqual, ComparedAt: metav1.Time{Time: time.Now()}, ComparedTo: appSource},
				},
			})
			assert.False(t, needRefresh)
		})

		t.Run("TestReturnsTrueIfAppWasComparedAfterRefreshTimeoutExpires", func(t *testing.T) {
			needRefresh := manager.NeedRefreshAppStatus(&v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{Source: appSource},
				Status: v1alpha1.ApplicationStatus{
					ComparisonResult: v1alpha1.ComparisonResult{
						Status:     v1alpha1.ComparisonStatusEqual,
						ComparedAt: metav1.Time{Time: time.Now().Add(-(refreshTimeout + time.Second))},
						ComparedTo: appSource,
					},
				},
			})
			assert.True(t, needRefresh)
		})

		t.Run("TestReturnsTrueApplicationSourceHasChanged", func(t *testing.T) {
			updatedSource := *appSource.DeepCopy()
			updatedSource.TargetRevision = "abc"
			needRefresh := manager.NeedRefreshAppStatus(&v1alpha1.Application{
				Spec: v1alpha1.ApplicationSpec{Source: appSource},
				Status: v1alpha1.ApplicationStatus{
					ComparisonResult: v1alpha1.ComparisonResult{Status: v1alpha1.ComparisonStatusEqual, ComparedAt: metav1.Time{Time: time.Now()}, ComparedTo: updatedSource},
				},
			})
			assert.True(t, needRefresh)
		})
	})

	t.Run("RefreshAppStatus", func(t *testing.T) {
		repo := v1alpha1.Repository{
			Repo:     "https://testRepo/repo.git",
			Username: "user",
			Password: "test",
		}
		app := v1alpha1.Application{
			Spec:   v1alpha1.ApplicationSpec{Source: appSource},
			Status: v1alpha1.ApplicationStatus{},
		}

		repoServiceMock.On("Get", context.Background(), &repository.RepoQuery{
			Repo: appSource.RepoURL,
		}).Return(&repo, nil)
		var repoPath string
		gitClientMock.On("CloneOrFetch", repo.Repo, repo.Username, repo.Password, mock.MatchedBy(func(receivedRepoPath string) bool {
			repoPath = receivedRepoPath
			return true
		})).Return(nil)
		gitClientMock.On("Checkout", mock.MatchedBy(func(receivedRepoPath string) bool {
			return repoPath == receivedRepoPath
		}), appSource.TargetRevision).Return(nil)

		appComparatorMock.On("CompareAppState", mock.MatchedBy(func(receivedRepoPath string) bool {
			return repoPath == receivedRepoPath
		}), &app).Return(&v1alpha1.ComparisonResult{
			Status: v1alpha1.ComparisonStatusEqual,
		}, nil)
		updatedAppStatus := manager.RefreshAppStatus(&app)
		assert.Equal(t, updatedAppStatus.ComparisonResult.Status, v1alpha1.ComparisonStatusEqual)
	})
}
