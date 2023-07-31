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

func EvaluateDeepLinksResponse(obj unstructured.Unstructured, links []settings.DeepLink) (*application.LinksResponse, []string) {
	finalLinks := []*application.LinkInfo{}
	errors := []string{}
	for _, link := range links {
		t, err := template.New("deep-link").Parse(link.URL)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to parse link template '%v', error=%v", link.URL, err.Error()))
			continue
		}
		finalURL := bytes.Buffer{}
		err = t.Execute(&finalURL, obj.Object)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to evaluate link template '%v' with resource %v, error=%v", link.URL, obj.GetName(), err.Error()))
			continue
		}
		if link.Condition != nil {
			out, err := expr.Eval(*link.Condition, obj.Object)
			if err != nil {
				errors = append(errors, fmt.Sprintf("failed to evaluate link condition '%v' with resource %v, error=%v", *link.Condition, obj.GetName(), err.Error()))
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
				errors = append(errors, fmt.Sprintf("link condition '%v' evaluated to non-boolean value for resource %v", *link.Condition, obj.GetName()))
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
