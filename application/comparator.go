package application

import (
	"time"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppComparator defines methods which allow to compare application spec and actual application state.
type AppComparator interface {
	CompareAppState(appRepoPath string, app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error)
}

// KsonnetAppComparator allows to compare application using KSonnet CLI
type KsonnetAppComparator struct {
}

// CompareAppState compares application spec and real app state using KSonnet
func (ks *KsonnetAppComparator) CompareAppState(appRepoPath string, app *v1alpha1.Application) (*v1alpha1.ComparisonResult, error) {
	// TODO (amatyushentsev): Implement actual comparison
	return &v1alpha1.ComparisonResult{
		Status:     v1alpha1.ComparisonStatusEqual,
		ComparedTo: app.Spec.Source,
		ComparedAt: metav1.Time{Time: time.Now().UTC()},
	}, nil
}

// NewKsonnetAppComparator creates new instance of Ksonnet app comparator
func NewKsonnetAppComparator() AppComparator {
	return &KsonnetAppComparator{}
}
