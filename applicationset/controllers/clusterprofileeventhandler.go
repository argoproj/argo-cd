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

	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// clusterProfileEventHandler is used when watching ClusterProfiles to requeue any related ApplicationSets.
type clusterProfileEventHandler struct {
	// handler.EnqueueRequestForOwner
	Log    log.FieldLogger
	Client client.Client
}

func (h *clusterProfileEventHandler) Create(ctx context.Context, e event.CreateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.Object)
}

func (h *clusterProfileEventHandler) Update(ctx context.Context, e event.UpdateEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.ObjectNew)
}

func (h *clusterProfileEventHandler) Delete(ctx context.Context, e event.DeleteEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.Object)
}

func (h *clusterProfileEventHandler) Generic(ctx context.Context, e event.GenericEvent, q workqueue.TypedRateLimitingInterface[reconcile.Request]) {
	h.queueRelatedAppGenerators(ctx, q, e.Object)
}

// addRateLimitingInterface defines the Add method of workqueue.RateLimitingInterface, allow us to easily mock
// it for testing purposes.
type addRateLimitingInterface[T comparable] interface {
	Add(item T)
}

func (h *clusterProfileEventHandler) queueRelatedAppGenerators(ctx context.Context, q addRateLimitingInterface[reconcile.Request], object client.Object) {
	h.Log.WithFields(log.Fields{
		"namespace": object.GetNamespace(),
		"name":      object.GetName(),
	}).Info("processing event for cluster profile")

	appSetList := &argoprojiov1alpha1.ApplicationSetList{}
	err := h.Client.List(ctx, appSetList)
	if err != nil {
		h.Log.WithError(err).Error("unable to list ApplicationSets")
		return
	}

	h.Log.WithField("count", len(appSetList.Items)).Info("listed ApplicationSets")
	for _, appSet := range appSetList.Items {
		foundClusterProfileGenerator := false
		for _, generator := range appSet.Spec.Generators {
			if generator.ClusterProfile != nil {
				foundClusterProfileGenerator = true
				break
			}

			if generator.Matrix != nil {
				ok, err := nestedGeneratorsHaveClusterProfileGenerator(generator.Matrix.Generators)
				if err != nil {
					h.Log.
						WithFields(log.Fields{
							"namespace": appSet.GetNamespace(),
							"name":      appSet.GetName(),
						}).
						WithError(err).
						Error("Unable to check if ApplicationSet matrix generators have cluster profile generator")
				}
				if ok {
					foundClusterProfileGenerator = true
					break
				}
			}

			if generator.Merge != nil {
				ok, err := nestedGeneratorsHaveClusterProfileGenerator(generator.Merge.Generators)
				if err != nil {
					h.Log.
						WithFields(log.Fields{
							"namespace": appSet.GetNamespace(),
							"name":      appSet.GetName(),
						}).
						WithError(err).
						Error("Unable to check if ApplicationSet merge generators have cluster profile generator")
				}
				if ok {
					foundClusterProfileGenerator = true
					break
				}
			}
		}
		if foundClusterProfileGenerator {
			// TODO: only queue the AppGenerator if the labels match this cluster
			req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: appSet.Namespace, Name: appSet.Name}}
			q.Add(req)
		}
	}
}

// nestedGeneratorsHaveClusterProfileGenerator iterate over provided nested generators to check if they have a cluster profile generator.
func nestedGeneratorsHaveClusterProfileGenerator(generators []argoprojiov1alpha1.ApplicationSetNestedGenerator) (bool, error) {
	for _, generator := range generators {
		if ok, err := nestedGeneratorHasClusterProfileGenerator(generator); ok || err != nil {
			return ok, err
		}
	}
	return false, nil
}

// nestedGeneratorHasClusterProfileGenerator checks if the provided generator has a cluster profile generator.
func nestedGeneratorHasClusterProfileGenerator(nested argoprojiov1alpha1.ApplicationSetNestedGenerator) (bool, error) {
	if nested.ClusterProfile != nil {
		return true, nil
	}

	if nested.Matrix != nil {
		nestedMatrix, err := argoprojiov1alpha1.ToNestedMatrixGenerator(nested.Matrix)
		if err != nil {
			return false, fmt.Errorf("unable to get nested matrix generator: %w", err)
		}
		if nestedMatrix != nil {
			hasClusterProfileGenerator, err := nestedGeneratorsHaveClusterProfileGenerator(nestedMatrix.ToMatrixGenerator().Generators)
			if err != nil {
				return false, fmt.Errorf("error evaluating nested matrix generator: %w", err)
			}
			return hasClusterProfileGenerator, nil
		}
	}

	if nested.Merge != nil {
		nestedMerge, err := argoprojiov1alpha1.ToNestedMergeGenerator(nested.Merge)
		if err != nil {
			return false, fmt.Errorf("unable to get nested merge generator: %w", err)
		}
		if nestedMerge != nil {
			hasClusterProfileGenerator, err := nestedGeneratorsHaveClusterProfileGenerator(nestedMerge.ToMergeGenerator().Generators)
			if err != nil {
				return false, fmt.Errorf("error evaluating nested merge generator: %w", err)
			}
			return hasClusterProfileGenerator, nil
		}
	}

	return false, nil
}
