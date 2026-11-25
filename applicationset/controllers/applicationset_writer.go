package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/progressivesync"
	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// applicationsetWriter records the status changes made during one reconciliation and writes them once.
// A applicationsetWriter is scoped to a single Reconcile call and is not safe for concurrent use.
type applicationsetWriter struct {
	mutators []func(*argov1alpha1.ApplicationSet)
}

var _ progressivesync.ApplicationSetWriter = (*applicationsetWriter)(nil)

func (w *applicationsetWriter) Apply(appset *argov1alpha1.ApplicationSet, mutate func(*argov1alpha1.ApplicationSet)) {
	mutate(appset)
	w.mutators = append(w.mutators, mutate)
}

func (w *applicationsetWriter) Flush(ctx context.Context, c client.Client, appset *argov1alpha1.ApplicationSet) error {
	if len(w.mutators) == 0 {
		return nil
	}

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updatedAppset := &argov1alpha1.ApplicationSet{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(appset), updatedAppset); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("error fetching updated application set: %w", err)
		}

		previousStatus := updatedAppset.Status.DeepCopy()
		for _, mutate := range w.mutators {
			mutate(updatedAppset)
		}

		if equality.Semantic.DeepEqual(previousStatus, &updatedAppset.Status) {
			return nil
		}

		if err := c.Status().Update(ctx, updatedAppset); err != nil {
			return err
		}
		updatedAppset.DeepCopyInto(appset)
		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to set application set status: %w", err)
	}
	return nil
}
