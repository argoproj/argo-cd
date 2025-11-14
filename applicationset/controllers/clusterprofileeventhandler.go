package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	log "github.com/sirupsen/logrus"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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

	clusterProfileLabels := object.GetLabels()
	if clusterProfileLabels == nil {
		clusterProfileLabels = map[string]string{}
	}

	h.Log.WithField("count", len(appSetList.Items)).Info("listed ApplicationSets")
	for _, appSet := range appSetList.Items {
		found, err := hasMatchingClusterProfileGenerator(appSet, clusterProfileLabels)
		if err != nil {
			h.Log.
				WithFields(log.Fields{
					"namespace": appSet.GetNamespace(),
					"name":      appSet.GetName(),
				}).
				WithError(err).
				Error("Unable to check if ApplicationSet has matching cluster profile generator")
			continue
		}

		if found {
			req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: appSet.Namespace, Name: appSet.Name}}
			q.Add(req)
		}
	}
}

// hasMatchingClusterProfileGenerator checks if an ApplicationSet has a ClusterProfiles generator that matches the given labels.
func hasMatchingClusterProfileGenerator(appSet argoprojiov1alpha1.ApplicationSet, clusterProfileLabels map[string]string) (bool, error) {
	for _, generator := range appSet.Spec.Generators {
		if generator.ClusterProfiles != nil {
			selector, err := metav1.LabelSelectorAsSelector(&generator.ClusterProfiles.Selector)
			if err != nil {
				return false, fmt.Errorf("failed to parse label selector: %w", err)
			}
			if selector.Matches(labels.Set(clusterProfileLabels)) {
				return true, nil
			}
		}

		if generator.Matrix != nil {
			ok, err := nestedGeneratorsHaveMatchingClusterProfileGenerator(generator.Matrix.Generators, clusterProfileLabels)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}

		if generator.Merge != nil {
			ok, err := nestedGeneratorsHaveMatchingClusterProfileGenerator(generator.Merge.Generators, clusterProfileLabels)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
	}

	return false, nil
}

// nestedGeneratorsHaveMatchingClusterProfileGenerator iterates over provided nested generators to check if they have a matching cluster profile generator.
func nestedGeneratorsHaveMatchingClusterProfileGenerator(generators []argoprojiov1alpha1.ApplicationSetNestedGenerator, clusterProfileLabels map[string]string) (bool, error) {
	for _, generator := range generators {
		if ok, err := nestedGeneratorHasMatchingClusterProfileGenerator(generator, clusterProfileLabels); ok || err != nil {
			return ok, err
		}
	}
	return false, nil
}

// nestedGeneratorHasMatchingClusterProfileGenerator checks if the provided generator has a matching cluster profile generator.
func nestedGeneratorHasMatchingClusterProfileGenerator(nested argoprojiov1alpha1.ApplicationSetNestedGenerator, clusterProfileLabels map[string]string) (bool, error) {
	if nested.ClusterProfiles != nil {
		selector, err := metav1.LabelSelectorAsSelector(&nested.ClusterProfiles.Selector)
		if err != nil {
			return false, fmt.Errorf("failed to parse label selector: %w", err)
		}
		return selector.Matches(labels.Set(clusterProfileLabels)), nil
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
			var genDef argoprojiov1alpha1.ApplicationSetNestedGenerator
			if err2 := json.Unmarshal(rawGenerators.Raw, &genDef); err2 != nil {
				return false, fmt.Errorf("unable to unmarshal nested generator: %w", err)
			}
			return nestedGeneratorHasMatchingClusterProfileGenerator(genDef, clusterProfileLabels)
		}

		for _, gen := range nestedGenerators.Generators {
			if ok, err := nestedGeneratorHasMatchingClusterProfileGenerator(gen, clusterProfileLabels); ok || err != nil {
				return ok, err
			}
		}
	}

	return false, nil
}
