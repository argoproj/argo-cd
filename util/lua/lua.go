package lua

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
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
	actionDiscoveryScriptFile        = "discovery.lua"
)

var (
	box packr.Box
)

func init() {
	box = packr.NewBox(resourceCustomizationBuiltInPath)
}

type ResourceHealthOverrides map[string]appv1.ResourceOverride

func (overrides ResourceHealthOverrides) GetResourceHealth(obj *unstructured.Unstructured) (*health.HealthStatus, error) {
	luaVM := VM{
		ResourceOverrides: overrides,
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
	// Opens table library to allow access to functions to manipulate tables
	for _, pair := range []struct {
		n string
		f lua.LGFunction
	}{
		{lua.LoadLibName, lua.OpenPackage},
		{lua.BaseLibName, lua.OpenBase},
		{lua.TabLibName, lua.OpenTable},
		// load our 'safe' version of the os library
		{lua.OsLibName, OpenSafeOs},
	} {
		if err := l.CallByParam(lua.P{
			Fn:      l.NewFunction(pair.f),
			NRet:    0,
			Protect: true,
		}, lua.LString(pair.n)); err != nil {
			panic(err)
		}
	}
	// preload our 'safe' version of the os library. Allows the 'local os = require("os")' to work
	l.PreloadModule(lua.OsLibName, SafeOsLoader)

	// preload json library to parse json in lua
	luajson.Preload(l)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	l.SetContext(ctx)
	objectValue := decodeValue(l, obj.Object)
	l.SetGlobal("obj", objectValue)
	err := l.DoString(script)
	return l, err
}

// ExecuteHealthLua runs the lua script to generate the health status of a resource
func (vm VM) ExecuteHealthLua(obj *unstructured.Unstructured, script string) (*health.HealthStatus, error) {
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
		healthStatus := &health.HealthStatus{}
		err = json.Unmarshal(jsonBytes, healthStatus)
		if err != nil {
			return nil, err
		}
		if !isValidHealthStatusCode(healthStatus.Status) {
			return &health.HealthStatus{
				Status:  health.HealthStatusUnknown,
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

func (vm VM) ExecuteResourceAction(obj *unstructured.Unstructured, script string) (*unstructured.Unstructured, error) {
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
		cleanedNewObj := cleanReturnedObj(newObj.Object, obj.Object)
		newObj.Object = cleanedNewObj
		return newObj, nil
	}
	return nil, fmt.Errorf(incorrectReturnType, "table", returnValue.Type().String())
}

// cleanReturnedObj Lua cannot distinguish an empty table as an array or map, and the library we are using choose to
// decoded an empty table into an empty array. This function prevents the lua scripts from unintentionally changing an
// empty struct into empty arrays
func cleanReturnedObj(newObj, obj map[string]interface{}) map[string]interface{} {
	mapToReturn := newObj
	for key := range obj {
		if newValueInterface, ok := newObj[key]; ok {
			oldValueInterface, ok := obj[key]
			if !ok {
				continue
			}
			switch newValue := newValueInterface.(type) {
			case map[string]interface{}:
				if oldValue, ok := oldValueInterface.(map[string]interface{}); ok {
					convertedMap := cleanReturnedObj(newValue, oldValue)
					mapToReturn[key] = convertedMap
				}

			case []interface{}:
				switch oldValue := oldValueInterface.(type) {
				case map[string]interface{}:
					if len(newValue) == 0 {
						mapToReturn[key] = oldValue
					}
				case []interface{}:
					newArray := cleanReturnedArray(newValue, oldValue)
					mapToReturn[key] = newArray
				}
			}
		}
	}
	return mapToReturn
}

// cleanReturnedArray allows Argo CD to recurse into nested arrays when checking for unintentional empty struct to
// empty array conversions.
func cleanReturnedArray(newObj, obj []interface{}) []interface{} {
	arrayToReturn := newObj
	for i := range newObj {
		switch newValue := newObj[i].(type) {
		case map[string]interface{}:
			if oldValue, ok := obj[i].(map[string]interface{}); ok {
				convertedMap := cleanReturnedObj(newValue, oldValue)
				arrayToReturn[i] = convertedMap
			}
		case []interface{}:
			if oldValue, ok := obj[i].([]interface{}); ok {
				convertedMap := cleanReturnedArray(newValue, oldValue)
				arrayToReturn[i] = convertedMap
			}
		}
	}
	return arrayToReturn
}

func (vm VM) ExecuteResourceActionDiscovery(obj *unstructured.Unstructured, script string) ([]appv1.ResourceAction, error) {
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
		if noAvailableActions(jsonBytes) {
			return availableActions, nil
		}
		availableActionsMap := make(map[string]interface{})
		err = json.Unmarshal(jsonBytes, &availableActionsMap)
		if err != nil {
			return nil, err
		}
		for key := range availableActionsMap {
			value := availableActionsMap[key]
			resourceAction := appv1.ResourceAction{Name: key, Disabled: isActionDisabled(value)}
			if emptyResourceActionFromLua(value) {
				availableActions = append(availableActions, resourceAction)
				continue
			}
			resourceActionBytes, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}

			err = json.Unmarshal(resourceActionBytes, &resourceAction)
			if err != nil {
				return nil, err
			}
			availableActions = append(availableActions, resourceAction)
		}
		return availableActions, err
	}

	return nil, fmt.Errorf(incorrectReturnType, "table", returnValue.Type().String())
}

// Actions are enabled by default
func isActionDisabled(actionsMap interface{}) bool {
	actions, ok := actionsMap.(map[string]interface{})
	if !ok {
		return false
	}
	for key, val := range actions {
		switch vv := val.(type) {
		case bool:
			if key == "disabled" {
				return vv
			}
		}
	}
	return false
}

func emptyResourceActionFromLua(i interface{}) bool {
	_, ok := i.([]interface{})
	return ok
}

func noAvailableActions(jsonBytes []byte) bool {
	// When the Lua script returns an empty table, it is decoded as a empty array.
	return string(jsonBytes) == "[]"
}

func (vm VM) GetResourceActionDiscovery(obj *unstructured.Unstructured) (string, error) {
	key := getConfigMapKey(obj)
	override, ok := vm.ResourceOverrides[key]
	if ok && override.Actions != "" {
		actions, err := override.GetActions()
		if err != nil {
			return "", err
		}
		return actions.ActionDiscoveryLua, nil
	}
	discoveryKey := fmt.Sprintf("%s/actions/", key)
	discoveryScript, err := vm.getPredefinedLuaScripts(discoveryKey, actionDiscoveryScriptFile)
	if err != nil {
		return "", err
	}
	return discoveryScript, nil
}

// GetResourceAction attempts to read lua script from config and then filesystem for that resource
func (vm VM) GetResourceAction(obj *unstructured.Unstructured, actionName string) (appv1.ResourceActionDefinition, error) {
	key := getConfigMapKey(obj)
	override, ok := vm.ResourceOverrides[key]
	if ok && override.Actions != "" {
		actions, err := override.GetActions()
		if err != nil {
			return appv1.ResourceActionDefinition{}, err
		}
		for _, action := range actions.Definitions {
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

func isValidHealthStatusCode(statusCode health.HealthStatusCode) bool {
	switch statusCode {
	case health.HealthStatusUnknown, health.HealthStatusProgressing, health.HealthStatusSuspended, health.HealthStatusHealthy, health.HealthStatusDegraded, health.HealthStatusMissing:
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
