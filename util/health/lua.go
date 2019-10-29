package health

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/lua"
)

func getResourceHealthFromLuaScript(obj *unstructured.Unstructured, resourceOverrides map[string]appv1.ResourceOverride) (*appv1.HealthStatus, error) {
	luaVM := lua.VM{
		ResourceOverrides: resourceOverrides,
	}
	script, err := luaVM.GetHealthScript(obj)
	if err != nil {
		return nil, err
	}
	if script == "" {
		return nil, nil
	}
	result, err := luaVM.ExecuteHealthLua(obj, script)
	if err != nil {
		return nil, err
	}
	return result, nil
}
