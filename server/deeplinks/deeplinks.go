package deeplinks

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/expr-lang/expr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

var sprigFuncMap = sprig.GenericFuncMap() // a singleton for better performance

func init() {
	// Avoid allowing the user to learn things about the environment.
	delete(sprigFuncMap, "env")
	delete(sprigFuncMap, "expandenv")
	delete(sprigFuncMap, "getHostByName")
}

const (
	ResourceDeepLinkKey = "resource"
	AppDeepLinkKey      = "application"
	AppDeepLinkShortKey = "app"
	ClusterDeepLinkKey  = "cluster"
	ProjectDeepLinkKey  = "project"
)

type ClusterLinksData struct {
	// Server is the API server URL of the Kubernetes cluster
	Server string `json:"server" protobuf:"bytes,1,opt,name=server"`
	// Name of the cluster. If omitted, will use the server address
	Name string `json:"name" protobuf:"bytes,2,opt,name=name"`
	// Holds list of namespaces which are accessible in that cluster. Cluster level resources will be ignored if namespace list is not empty.
	Namespaces []string `json:"namespaces,omitempty" protobuf:"bytes,6,opt,name=namespaces"`
	// Shard contains optional shard number. Calculated on the fly by the application controller if not specified.
	Shard *int64 `json:"shard,omitempty" protobuf:"bytes,9,opt,name=shard"`
	// Reference between project and cluster that allow you automatically to be added as item inside Destinations project entity
	Project string `json:"project,omitempty" protobuf:"bytes,11,opt,name=project"`
	// Labels for cluster secret metadata
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,12,opt,name=labels"`
	// Annotations for cluster secret metadata
	Annotations map[string]string `json:"annotations,omitempty" protobuf:"bytes,13,opt,name=annotations"`
}

func SanitizeCluster(cluster *v1alpha1.Cluster) (*unstructured.Unstructured, error) {
	return kube.ToUnstructured(&ClusterLinksData{
		Server:      cluster.Server,
		Name:        cluster.Name,
		Namespaces:  cluster.Namespaces,
		Shard:       cluster.Shard,
		Project:     cluster.Project,
		Labels:      cluster.Labels,
		Annotations: cluster.Annotations,
	})
}

func CreateDeepLinksObject(resourceObj *unstructured.Unstructured, app *unstructured.Unstructured, cluster *unstructured.Unstructured, project *unstructured.Unstructured) map[string]interface{} {
	deeplinkObj := map[string]interface{}{}
	if resourceObj != nil {
		deeplinkObj[ResourceDeepLinkKey] = resourceObj.Object
	}
	if app != nil {
		deeplinkObj[AppDeepLinkKey] = app.Object
		deeplinkObj[AppDeepLinkShortKey] = app.Object
	}
	if cluster != nil {
		deeplinkObj[ClusterDeepLinkKey] = cluster.Object
	}
	if project != nil {
		deeplinkObj[ProjectDeepLinkKey] = project.Object
	}
	return deeplinkObj
}

func EvaluateDeepLinksResponse(obj map[string]interface{}, name string, links []settings.DeepLink) (*application.LinksResponse, []string) {
	finalLinks := []*application.LinkInfo{}
	errors := []string{}
	for _, link := range links {
		if link.Condition != nil {
			out, err := expr.Eval(*link.Condition, obj)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to evaluate link condition '%v' with resource %v, error=%v", *link.Condition, name, err.Error()))
				continue
			}
			switch condResult := out.(type) {
			case bool:
				if !condResult {
					continue
				}
			default:
				errors = append(errors, fmt.Sprintf("link condition '%v' evaluated to non-boolean value for resource %v", *link.Condition, name))
				continue
			}
		}

		t, err := template.New("deep-link").Funcs(sprigFuncMap).Parse(link.URL)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to parse link template '%v', error=%v", link.URL, err.Error()))
			continue
		}
		finalURL := bytes.Buffer{}
		err = t.Execute(&finalURL, obj)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to evaluate link template '%v' with resource %v, error=%v", link.URL, name, err.Error()))
			continue
		}

		finalLinks = append(finalLinks, &application.LinkInfo{
			Title:       ptr.To(link.Title),
			Url:         ptr.To(finalURL.String()),
			Description: link.Description,
			IconClass:   link.IconClass,
		})
	}
	return &application.LinksResponse{
		Items: finalLinks,
	}, errors
}
