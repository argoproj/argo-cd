package generators

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v2/util/settings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoappsetv1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const (
	ArgoCDSecretTypeLabel   = "argocd.argoproj.io/secret-type"
	ArgoCDSecretTypeCluster = "cluster"
)

var _ Generator = (*ClusterGenerator)(nil)

// ClusterGenerator generates Applications for some or all clusters registered with ArgoCD.
type ClusterGenerator struct {
	client.Client
	ctx       context.Context
	clientset kubernetes.Interface
	// namespace is the Argo CD namespace
	namespace       string
	settingsManager *settings.SettingsManager
}

var render = &utils.Render{}

func NewClusterGenerator(c client.Client, ctx context.Context, clientset kubernetes.Interface, namespace string) Generator {

	settingsManager := settings.NewSettingsManager(ctx, clientset, namespace)

	g := &ClusterGenerator{
		Client:          c,
		ctx:             ctx,
		clientset:       clientset,
		namespace:       namespace,
		settingsManager: settingsManager,
	}
	return g
}

func (g *ClusterGenerator) GetRequeueAfter(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator) time.Duration {
	return NoRequeueAfter
}

func (g *ClusterGenerator) GetTemplate(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator) *argoappsetv1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.Clusters.Template
}

func (g *ClusterGenerator) GenerateParams(
	appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator, appSet *argoappsetv1alpha1.ApplicationSet) ([]map[string]interface{}, error) {

	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	if appSetGenerator.Clusters == nil {
		return nil, EmptyAppSetGeneratorError
	}

	// Do not include the local cluster in the cluster parameters IF there is a non-empty selector
	// - Since local clusters do not have secrets, they do not have labels to match against
	ignoreLocalClusters := len(appSetGenerator.Clusters.Selector.MatchExpressions) > 0 || len(appSetGenerator.Clusters.Selector.MatchLabels) > 0

	// ListCluster from Argo CD's util/db package will include the local cluster in the list of clusters
	clustersFromArgoCD, err := utils.ListClusters(g.ctx, g.clientset, g.namespace)
	if err != nil {
		return nil, err
	}

	if clustersFromArgoCD == nil {
		return nil, nil
	}

	clusterSecrets, err := g.getSecretsByClusterName(appSetGenerator)
	if err != nil {
		return nil, err
	}

	res := []map[string]interface{}{}

	secretsFound := []corev1.Secret{}

	for _, cluster := range clustersFromArgoCD.Items {

		// If there is a secret for this cluster, then it's a non-local cluster, so it will be
		// handled by the next step.
		if secretForCluster, exists := clusterSecrets[cluster.Name]; exists {
			secretsFound = append(secretsFound, secretForCluster)

		} else if !ignoreLocalClusters {
			// If there is no secret for the cluster, it's the local cluster, so handle it here.
			params := map[string]interface{}{}
			params["name"] = cluster.Name
			params["nameNormalized"] = cluster.Name
			params["server"] = cluster.Server

			err = appendTemplatedValues(appSetGenerator.Clusters.Values, params, appSet)
			if err != nil {
				return nil, err
			}

			res = append(res, params)

			log.WithField("cluster", "local cluster").Info("matched local cluster")
		}
	}

	// For each matching cluster secret (non-local clusters only)
	for _, cluster := range secretsFound {
		params := map[string]interface{}{}

		params["name"] = string(cluster.Data["name"])
		params["nameNormalized"] = utils.SanitizeName(string(cluster.Data["name"]))
		params["server"] = string(cluster.Data["server"])

		if appSet.Spec.GoTemplate {
			meta := map[string]interface{}{}

			if len(cluster.ObjectMeta.Annotations) > 0 {
				meta["annotations"] = cluster.ObjectMeta.Annotations
			}
			if len(cluster.ObjectMeta.Labels) > 0 {
				meta["labels"] = cluster.ObjectMeta.Labels
			}

			params["metadata"] = meta
		} else {
			for key, value := range cluster.ObjectMeta.Annotations {
				params[fmt.Sprintf("metadata.annotations.%s", key)] = value
			}

			for key, value := range cluster.ObjectMeta.Labels {
				params[fmt.Sprintf("metadata.labels.%s", key)] = value
			}
		}

		err = appendTemplatedValues(appSetGenerator.Clusters.Values, params, appSet)
		if err != nil {
			return nil, err
		}

		res = append(res, params)

		log.WithField("cluster", cluster.Name).Info("matched cluster secret")
	}

	return res, nil
}

func appendTemplatedValues(clusterValues map[string]string, params map[string]interface{}, appSet *argoappsetv1alpha1.ApplicationSet) error {
	// We create a local map to ensure that we do not fall victim to a billion-laughs attack. We iterate through the
	// cluster values map and only replace values in said map if it has already been whitelisted in the params map.
	// Once we iterate through all the cluster values we can then safely merge the `tmp` map into the main params map.
	tmp := map[string]interface{}{}

	for key, value := range clusterValues {
		result, err := replaceTemplatedString(value, params, appSet)

		if err != nil {
			return err
		}

		if appSet.Spec.GoTemplate {
			if tmp["values"] == nil {
				tmp["values"] = map[string]string{}
			}
			tmp["values"].(map[string]string)[key] = result
		} else {
			tmp[fmt.Sprintf("values.%s", key)] = result
		}
	}

	for key, value := range tmp {
		params[key] = value
	}

	return nil
}

func replaceTemplatedString(value string, params map[string]interface{}, appSet *argoappsetv1alpha1.ApplicationSet) (string, error) {
	replacedTmplStr, err := render.Replace(value, params, appSet.Spec.GoTemplate)
	if err != nil {
		return "", err
	}
	return replacedTmplStr, nil
}

func (g *ClusterGenerator) getSecretsByClusterName(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator) (map[string]corev1.Secret, error) {
	// List all Clusters:
	clusterSecretList := &corev1.SecretList{}

	selector := metav1.AddLabelToSelector(&appSetGenerator.Clusters.Selector, ArgoCDSecretTypeLabel, ArgoCDSecretTypeCluster)
	secretSelector, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil {
		return nil, err
	}

	if err := g.Client.List(context.Background(), clusterSecretList, client.MatchingLabelsSelector{Selector: secretSelector}); err != nil {
		return nil, err
	}
	log.Debug("clusters matching labels", "count", len(clusterSecretList.Items))

	res := map[string]corev1.Secret{}

	for _, cluster := range clusterSecretList.Items {
		clusterName := string(cluster.Data["name"])

		res[clusterName] = cluster
	}

	return res, nil

}
