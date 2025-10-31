package generators

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterinventory "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"

	"github.com/argoproj/argo-cd/v3/applicationset/utils"
	argoappsetv1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// https://argo-cd.readthedocs.io/en/stable/operator-manual/declarative-setup/#clusters
	argoCDSecretType = "argocd.argoproj.io/secret-type"

	// Labels and annotations
	managedByLabel                 = "argocd.argoproj.io/managed-by-applicationset-controller"
	clusterProfileOriginAnnotation = "clusterprofile.x-k8s.io/origin"

	clusterProfileSecretTemplate = `{
	"execProviderConfig": {
		"command": "argocd-k8s-auth",
		"args": ["%s"],
		"apiVersion": "client.authentication.k8s.io/v1beta1"
	},
	"tlsClientConfig": {
		"insecure": false,
		"caData": ""
	}
}`
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

// GetRequeueAfter never requeue the cluster profile generator because the `clusterProfileEventHandler` will requeue the appsets
// when the cluster profiles change
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
		// Generate or update the cluster's corresponding secret
		if err := g.createOrUpdateClusterSecret(g.ctx, &cluster); err != nil {
			logCtx.WithError(err).Error("Failed to reconcile secret")
			return nil, err
		}

		params := g.getClusterParameters(cluster, appSet)

		err := appendTemplatedValues(appSetGenerator.ClusterProfiles.Values, params, appSet.Spec.GoTemplate, appSet.Spec.GoTemplateOptions)
		if err != nil {
			return nil, fmt.Errorf("error appending templated values for cluster: %w", err)
		}

		paramHolder.append(params)
		logCtx.WithField("cluster", cluster.Name).Debug("matched cluster profile")
	}
	if err := g.PruneSecrets(appSetGenerator, appSet); err != nil {
		return nil, fmt.Errorf("error pruning secrets: %w", err)
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

func (g *ClusterProfileGenerator) PruneSecrets(appSetGenerator *argoappsetv1alpha1.ApplicationSetGenerator, appSet *argoappsetv1alpha1.ApplicationSet) error {
	logCtx := log.WithField("applicationset", appSet.GetName()).WithField("namespace", appSet.GetNamespace())
	logCtx.Info("Pruning generated ClusterProfile secrets")

	// List all ClusterProfile objects
	clusterProfiles := &clusterinventory.ClusterProfileList{}
	if err := g.List(g.ctx, clusterProfiles); err != nil {
		return fmt.Errorf("error listing cluster profiles: %w", err)
	}

	managedSecrets := &corev1.SecretList{}
	if err := g.List(g.ctx, managedSecrets, client.InNamespace(g.namespace), client.HasLabels{managedByLabel}); err != nil {
		return fmt.Errorf("error listing managed secrets: %w", err)
	}

	for _, secret := range managedSecrets.Items {
		found := false
		for _, cluster := range clusterProfiles.Items {
			if secret.Annotations[clusterProfileOriginAnnotation] == fmt.Sprintf("%s/%s", cluster.Namespace, cluster.Name) {
				if g.matchesSelector(&cluster, &appSetGenerator.ClusterProfiles.Selector) {
					found = true
					break
				}
			}
		}

		if !found {
			logCtx.Infof("Pruning secret %s for cluster profile %s", secret.Name, secret.Annotations[clusterProfileOriginAnnotation])
			if err := g.Delete(g.ctx, &secret); err != nil {
				return fmt.Errorf("error pruning secret %s: %w", secret.Name, err)
			}
		}
	}
	return nil
}

func (g *ClusterProfileGenerator) createOrUpdateClusterSecret(ctx context.Context, cp *clusterinventory.ClusterProfile) error {
	logCtx := log.WithContext(ctx)
	// Get server URL
	if len(cp.Status.CredentialProviders) == 0 {
		return fmt.Errorf("cluster profile %s/%s has no credential providers", cp.Namespace, cp.Name)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cp.Name,
			Namespace: g.namespace,
		},
	}

	logCtx.Info("Reconciling secret", "name", cp.Name)
	if _, err := controllerutil.CreateOrUpdate(ctx, g.Client, secret, func() error {
		return g.mutateSecret(secret, cp)
	}); err != nil {
		return fmt.Errorf("failed to create/update secret: %w", err)
	}

	return nil
}

func (g *ClusterProfileGenerator) mutateSecret(secret *corev1.Secret, cp *clusterinventory.ClusterProfile) error {
	if secret.Labels == nil {
		secret.Labels = make(map[string]string)
	}
	secret.Labels[argoCDSecretType] = "cluster"

	if secret.Annotations == nil {
		secret.Annotations = make(map[string]string)
	}
	secret.Labels[managedByLabel] = "true"
	secret.Annotations[clusterProfileOriginAnnotation] = fmt.Sprintf("%s/%s", cp.Namespace, cp.Name)

	secretName := cp.Name
	serverURL := cp.Status.CredentialProviders[0].Cluster.Server
	credentialName := cp.Status.CredentialProviders[0].Name
	secret.Data = map[string][]byte{
		"name":   []byte(secretName),
		"server": []byte(serverURL),
		"config": []byte(fmt.Sprintf(clusterProfileSecretTemplate, credentialName)),
	}

	secret.Type = corev1.SecretTypeOpaque

	return nil
}
