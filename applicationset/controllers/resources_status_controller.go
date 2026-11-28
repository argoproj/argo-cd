package controllers

import (
	"context"
	"fmt"

	log "github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

const resourcesStatusControllerName = "applicationset-resources-status"

// setupResourcesStatusController registers a controller that refreshes status.resources when the health or sync
// status of an owned Application changes. The main controller does not requeue on those changes.
func (r *ApplicationSetReconciler) setupResourcesStatusController(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Named(resourcesStatusControllerName).
		Watches(
			&argov1alpha1.Application{},
			handler.EnqueueRequestForOwner(mgr.GetScheme(), mgr.GetRESTMapper(), &argov1alpha1.ApplicationSet{}, handler.OnlyControllerOwner()),
			builder.WithPredicates(getApplicationStatusChangedPredicate()),
		).
		WithEventFilter(ignoreNotAllowedNamespaces(r.ApplicationSetNamespaces)).
		Complete(reconcile.Func(r.reconcileResourcesStatus))
}

// getApplicationStatusChangedPredicate only passes updates that change an Application's health or sync status.
func getApplicationStatusChangedPredicate() predicate.Funcs {
	return predicate.Funcs{
		CreateFunc: func(_ event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			appOld, isApp := e.ObjectOld.(*argov1alpha1.Application)
			if !isApp {
				return false
			}
			appNew, isApp := e.ObjectNew.(*argov1alpha1.Application)
			if !isApp {
				return false
			}
			return appOld.Status.Health.Status != appNew.Status.Health.Status ||
				appOld.Status.Sync.Status != appNew.Status.Sync.Status
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}
}

// reconcileResourcesStatus rebuilds status.resources from the owned Applications without running generators.
func (r *ApplicationSetReconciler) reconcileResourcesStatus(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logCtx := log.WithField("applicationset", req.NamespacedName)

	var appset argov1alpha1.ApplicationSet
	if err := r.Get(ctx, req.NamespacedName, &appset); err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get application set: %w", err)
		}
		return ctrl.Result{}, nil
	}
	if appset.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	apps, err := r.getCurrentApplications(ctx, appset)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get current applications for application set: %w", err)
	}
	if err := r.updateResourcesStatus(ctx, logCtx, &appset, apps); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update resources status for application set: %w", err)
	}
	return ctrl.Result{}, nil
}
