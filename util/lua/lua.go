package lua

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gobuffalo/packr"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	luajson "layeh.com/gopher-json"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

const (
	incorrectReturnType              = "expect %s output from Lua script, not %s"
	invalidHealthStatus              = "Lua returned an invalid health status"
	resourceCustomizationBuiltInPath = "../../resource_customizations"
	healthScriptFile                 = "health.lua"
	actionScriptFile                 = "action.lua"
	actionDiscoveryScriptFile        = "actionDiscovery.lua"
)

var (
	box packr.Box
)

func init() {
	box = packr.NewBox(resourceCustomizationBuiltInPath)
}

// VM Defines a struct that implements the luaVM
type VM struct {
	ResourceOverrides map[string]appv1.ResourceOverride
	// UseOpenLibs flag to enable open libraries. Libraries are always disabled while running, but enabled during testing to allow the use of print statements
	UseOpenLibs bool
}

func (vm VM) runLua(obj *unstructured.Unstructured, script string) (*lua.LState, error) {
	l := lua.NewState(lua.Options{
		SkipOpenLibs: !vm.UseOpenLibs,
	})
	defer l.Close()
	// Opens table library to allow access to functions to manulate tables
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
	} {
		if err := l.CallByParam(lua.P{
			Fn:      l.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			panic(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)
	objectValue := decodeValue(l, obj.Object)
	l.SetGlobal("obj", objectValue)
	err := l.DoString(script)
	return l, err
}

// ExecuteHealthLua runs the lua script to generate the health status of a resource
func (vm VM) ExecuteHealthLua(obj *unstructured.Unstructured, script string) (*appv1.HealthStatus, error) {
	l, err := vm.runLua(obj, script)
	if err != nil {
		return nil, err
	}
	returnValue := l.Get(-1)
	if returnValue.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(returnValue)
		if err != nil {
			return nil, err
		}
		healthStatus := &appv1.HealthStatus{}
		err = json.Unmarshal(jsonBytes, healthStatus)
		if err != nil {
			return nil, err
		}
		if !isValidHealthStatusCode(healthStatus.Status) {
			return &appv1.HealthStatus{
				Status:  appv1.HealthStatusUnknown,
				Message: invalidHealthStatus,
			}, nil
		}

		return healthStatus, nil
	}
	return nil, fmt.Errorf(incorrectReturnType, "table", returnValue.Type().String())
}

// GetHealthScript attempts to read lua script from config and then filesystem for that resource
func (vm VM) GetHealthScript(obj *unstructured.Unstructured) (string, error) {
	key := getConfigMapKey(obj)
	if script, ok := vm.ResourceOverrides[key]; ok && script.HealthLua != "" {
		return script.HealthLua, nil
	}
	return vm.getPredefinedLuaScripts(key, healthScriptFile)
}

func (vm VM) ExecuteCustomAction(obj *unstructured.Unstructured, script string) (*unstructured.Unstructured, error) {
	l, err := vm.runLua(obj, script)
	if err != nil {
		return nil, err
	}
	returnValue := l.Get(-1)
	if returnValue.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(returnValue)
		if err != nil {
			return nil, err
		}
		newObj, err := appv1.UnmarshalToUnstructured(string(jsonBytes))
		if err != nil {
			return nil, err
		}
		return newObj, nil
	}
	return nil, fmt.Errorf(incorrectReturnType, "table", returnValue.Type().String())
}

func (vm VM) ExecuteCustomActionDiscovery(obj *unstructured.Unstructured, script string) ([]appv1.ResourceAction, error) {
	l, err := vm.runLua(obj, script)
	if err != nil {
		return nil, err
	}
	returnValue := l.Get(-1)
	if returnValue.Type() == lua.LTTable {
		jsonBytes, err := luajson.Encode(returnValue)
		if err != nil {
			return nil, err
		}
		availableActions := make([]appv1.ResourceAction, 0)
		err = json.Unmarshal(jsonBytes, &availableActions)
		return availableActions, err
	}

	return nil, fmt.Errorf(incorrectReturnType, "table", returnValue.Type().String())
}

func (vm VM) GetCustomActionDiscovery(obj *unstructured.Unstructured) (string, error) {
	key := getConfigMapKey(obj)
	override, ok := vm.ResourceOverrides[key]
	if ok {
		return override.Actions.ActionDiscoveryLua, nil
	}
	discoveryKey := fmt.Sprintf("%s/actions/", key)
	discoveryScript, err := vm.getPredefinedLuaScripts(discoveryKey, actionDiscoveryScriptFile)
	if err != nil {
		return "", err
	}
	return discoveryScript, nil
}

// GetCustomAction attempts to read lua script from config and then filesystem for that resource
func (vm VM) GetCustomAction(obj *unstructured.Unstructured, actionName string) (appv1.ResourceActionDefinition, error) {
	key := getConfigMapKey(obj)
	override, ok := vm.ResourceOverrides[key]
	if ok {
		for _, action := range override.Actions.Definitions {
			if action.Name == actionName {
				return action, nil
			}
		}
	}

	actionKey := fmt.Sprintf("%s/actions/%s", key, actionName)
	actionScript, err := vm.getPredefinedLuaScripts(actionKey, actionScriptFile)
	if err != nil {
		return appv1.ResourceActionDefinition{}, err
	}

	return appv1.ResourceActionDefinition{
		Name:      actionName,
		ActionLua: actionScript,
	}, nil
}

func getConfigMapKey(obj *unstructured.Unstructured) string {
	gvk := obj.GroupVersionKind()
	if gvk.Group == "" {
		return gvk.Kind
	}
	return fmt.Sprintf("%s/%s", gvk.Group, gvk.Kind)

}

func (vm VM) getPredefinedLuaScripts(objKey string, scriptFile string) (string, error) {
	data, err := box.MustBytes(filepath.Join(objKey, scriptFile))
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

func isValidHealthStatusCode(statusCode string) bool {
	switch statusCode {
	case appv1.HealthStatusUnknown, appv1.HealthStatusProgressing, appv1.HealthStatusSuspended, appv1.HealthStatusHealthy, appv1.HealthStatusDegraded, appv1.HealthStatusMissing:
		return true
	}
	return false
}

// Took logic from the link below and added the int, int32, and int64 types since the value would have type int64
// while actually running in the controller and it was not reproducible through testing.
// https://github.com/layeh/gopher-json/blob/97fed8db84274c421dbfffbb28ec859901556b97/json.go#L154
func decodeValue(L *lua.LState, value interface{}) lua.LValue {
	switch converted := value.(type) {
	case bool:
		return lua.LBool(converted)
	case float64:
		return lua.LNumber(converted)
	case string:
		return lua.LString(converted)
	case json.Number:
		return lua.LString(converted)
	case int:
		return lua.LNumber(converted)
	case int32:
		return lua.LNumber(converted)
	case int64:
		return lua.LNumber(converted)
	case []interface{}:
		arr := L.CreateTable(len(converted), 0)
		for _, item := range converted {
			arr.Append(decodeValue(L, item))
		}
		return arr
	case map[string]interface{}:
		tbl := L.CreateTable(0, len(converted))
		for key, item := range converted {
			tbl.RawSetH(lua.LString(key), decodeValue(L, item))
		}
		return tbl
	case nil:
		return lua.LNil
	}

	return lua.LNil
}
