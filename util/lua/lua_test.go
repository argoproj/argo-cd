package lua

import (
	"fmt"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/grpc"
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
	expectedHealthStatus := &health.HealthStatus{
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
	expectedStatus := &health.HealthStatus{
		Status:  health.HealthStatusUnknown,
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

func TestGetResourceActionPredefined(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}

	action, err := vm.GetResourceAction(testObj, "resume")
	assert.Nil(t, err)
	assert.NotEmpty(t, action)
}

func TestGetResourceActionNoPredefined(t *testing.T) {
	testObj := StrToUnstructured(objWithNoScriptJSON)
	vm := VM{}
	action, err := vm.GetResourceAction(testObj, "test")
	assert.Nil(t, err)
	assert.Empty(t, action.ActionLua)
}

func TestGetResourceActionWithOverride(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	test := appv1.ResourceActionDefinition{
		Name:      "test",
		ActionLua: "return obj",
	}

	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/Rollout": {
				Actions: string(grpc.MustMarshal(appv1.ResourceActions{
					Definitions: []appv1.ResourceActionDefinition{
						test,
					},
				})),
			},
		},
	}
	action, err := vm.GetResourceAction(testObj, "test")
	assert.Nil(t, err)
	assert.Equal(t, test, action)
}

func TestGetResourceActionDiscoveryPredefined(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}

	discoveryLua, err := vm.GetResourceActionDiscovery(testObj)
	assert.Nil(t, err)
	assert.NotEmpty(t, discoveryLua)
}

func TestGetResourceActionDiscoveryNoPredefined(t *testing.T) {
	testObj := StrToUnstructured(objWithNoScriptJSON)
	vm := VM{}
	discoveryLua, err := vm.GetResourceActionDiscovery(testObj)
	assert.Nil(t, err)
	assert.Empty(t, discoveryLua)
}

func TestGetResourceActionDiscoveryWithOverride(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/Rollout": {
				Actions: string(grpc.MustMarshal(appv1.ResourceActions{
					ActionDiscoveryLua: validDiscoveryLua,
				})),
			},
		},
	}
	discoveryLua, err := vm.GetResourceActionDiscovery(testObj)
	assert.Nil(t, err)
	assert.Equal(t, validDiscoveryLua, discoveryLua)
}

const validDiscoveryLua = `
scaleParams = { {name = "replicas", type = "number"} }
scale = {name = 'scale', params = scaleParams}

resume = {name = 'resume'}

test = {}
a = {scale = scale, resume = resume, test = test}

return a
`

func TestExecuteResourceActionDiscovery(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	actions, err := vm.ExecuteResourceActionDiscovery(testObj, validDiscoveryLua)
	assert.Nil(t, err)
	expectedActions := []appv1.ResourceAction{
		{
			Name: "resume",
		}, {
			Name: "scale",
			Params: []appv1.ResourceActionParam{{
				Name: "replicas",
				Type: "number",
			}},
		}, {
			Name: "test",
		},
	}
	for _, expectedAction := range expectedActions {
		assert.Contains(t, actions, expectedAction)
	}
}

const discoveryLuaWithInvalidResourceAction = `
resume = {name = 'resume', invalidField: "test""}
a = {resume = resume}
return a`

func TestExecuteResourceActionDiscoveryInvalidResourceAction(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	actions, err := vm.ExecuteResourceActionDiscovery(testObj, discoveryLuaWithInvalidResourceAction)
	assert.Error(t, err)
	assert.Nil(t, actions)
}

const invalidDiscoveryLua = `
a = 1
return a
`

func TestExecuteResourceActionDiscoveryInvalidReturn(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	actions, err := vm.ExecuteResourceActionDiscovery(testObj, invalidDiscoveryLua)
	assert.Nil(t, actions)
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

func TestExecuteResourceAction(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	expectedObj := StrToUnstructured(expectedUpdatedObj)
	vm := VM{}
	newObj, err := vm.ExecuteResourceAction(testObj, validActionLua)
	assert.Nil(t, err)
	assert.Equal(t, expectedObj, newObj)
}

func TestExecuteResourceActionNonTableReturn(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	_, err := vm.ExecuteResourceAction(testObj, returnInt)
	assert.Errorf(t, err, incorrectReturnType, "table", "number")
}

const invalidTableReturn = `newObj = {}
newObj["test"] = "test"
return newObj
`

func TestExecuteResourceActionInvalidUnstructured(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	_, err := vm.ExecuteResourceAction(testObj, invalidTableReturn)
	assert.Error(t, err)
}

const objWithEmptyStruct = `
apiVersion: argoproj.io/v1alpha1
kind: Test
metadata:
  labels:
    app.kubernetes.io/instance: helm-guestbook
    test: test
  name: helm-guestbook
  namespace: default
  resourceVersion: "123"
spec:
  resources: {}
  paused: true
  containers:
   - name: name1
     test: {}
     anotherList:
     - name: name2
       test2: {}
`

const expectedUpdatedObjWithEmptyStruct = `
apiVersion: argoproj.io/v1alpha1
kind: Test
metadata:
  labels:
    app.kubernetes.io/instance: helm-guestbook
    test: test
  name: helm-guestbook
  namespace: default
  resourceVersion: "123"
spec:
  resources: {}
  paused: false
  containers:
   - name: name1
     test: {}
     anotherList:
     - name: name2
       test2: {}
`

const pausedToFalseLua = `
obj.spec.paused = false
return obj
`

func TestCleanPatch(t *testing.T) {
	testObj := StrToUnstructured(objWithEmptyStruct)
	expectedObj := StrToUnstructured(expectedUpdatedObjWithEmptyStruct)
	vm := VM{}
	newObj, err := vm.ExecuteResourceAction(testObj, pausedToFalseLua)
	assert.Nil(t, err)
	assert.Equal(t, expectedObj, newObj)

}
