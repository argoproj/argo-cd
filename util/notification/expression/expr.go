package expression

import (
	"github.com/argoproj-labs/argocd-notifications/expr/repo"
	"github.com/argoproj-labs/argocd-notifications/expr/strings"
	"github.com/argoproj-labs/argocd-notifications/expr/time"
	"github.com/argoproj-labs/argocd-notifications/shared/argocd"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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

func Spawn(app *unstructured.Unstructured, argocdService argocd.Service, vars map[string]interface{}) map[string]interface{} {
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
