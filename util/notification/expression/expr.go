package expression

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"

	"github.com/argoproj/argo-cd/v2/util/notification/expression/repo"
	"github.com/argoproj/argo-cd/v2/util/notification/expression/strings"
	"github.com/argoproj/argo-cd/v2/util/notification/expression/time"
)

var helpers = map[string]interface{}{}

func init() {
	helpers = make(map[string]interface{})
	register("time", time.NewExprs())
	register("strings", strings.NewExprs())
}

func register(namespace string, entry map[string]interface{}) {
	helpers[namespace] = entry
}

func Spawn(app *unstructured.Unstructured, argocdService service.Service, vars map[string]interface{}) map[string]interface{} {
	clone := make(map[string]interface{})
	for k := range vars {
		clone[k] = vars[k]
	}
	for namespace, helper := range helpers {
		clone[namespace] = helper
	}
	clone["repo"] = repo.NewExprs(argocdService, app)

	return clone
}
