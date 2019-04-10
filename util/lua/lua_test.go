package lua

import (
	"fmt"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

const objJSON = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  labels:
    app.kubernetes.io/instance: helm-guestbook
  name: helm-guestbook
  namespace: default
  resourceVersion: "123"
`
const objWithNoScriptJSON = `
apiVersion: not-an-endpoint.io/v1alpha1
kind: Test
metadata:
  labels:
    app.kubernetes.io/instance: helm-guestbook
  name: helm-guestbook
  namespace: default
  resourceVersion: "123"
`

const newHealthStatusFunction = `a = {}
a.status = "Healthy"
a.message ="NeedsToBeChanged"
if obj.metadata.name == "helm-guestbook" then
	a.message = "testMessage"
end
return a`

func StrToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestExecuteNewHealthStatusFunction(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	status, err := vm.ExecuteHealthLua(testObj, newHealthStatusFunction)
	assert.Nil(t, err)
	expectedHealthStatus := &appv1.HealthStatus{
		Status:  "Healthy",
		Message: "testMessage",
	}
	assert.Equal(t, expectedHealthStatus, status)

}

const osLuaScript = `os.getenv("HOME")`

func TestFailExternalLibCall(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	_, err := vm.ExecuteHealthLua(testObj, osLuaScript)
	assert.Error(t, err, "")
	assert.IsType(t, &lua.ApiError{}, err)
}

const returnInt = `return 1`

func TestFailLuaReturnNonTable(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	_, err := vm.ExecuteHealthLua(testObj, returnInt)
	assert.Equal(t, fmt.Errorf(incorrectReturnType, "table", "number"), err)
}

const invalidHealthStatusStatus = `local healthStatus = {}
healthStatus.status = "test"
return healthStatus
`

func TestInvalidHealthStatusStatus(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	status, err := vm.ExecuteHealthLua(testObj, invalidHealthStatusStatus)
	assert.Nil(t, err)
	expectedStatus := &appv1.HealthStatus{
		Status:  appv1.HealthStatusUnknown,
		Message: invalidHealthStatus,
	}
	assert.Equal(t, expectedStatus, status)
}

const infiniteLoop = `while true do ; end`

func TestHandleInfiniteLoop(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	_, err := vm.ExecuteHealthLua(testObj, infiniteLoop)
	assert.IsType(t, &lua.ApiError{}, err)
}

func TestGetHealthScriptWithOverride(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/Rollout": {
				HealthLua: newHealthStatusFunction,
			},
		},
	}
	script, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, newHealthStatusFunction, script)
}

func TestGetHealthScriptPredefined(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	script, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.NotEmpty(t, script)
}

func TestGetHealthScriptNoPredefined(t *testing.T) {
	testObj := StrToUnstructured(objWithNoScriptJSON)
	vm := VM{}
	script, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, "", script)
}

func TestGetCustomActionPredefined(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}

	customAction, err := vm.GetCustomAction(testObj, "resume")
	assert.Nil(t, err)
	assert.NotEmpty(t, customAction)
}

func TestGetCustomActionNoPredefined(t *testing.T) {
	testObj := StrToUnstructured(objWithNoScriptJSON)
	vm := VM{}
	customAction, err := vm.GetCustomAction(testObj, "test")
	assert.Nil(t, err)
	assert.Empty(t, customAction.ActionLua)
}

func TestGetCustomActionWithOverride(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	test := appv1.ResourceActionDefinition{
		Name:      "test",
		ActionLua: "return obj",
	}

	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/Rollout": {
				Actions: appv1.ResourceActions{
					Definitions: []appv1.ResourceActionDefinition{
						test,
					},
				},
			},
		},
	}
	action, err := vm.GetCustomAction(testObj, "test")
	assert.Nil(t, err)
	assert.Equal(t, test, action)
}

func TestGetResourceActionDiscoveryPredefined(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}

	discoveryLua, err := vm.GetCustomActionDiscovery(testObj)
	assert.Nil(t, err)
	assert.NotEmpty(t, discoveryLua)
}

func TestGetCustomActionDiscoveryNoPredefined(t *testing.T) {
	testObj := StrToUnstructured(objWithNoScriptJSON)
	vm := VM{}
	discoveryLua, err := vm.GetCustomActionDiscovery(testObj)
	assert.Nil(t, err)
	assert.Empty(t, discoveryLua)
}

func TestGetCustomActionDiscoveryWithOverride(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/Rollout": {
				Actions: appv1.ResourceActions{
					ActionDiscoveryLua: validDiscoveryLua,
				},
			},
		},
	}
	discoveryLua, err := vm.GetCustomActionDiscovery(testObj)
	assert.Nil(t, err)
	assert.Equal(t, validDiscoveryLua, discoveryLua)
}

const validDiscoveryLua = `
scaleParam = {}
scaleParam['name'] = 'replicas'
scaleParam['type'] = 'number'

scaleParams = {}
scaleParams[1] = scaleParam

scale = {}
scale['name'] = 'scale'
scale['params'] = scaleParams

resume = {}
resume['name'] = 'resume'
resume['ignoresIrrelevant'] = 'fields'

a = {}
a[1] = scale
a[2] = resume
return a
`

func TestExecuteCustomActionDiscovery(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	customActions, err := vm.ExecuteCustomActionDiscovery(testObj, validDiscoveryLua)
	assert.Nil(t, err)
	expectedActions := []appv1.ResourceAction{
		{
			Name: "scale",
			Params: []appv1.ResourceActionParam{{
				Name: "replicas",
				Type: "number",
			}},
		}, {
			Name: "resume",
		},
	}
	assert.Equal(t, expectedActions, customActions)
}

const invalidDiscoveryLua = `
a = 1
return a
`

func TestExecuteCustomActionDiscoveryInvalidReturn(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	customActions, err := vm.ExecuteCustomActionDiscovery(testObj, invalidDiscoveryLua)
	assert.Nil(t, customActions)
	assert.Error(t, err)

}

const validActionLua = `
obj.metadata.labels["test"] = "test"
return obj
`

const expectedUpdatedObj = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  labels:
    app.kubernetes.io/instance: helm-guestbook
    test: test
  name: helm-guestbook
  namespace: default
  resourceVersion: "123"
`

func TestExecuteCustomAction(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	expectedObj := StrToUnstructured(expectedUpdatedObj)
	vm := VM{}
	newObj, err := vm.ExecuteCustomAction(testObj, validActionLua)
	assert.Nil(t, err)
	assert.Equal(t, expectedObj, newObj)
}

func TestExecuteCustomActionNonTableReturn(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	_, err := vm.ExecuteCustomAction(testObj, returnInt)
	assert.Errorf(t, err, incorrectReturnType, "table", "number")
}

const invalidTableReturn = `newObj = {}
newObj["test"] = "test"
return newObj
`

func TestExecuteCustomActionInvalidUnstructured(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	_, err := vm.ExecuteCustomAction(testObj, invalidTableReturn)
	assert.Error(t, err)
}