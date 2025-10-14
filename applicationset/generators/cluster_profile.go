package generators

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoappsetv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	clusterinventory "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
)

var _ Generator = (*ClusterProfileGenerator)(nil)

// ClusterProfileGenerator generates Applications for some or all clusters registered with ArgoCD.
type ClusterProfileGenerator struct {
	client.Client
	ctx       context.Context
	namespace string
}

func NewClusterProfileGenerator(ctx context.Context, c client.Client, namespace string) Generator {
	g := &ClusterProfileGenerator{
		Client:    c,
		ctx:       ctx,
		namespace: namespace,
	}
	return g
}

// GetRequeueAfter never requeue the cluster profile generator because the `clusterSecretEventHandler` will requeue the appsets
// when the cluster secrets change
func (g *ClusterProfileGenerator) GetRequeueAfter(_ *argoappsetv1alpha1.ApplicationSetGenerator) time.Duration {
	return NoRequeueAfter
}

func (g *ClusterProfileGenerator) GetTemplate(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator) *argoappsetv1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.ClusterProfiles.Template
}

func (g *ClusterProfileGenerator) GenerateParams(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator, appSet *argoappsetv1alpha1.ApplicationSet, _ client.Client) ([]map[string]any, error) {
	logCtx := log.WithField("applicationset", appSet.GetName()).WithField("namespace", appSet.GetNamespace())
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.ClusterProfiles == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	// List all ClusterProfile objects
	clusterProfiles := &clusterinventory.ClusterProfileList{}
	if err := g.List(g.ctx, clusterProfiles); err != nil {
		return nil, fmt.Errorf("error listing cluster profiles: %w", err)
	}

	paramHolder := &paramHolder{isFlatMode: appSetGenerator.ClusterProfiles.FlatList}
	logCtx.Debugf("Using flat mode = %t for cluster profile generator", paramHolder.isFlatMode)

	for _, cluster := range clusterProfiles.Items {
		if !g.matchesSelector(&cluster, &appSetGenerator.ClusterProfiles.Selector) {
			continue
		}

		params := g.getClusterParameters(cluster, appSet)

		err := appendTemplatedValues(appSetGenerator.ClusterProfiles.Values, params, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("error appending templated values for cluster: %w", err)
		}

		paramHolder.append(params)
		logCtx.WithField("cluster", cluster.Name).Debug("matched cluster profile")
	}

	return paramHolder.consolidate(), nil
}

func (g *ClusterProfileGenerator) matchesSelector(cluster *clusterinventory.ClusterProfile, selector *metav1.LabelSelector) bool {
	labelSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		log.Errorf("error converting label selector: %v", err)
		return false
	}
	return labelSelector.Matches(labels.Set(cluster.Labels))
}

func (g *ClusterProfileGenerator) getClusterParameters(cluster clusterinventory.ClusterProfile, appSet *argoappsetv1alpha1.ApplicationSet) map[string]any {
	params := map[string]any{}

	params["name"] = cluster.Name
	params["nameNormalized"] = utils.SanitizeName(cluster.Name)
	// Find the server URL from the credential providers
	for _, provider := range cluster.Status.CredentialProviders {
		if provider.Name == "kubeconfig" {
			params["server"] = provider.Cluster.Server
			break
		}
	}
	if _, ok := params["server"]; !ok {
		params["server"] = ""
	}

	params["project"] = "" // Project information is not available in ClusterProfile

	if appSet.Spec.GoTemplate {
		meta := map[string]any{}

		if len(cluster.Annotations) > 0 {
			meta["annotations"] = cluster.Annotations
		}
		if len(cluster.Labels) > 0 {
			meta["labels"] = cluster.Labels
		}

		params["metadata"] = meta
	} else {
		for key, value := range cluster.Annotations {
			params["metadata.annotations."+key] = value
		}

		for key, value := range cluster.Labels {
			params["metadata.labels."+key] = value
		}
	}
	return params
}
