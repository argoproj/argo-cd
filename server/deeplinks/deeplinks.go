package deeplinks

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/antonmedv/expr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	ResourceDeepLinkKey = "resource"
	AppDeepLinkKey      = "application"
	ClusterDeepLinkKey  = "cluster"
	ProjectDeepLinkKey  = "project"
)

func CreateDeepLinksObject(resourceObj *unstructured.Unstructured, app *unstructured.Unstructured, cluster *unstructured.Unstructured, project *unstructured.Unstructured) map[string]interface{} {
	deeplinkObj := map[string]interface{}{}
	if resourceObj != nil {
		deeplinkObj[ResourceDeepLinkKey] = resourceObj.Object
	}
	if app != nil {
		deeplinkObj[AppDeepLinkKey] = app.Object
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
		t, err := template.New("deep-link").Parse(link.URL)
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
		if link.Condition != nil {
			out, err := expr.Eval(*link.Condition, obj)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to evaluate link condition '%v' with resource %v, error=%v", *link.Condition, name, err.Error()))
				continue
			}
			switch resOut := out.(type) {
			case bool:
				if resOut {
					finalLinks = append(finalLinks, &application.LinkInfo{
						Title:       pointer.String(link.Title),
						Url:         pointer.String(finalURL.String()),
						Description: link.Description,
						IconClass:   link.IconClass,
					})
				}
			default:
				errors = append(errors, fmt.Sprintf("link condition '%v' evaluated to non-boolean value for resource %v", *link.Condition, name))
				continue
			}
		} else {
			finalLinks = append(finalLinks, &application.LinkInfo{
				Title:       pointer.String(link.Title),
				Url:         pointer.String(finalURL.String()),
				Description: link.Description,
				IconClass:   link.IconClass,
			})
		}
	}
	return &application.LinksResponse{
		Items: finalLinks,
	}, errors
}
