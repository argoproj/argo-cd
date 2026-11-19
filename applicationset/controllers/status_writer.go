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

/**
  statusWriter consolidates every write to the ApplicationSet status subresource onto a single
  code path.

  Reconcile builds up the desired status in memory over the course of a reconciliation and the
  writer replays those changes onto a freshly read object exactly once, at the end. Keeping the
  changes as functions rather than as a finished status object matters for two reasons:

    1. A change only touches the fields it owns, so unrelated fields on the server copy are never
       clobbered. This preserves the behaviour of the four separate writers this type replaces.
    2. On a conflict the retry re-reads the object and replays the same functions against the newer
       copy, so the result is correct rather than merely overwritten.
*/

// statusWriter records the status changes made during one reconciliation and writes them once.
// A statusWriter is scoped to a single Reconcile call and is not safe for concurrent use.
type statusWriter struct {
	mutators []func(*argov1alpha1.ApplicationSet)
}

var _ progressivesync.StatusWriter = (*statusWriter)(nil)

// Apply changes the in-memory ApplicationSet immediately so that later stages of the
// reconciliation observe the new status, and records the change so that Flush replays it against
// the object that is finally written.
//
// mutate must be safe to run more than once and against a different copy of the ApplicationSet
// than the one passed here.
func (w *statusWriter) Apply(appset *argov1alpha1.ApplicationSet, mutate func(*argov1alpha1.ApplicationSet)) {
	mutate(appset)
	w.mutators = append(w.mutators, mutate)
}

// Flush writes the recorded changes to the API server. It reads the ApplicationSet, replays every
// recorded change onto it, and issues one status update. When the replay produces no change to the
// status, Flush makes no write.
//
// Flush may be called more than once per reconciliation. Changes recorded before an earlier call
// are replayed again, which is harmless because they are idempotent, and which keeps the server
// copy correct if another writer has touched the object in between.
func (w *statusWriter) Flush(ctx context.Context, c client.Client, appset *argov1alpha1.ApplicationSet) error {
	if len(w.mutators) == 0 {
		return nil
	}

	// DefaultRetry will retry 5 times with a backoff factor of 1, jitter of 0.1 and a duration of 10ms
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updatedAppset := &argov1alpha1.ApplicationSet{}
		if err := c.Get(ctx, client.ObjectKeyFromObject(appset), updatedAppset); err != nil {
			if apierrors.IsNotFound(err) {
				// The ApplicationSet is gone, so there is no status left to report on.
				return nil
			}
			return fmt.Errorf("error fetching updated application set: %w", err)
		}

		previousStatus := updatedAppset.Status.DeepCopy()
		for _, mutate := range w.mutators {
			mutate(updatedAppset)
		}

		// Try to not call update if nothing has changed
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
