package controllers

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/argoproj/argo-cd/v2/applicationset/generators"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

// clusterSecretEventHandler is used when watching Secrets to check if they are ArgoCD Cluster Secrets, and if so
// requeue any related ApplicationSets.
type clusterSecretEventHandler struct {
	// handler.EnqueueRequestForOwner
	Log    log.FieldLogger
	Client client.Client
}

func (h *clusterSecretEventHandler) Create(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.Object)
}

func (h *clusterSecretEventHandler) Update(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.ObjectNew)
}

func (h *clusterSecretEventHandler) Delete(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.Object)
}

func (h *clusterSecretEventHandler) Generic(ctx context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.Object)
}

// addRateLimitingInterface defines the Add method of workqueue.RateLimitingInterface, allow us to easily mock
// it for testing purposes.
type addRateLimitingInterface[T comparable] interface {
	Add(item T)
}

func (h *clusterSecretEventHandler) queueRelatedAppGenerators(ctx context.Context, q addRateLimitingInterface[reconcile.Request], object client.Object) {
	// Check for label, lookup all ApplicationSets that might match the cluster, queue them all
	if object.GetLabels()[generators.ArgoCDSecretTypeLabel] != generators.ArgoCDSecretTypeCluster {
		return
	}

	h.Log.WithFields(log.Fields{
		"namespace": object.GetNamespace(),
		"name":      object.GetName(),
	}).Info("processing event for cluster secret")

	appSetList := &argoprojiov1alpha1.ApplicationSetList{}
	err := h.Client.List(ctx, appSetList)
	if err != nil {
		h.Log.WithError(err).Error("unable to list ApplicationSets")
		return
	}

	h.Log.WithField("count", len(appSetList.Items)).Info("listed ApplicationSets")
	for _, appSet := range appSetList.Items {
		foundClusterGenerator := false
		for _, generator := range appSet.Spec.Generators {
			if generator.Clusters != nil {
				foundClusterGenerator = true
				break
			}

			if generator.Matrix != nil {
				ok, err := nestedGeneratorsHaveClusterGenerator(generator.Matrix.Generators)
				if err != nil {
					h.Log.
						WithFields(log.Fields{
							"namespace": appSet.GetNamespace(),
							"name":      appSet.GetName(),
						}).
						WithError(err).
						Error("Unable to check if ApplicationSet matrix generators have cluster generator")
				}
				if ok {
					foundClusterGenerator = true
					break
				}
			}

			if generator.Merge != nil {
				ok, err := nestedGeneratorsHaveClusterGenerator(generator.Merge.Generators)
				if err != nil {
					h.Log.
						WithFields(log.Fields{
							"namespace": appSet.GetNamespace(),
							"name":      appSet.GetName(),
						}).
						WithError(err).
						Error("Unable to check if ApplicationSet merge generators have cluster generator")
				}
				if ok {
					foundClusterGenerator = true
					break
				}
			}
		}
		if foundClusterGenerator {
			// TODO: only queue the AppGenerator if the labels match this cluster
			req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: appSet.Namespace, Name: appSet.Name}}
			q.Add(req)
		}
	}
}

// nestedGeneratorsHaveClusterGenerator iterate over provided nested generators to check if they have a cluster generator.
func nestedGeneratorsHaveClusterGenerator(generators []argoprojiov1alpha1.ApplicationSetNestedGenerator) (bool, error) {
	for _, generator := range generators {
		if ok, err := nestedGeneratorHasClusterGenerator(generator); ok || err != nil {
			return ok, err
		}
	}
	return false, nil
}

// nestedGeneratorHasClusterGenerator checks if the provided generator has a cluster generator.
func nestedGeneratorHasClusterGenerator(nested argoprojiov1alpha1.ApplicationSetNestedGenerator) (bool, error) {
	if nested.Clusters != nil {
		return true, nil
	}

	if nested.Matrix != nil {
		nestedMatrix, err := argoprojiov1alpha1.ToNestedMatrixGenerator(nested.Matrix)
		if err != nil {
			return false, fmt.Errorf("unable to get nested matrix generator: %w", err)
		}
		if nestedMatrix != nil {
			hasClusterGenerator, err := nestedGeneratorsHaveClusterGenerator(nestedMatrix.ToMatrixGenerator().Generators)
			if err != nil {
				return false, fmt.Errorf("error evaluating nested matrix generator: %w", err)
			}
			return hasClusterGenerator, nil
		}
	}

	if nested.Merge != nil {
		nestedMerge, err := argoprojiov1alpha1.ToNestedMergeGenerator(nested.Merge)
		if err != nil {
			return false, fmt.Errorf("unable to get nested merge generator: %w", err)
		}
		if nestedMerge != nil {
			hasClusterGenerator, err := nestedGeneratorsHaveClusterGenerator(nestedMerge.ToMergeGenerator().Generators)
			if err != nil {
				return false, fmt.Errorf("error evaluating nested merge generator: %w", err)
			}
			return hasClusterGenerator, nil
		}
	}

	return false, nil
}
