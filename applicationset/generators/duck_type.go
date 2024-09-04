package generators

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v2/util/settings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"

	"github.com/argoproj/argo-cd/v2/applicationset/utils"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var _ Generator = (*DuckTypeGenerator)(nil)

// DuckTypeGenerator generates Applications for some or all clusters registered with ArgoCD.
type DuckTypeGenerator struct {
	ctx             context.Context
	dynClient       dynamic.Interface
	clientset       kubernetes.Interface
	namespace       string // namespace is the Argo CD namespace
	settingsManager *settings.SettingsManager
}

func NewDuckTypeGenerator(ctx context.Context, dynClient dynamic.Interface, clientset kubernetes.Interface, namespace string) Generator {
	settingsManager := settings.NewSettingsManager(ctx, clientset, namespace)

	g := &DuckTypeGenerator{
		ctx:             ctx,
		dynClient:       dynClient,
		clientset:       clientset,
		namespace:       namespace,
		settingsManager: settingsManager,
	}
	return g
}

func (g *DuckTypeGenerator) GetRequeueAfter(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) time.Duration {
	// Return a requeue default of 3 minutes, if no override is specified.

	if appSetGenerator.ClusterDecisionResource.RequeueAfterSeconds != nil {
		return time.Duration(*appSetGenerator.ClusterDecisionResource.RequeueAfterSeconds) * time.Second
	}

	return DefaultRequeueAfterSeconds
}

func (g *DuckTypeGenerator) GetTemplate(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator) *argoprojiov1alpha1.ApplicationSetTemplate {
	return &appSetGenerator.ClusterDecisionResource.Template
}

