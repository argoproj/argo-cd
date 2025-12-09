package generators

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	"github.com/argoproj/argo-cd/v3/common"
	argoappsetv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

var _ Generator = (*ClusterGenerator)(nil)

// ClusterGenerator generates Applications for some or all clusters registered with ArgoCD.
type ClusterGenerator struct {
	client.Client
	// namespace is the Argo CD namespace
	namespace string
}

var render = &utils.Render{}

func NewClusterGenerator(c client.Client, namespace string) Generator {
	g := &ClusterGenerator{
		Client:    c,
		namespace: namespace,
	}
	return g
}

// GetRequeueAfter never requeue the cluster generator because the `clusterSecretEventHandler` will requeue the appsets
// when the cluster secrets change
func (g *ClusterGenerator) GetRequeueAfter(_ *argoappsetv1alpha1.ApplicationSetGenerator) time.Duration {
	return NoRequeueAfter
}

func (g *ClusterGenerator) GetTemplate(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator) *argoappsetv1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Clusters.Template
}

func (g *ClusterGenerator) GenerateParams(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator, appSet *argoappsetv1alpha1.ApplicationSet, _ client.Client) ([]map[string]any, error) {
	logCtx := log.WithField("applicationset", appSet.GetName()).WithField("namespace", appSet.GetNamespace())
	if appSetGenerator == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	if appSetGenerator.Clusters == nil {
		return nil, ErrEmptyAppSetGenerator
	}

	// Do not include the local cluster in the cluster parameters IF there is a non-empty selector
	// - Since local clusters do not have secrets, they do not have labels to match against
	ignoreLocalClusters := len(appSetGenerator.Clusters.Selector.MatchExpressions) > 0 || len(appSetGenerator.Clusters.Selector.MatchLabels) > 0

	// Get cluster secrets using the cached controller-runtime client
	clusterSecrets, err := g.getSecretsByClusterName(logCtx, appSetGenerator)
	if err != nil {
		return nil, fmt.Errorf("error getting cluster secrets: %w", err)
	}

	paramHolder := &paramHolder{isFlatMode: appSetGenerator.Clusters.FlatList}
	logCtx.Debugf("Using flat mode = %t for cluster generator", paramHolder.isFlatMode)

	// Convert map values to slice to check for an in-cluster secret
	secretsList := make([]corev1.Secret, 0, len(clusterSecrets))
	for _, secret := range clusterSecrets {
		secretsList = append(secretsList, secret)
	}

	// Add the in-cluster if it doesn't have a secret, and we're not ignoring in-cluster
	if !ignoreLocalClusters && !utils.SecretsContainInClusterCredentials(secretsList) {
		params := map[string]any{}
		params["name"] = argoappsetv1alpha1.KubernetesInClusterName
		params["nameNormalized"] = argoappsetv1alpha1.KubernetesInClusterName
		params["server"] = argoappsetv1alpha1.KubernetesInternalAPIServerAddr
		params["project"] = ""

		err = appendTemplatedValues(appSetGenerator.Clusters.Values, params, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("error appending templated values for local cluster: %w", err)
		}

		paramHolder.append(params)
		logCtx.WithField("cluster", "local cluster").Info("matched local cluster")
	}

	// For each matching cluster secret (non-local clusters only)
	for _, cluster := range clusterSecrets {
		params := g.getClusterParameters(cluster, appSet)

		err = appendTemplatedValues(appSetGenerator.Clusters.Values, params, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("error appending templated values for cluster: %w", err)
		}

		paramHolder.append(params)
		logCtx.WithField("cluster", cluster.Name).Debug("matched cluster secret")
	}

	return paramHolder.consolidate(), nil
}

type paramHolder struct {
	isFlatMode bool
	params     []map[string]any
}

func (p *paramHolder) append(params map[string]any) {
	p.params = append(p.params, params)
}

func (p *paramHolder) consolidate() []map[string]any {
	if p.isFlatMode {
		p.params = []map[string]any{
			{"clusters": p.params},
		}
	}
	return p.params
}

func (g *ClusterGenerator) getClusterParameters(cluster corev1.Secret, appSet *argoappsetv1alpha1.ApplicationSet) map[string]any {
	params := map[string]any{}

	params["name"] = string(cluster.Data["name"])
	params["nameNormalized"] = utils.SanitizeName(string(cluster.Data["name"]))
	params["server"] = string(cluster.Data["server"])

	project, ok := cluster.Data["project"]
	if ok {
		params["project"] = string(project)
	} else {
		params["project"] = ""
	}

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

func (g *ClusterGenerator) getSecretsByClusterName(log *log.Entry, appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator) (map[string]corev1.Secret, error) {
	clusterSecretList := &corev1.SecretList{}

	selector := metav1.AddLabelToSelector(&appSetGenerator.Clusters.Selector, common.LabelKeySecretType, common.LabelValueSecretTypeCluster)
	secretSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, fmt.Errorf("error converting label selector: %w", err)
	}

	if err := g.List(context.Background(), clusterSecretList, client.InNamespace(g.namespace), client.MatchingLabelsSelector{Selector: secretSelector}); err != nil {
		return nil, err
	}
	log.Debugf("clusters matching labels: %d", len(clusterSecretList.Items))

	res := map[string]corev1.Secret{}

	for _, cluster := range clusterSecretList.Items {
		clusterName := string(cluster.Data["name"])

		res[clusterName] = cluster
	}

	return res, nil
}
