package application_test

import (
	"testing"

	"time"

	"github.com/argoproj/argo-cd/application"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
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

	t.Run("NeedRefreshAppStatus", func(t *testing.T) {

		manager := application.NewAppManager(nil, nil, nil, refreshTimeout)
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
}
