package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	log "github.com/sirupsen/logrus"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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
			if generator.ClusterProfiles != nil {
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
	if nested.ClusterProfiles != nil {
		return true, nil
	}

	var rawGenerators *apiextensionsv1.JSON
	if nested.Matrix != nil {
		rawGenerators = nested.Matrix
	} else if nested.Merge != nil {
		rawGenerators = nested.Merge
	}

	if rawGenerators != nil && len(rawGenerators.Raw) > 0 {
		var nestedGenerators struct {
			Generators []argoprojiov1alpha1.ApplicationSetNestedGenerator `json:"generators"`
		}
		if err := json.Unmarshal(rawGenerators.Raw, &nestedGenerators); err != nil {
			return false, fmt.Errorf("unable to unmarshal nested generator: %w", err)
		}

		return nestedGeneratorsHaveClusterProfileGenerator(nestedGenerators.Generators)
	}

	return false, nil
}
