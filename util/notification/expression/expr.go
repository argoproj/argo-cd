package expression

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"

	"github.com/argoproj/argo-cd/v3/util/notification/expression/repo"
	"github.com/argoproj/argo-cd/v3/util/notification/expression/strings"
	"github.com/argoproj/argo-cd/v3/util/notification/expression/time"
)

var helpers = map[string]any{}

func init() {
	helpers = make(map[string]any)
	register("time", time.NewExprs())
	register("strings", strings.NewExprs())
}

func register(namespace string, entry map[string]any) {
	helpers[namespace] = entry
}

func Spawn(app *unstructured.Unstructured, argocdService service.Service, vars map[string]any) map[string]any {
	clone := make(map[string]any)
	for k := range vars {
		clone[k] = vars[k]
	}
	for namespace, helper := range helpers {
		clone[namespace] = helper
	}
	clone["repo"] = repo.NewExprs(argocdService, app)

	return clone
}
