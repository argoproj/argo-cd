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
	assert.Equal(t, fmt.Errorf(incorrectReturnType, "number"), err)
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

func TestGetPredefinedLuaScript(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	script, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.NotEmpty(t, script)
}

func TestGetNonExistentPredefinedLuaScript(t *testing.T) {
	testObj := StrToUnstructured(objWithNoScriptJSON)
	vm := VM{}
	script, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, "", script)
}