func (g *DuckTypeGenerator) GenerateParams(appSetGenerator *argoprojiov1alpha1.ApplicationSetGenerator, appSet *argoprojiov1alpha1.ApplicationSet, _ client.Client) ([]map[string]interface{}, error) {
	if appSetGenerator == nil {
		return nil, EmptyAppSetGeneratorError
	}

	// Not likely to happen
	if appSetGenerator.ClusterDecisionResource == nil {
		return nil, EmptyAppSetGeneratorError
	}

	// ListCluster from Argo CD's util/db package will include the local cluster in the list of clusters
	clustersFromArgoCD, err := utils.ListClusters(g.ctx, g.clientset, g.namespace)
	if err != nil {
		return nil, fmt.Errorf("error listing clusters: %w", err)
	}

	if clustersFromArgoCD == nil {
		return nil, nil
	}

	// Read the configMapRef
	cm, err := g.clientset.CoreV1().ConfigMaps(g.namespace).Get(g.ctx, appSetGenerator.ClusterDecisionResource.ConfigMapRef, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("error reading configMapRef: %w", err)
	}

	// Extract GVK data for the dynamic client to use
	versionIdx := strings.Index(cm.Data["apiVersion"], "/")
	kind := cm.Data["kind"]
	resourceName := appSetGenerator.ClusterDecisionResource.Name
	labelSelector := appSetGenerator.ClusterDecisionResource.LabelSelector

	log.WithField("kind.apiVersion", kind+"."+cm.Data["apiVersion"]).Info("Kind.Group/Version Reference")

	// Validate the fields
	if kind == "" || versionIdx < 1 {
		log.Warningf("kind=%v, resourceName=%v, versionIdx=%v", kind, resourceName, versionIdx)
		return nil, fmt.Errorf("There is a problem with the apiVersion, kind or resourceName provided")
	}

	if (resourceName == "" && labelSelector.MatchLabels == nil && labelSelector.MatchExpressions == nil) ||
		(resourceName != "" && (labelSelector.MatchExpressions != nil || labelSelector.MatchLabels != nil)) {
		log.Warningf("You must choose either resourceName=%v, labelSelector.matchLabels=%v or labelSelect.matchExpressions=%v", resourceName, labelSelector.MatchLabels, labelSelector.MatchExpressions)
		return nil, fmt.Errorf("There is a problem with the definition of the ClusterDecisionResource generator")
	}

	// Split up the apiVersion
	group := cm.Data["apiVersion"][0:versionIdx]
	version := cm.Data["apiVersion"][versionIdx+1:]
	log.WithField("kind.group.version", kind+"."+group+"/"+version).Debug("decoded Ref")

	duckGVR := schema.GroupVersionResource{Group: group, Version: version, Resource: kind}

	listOptions := metav1.ListOptions{}
	if resourceName == "" {
		listOptions.LabelSelector = metav1.FormatLabelSelector(&labelSelector)
		log.WithField("listOptions.LabelSelector", listOptions.LabelSelector).Info("selection type")
	} else {
		listOptions.FieldSelector = fields.OneTermEqualSelector("metadata.name", resourceName).String()
		// metav1.Convert_fields_Selector_To_string(fields.).Sprintf("metadata.name=%s", resourceName)
		log.WithField("listOptions.FieldSelector", listOptions.FieldSelector).Info("selection type")
	}

	duckResources, err := g.dynClient.Resource(duckGVR).Namespace(g.namespace).List(g.ctx, listOptions)
	if err != nil {
		log.WithField("GVK", duckGVR).Warning("resources were not found")
		return nil, fmt.Errorf("failed to get dynamic resources: %w", err)
	}

	if len(duckResources.Items) == 0 {
		log.Warning("no resource found, make sure you clusterDecisionResource is defined correctly")
		return nil, fmt.Errorf("no clusterDecisionResources found")
	}

	// Override the duck type in the status of the resource
	statusListKey := "clusters"

	matchKey := cm.Data["matchKey"]

	if cm.Data["statusListKey"] != "" {
		statusListKey = cm.Data["statusListKey"]
	}
	if matchKey == "" {
		log.WithField("matchKey", matchKey).Warning("matchKey not found in " + cm.Name)
		return nil, nil
	}

	res := []map[string]interface{}{}
	clusterDecisions := []interface{}{}

	// Build the decision slice
	for _, duckResource := range duckResources.Items {
		log.WithField("duckResourceName", duckResource.GetName()).Debug("found resource")

		if duckResource.Object["status"] == nil || len(duckResource.Object["status"].(map[string]interface{})) == 0 {
			log.Warningf("clusterDecisionResource: %s, has no status", duckResource.GetName())
			continue
		}

		log.WithField("duckResourceStatus", duckResource.Object["status"]).Debug("found resource")

		clusterDecisions = append(clusterDecisions, duckResource.Object["status"].(map[string]interface{})[statusListKey].([]interface{})...)
	}
	log.Infof("Number of decisions found: %v", len(clusterDecisions))

	// Read this outside the loop to improve performance
	argoClusters := clustersFromArgoCD.Items

	if len(clusterDecisions) > 0 {
		for _, cluster := range clusterDecisions {
			// generated instance of cluster params
			params := map[string]interface{}{}

			log.Infof("cluster: %v", cluster)
			matchValue := cluster.(map[string]interface{})[matchKey]
			if matchValue == nil || matchValue.(string) == "" {
				log.Warningf("matchKey=%v not found in \"%v\" list: %v\n", matchKey, statusListKey, cluster.(map[string]interface{}))
				continue
			}

			strMatchValue := matchValue.(string)
			log.WithField(matchKey, strMatchValue).Debug("validate against ArgoCD")

			found := false

			for _, argoCluster := range argoClusters {
				if argoCluster.Name == strMatchValue {
					log.WithField(matchKey, argoCluster.Name).Info("matched cluster in ArgoCD")
					params["name"] = argoCluster.Name
					params["server"] = argoCluster.Server

					found = true
					break // Stop looking
				}
			}

			if !found {
				log.WithField(matchKey, strMatchValue).Warning("unmatched cluster in ArgoCD")
				continue
			}

			for key, value := range cluster.(map[string]interface{}) {
				params[key] = value.(string)
			}

			for key, value := range appSetGenerator.ClusterDecisionResource.Values {
				if appSet.Spec.GoTemplate {
					if params["values"] == nil {
						params["values"] = map[string]string{}
					}
					params["values"].(map[string]string)[key] = value
				} else {
					params[fmt.Sprintf("values.%s", key)] = value
				}
			}

			res = append(res, params)
		}
	} else {
		log.Warningf("clusterDecisionResource status.%s missing", statusListKey)
		return nil, nil
	}

	return res, nil
}
