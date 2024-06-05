package lua

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/stretchr/testify/assert"
	lua "github.com/yuin/gopher-lua"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/grpc"
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
const rolloutJSON = `
apiVersion: argoproj.io/v1alpha1
kind: Rollout
metadata:
  name: istio-host-split
  namespace: testns1
spec:
  replicas: 1
  strategy:
    canary:
      analysis:
        analysisRunMetadata: {}
status:
  canary:
    weights:
      canary:
        podTemplateHash: 7766c75d45
        serviceName: istio-host-split-canary
        weight: 0
      stable:
        podTemplateHash: 7766c75d45
        serviceName: istio-host-split-stable
        weight: 100
  currentPodHash: 7766c75d45
  currentStepHash: 85659df978
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

const ec2AWSCrossplaneObjJson = `
apiVersion: ec2.aws.crossplane.io/v1alpha1
kind: Instance
metadata:
  name: sample-crosspalne-ec2-instance
spec:
  forProvider:
    region: us-west-2
    instanceType: t2.micro
    keyName: my-crossplane-key-pair
  providerConfigRef:
    name: awsconfig
`

const newHealthStatusFunction = `a = {}
a.status = "Healthy"
a.message ="NeedsToBeChanged"
if obj.metadata.name == "helm-guestbook" then
	a.message = "testMessage"
end
return a`

const newWildcardHealthStatusFunction = `a = {}
a.status = "Healthy"
a.message ="NeedsToBeChanged"
if obj.metadata.name == "sample-crosspalne-ec2-instance" then
	a.message = "testWildcardMessage"
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

func TestExecuteWildcardHealthStatusFunction(t *testing.T) {
	testObj := StrToUnstructured(ec2AWSCrossplaneObjJson)
	vm := VM{}
	status, err := vm.ExecuteHealthLua(testObj, newWildcardHealthStatusFunction)
	assert.Nil(t, err)
	expectedHealthStatus := &health.HealthStatus{
		Status:  "Healthy",
		Message: "testWildcardMessage",
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
				HealthLua:   newHealthStatusFunction,
				UseOpenLibs: false,
			},
		},
	}
	script, useOpenLibs, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, false, useOpenLibs)
	assert.Equal(t, newHealthStatusFunction, script)
}

func TestGetHealthScriptWithKindWildcardOverride(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/*": {
				HealthLua:   newHealthStatusFunction,
				UseOpenLibs: false,
			},
		},
	}

	script, useOpenLibs, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, false, useOpenLibs)
	assert.Equal(t, newHealthStatusFunction, script)
}

func TestGetHealthScriptWithGroupWildcardOverride(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"*.io/Rollout": {
				HealthLua:   newHealthStatusFunction,
				UseOpenLibs: false,
			},
		},
	}

	script, useOpenLibs, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, false, useOpenLibs)
	assert.Equal(t, newHealthStatusFunction, script)
}

func TestGetHealthScriptWithGroupAndKindWildcardOverride(t *testing.T) {
	testObj := StrToUnstructured(ec2AWSCrossplaneObjJson)
	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"*.aws.crossplane.io/*": {
				HealthLua:   newHealthStatusFunction,
				UseOpenLibs: false,
			},
		},
	}

	script, useOpenLibs, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, false, useOpenLibs)
	assert.Equal(t, newHealthStatusFunction, script)
}

func TestGetHealthScriptPredefined(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	vm := VM{}
	script, useOpenLibs, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, true, useOpenLibs)
	assert.NotEmpty(t, script)
}

func TestGetHealthScriptNoPredefined(t *testing.T) {
	testObj := StrToUnstructured(objWithNoScriptJSON)
	vm := VM{}
	script, useOpenLibs, err := vm.GetHealthScript(testObj)
	assert.Nil(t, err)
	assert.Equal(t, true, useOpenLibs)
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

func TestGetResourceActionsWithAddBuildInActionsFlag(t *testing.T) {
	testObj := StrToUnstructured(rolloutJSON)
	test := appv1.ResourceActionDefinition{
		Name:      "retry",
		ActionLua: "obj.status.abort = nil\nreturn obj\n",
	}

	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/Rollout": {
				Actions: string(grpc.MustMarshal(appv1.ResourceActions{
					ActionDiscoveryLua: validDiscoveryLua,
					AddBuildInActions:  true,
				})),
			},
		},
	}
	action, err := vm.GetResourceAction(testObj, "retry")
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

func TestExecuteResourceActionDiscoveryWithAddBuildInActionsFlag(t *testing.T) {

	const validDiscoveryLua = `
			scaleParams = { {name = "replicas", type = "number"} }
			scale = {name = 'scale', params = scaleParams}
				
			test = {}
			actions = {scale = scale, resume = resume, test = test}
			
			return actions
			`
	vm := VM{
		ResourceOverrides: map[string]appv1.ResourceOverride{
			"argoproj.io/Rollout": {
				Actions: string(grpc.MustMarshal(appv1.ResourceActions{
					ActionDiscoveryLua: validDiscoveryLua,
					AddBuildInActions:  true,
				})),
			},
		},
	}
	testObj := StrToUnstructured(rolloutJSON)
	discoveryLua, err := vm.GetResourceActionDiscovery(testObj)
	assert.Nil(t, err)
	actions, err := vm.ExecuteResourceActionDiscovery(testObj, discoveryLua)
	assert.Nil(t, err)
	expectedActions := []appv1.ResourceAction{

		{
			Name: "resume",
		},
		{
			Name: "retry",
		},
		{
			Name: "abort",
		},
		{
			Name: "restart",
		},
		{
			Name: "promote-full",
		},
		{
			Name: "test",
		},
		{
			Name: "scale",
		},
	}
	for _, expectedAction := range expectedActions {
		found := false
		for _, action := range actions {
			if action.Name == expectedAction.Name {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected action %s not found", expectedAction.Name)
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

const expectedLuaUpdatedResult = `
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

// Test an action that returns a single k8s resource json
func TestExecuteOldStyleResourceAction(t *testing.T) {
	testObj := StrToUnstructured(objJSON)
	expectedLuaUpdatedObj := StrToUnstructured(expectedLuaUpdatedResult)
	vm := VM{}
	newObjects, err := vm.ExecuteResourceAction(testObj, validActionLua)
	assert.Nil(t, err)
	assert.Equal(t, len(newObjects), 1)
	assert.Equal(t, newObjects[0].K8SOperation, K8SOperation("patch"))
	assert.Equal(t, expectedLuaUpdatedObj, newObjects[0].UnstructuredObj)
}

const cronJobObjYaml = `
apiVersion: batch/v1
kind: CronJob
metadata:
  name: hello
  namespace: test-ns
`

const expectedCreatedJobObjList = `
- operation: create
  resource:
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: hello-1
      namespace: test-ns
`

const expectedCreatedMultipleJobsObjList = `
- operation: create
  resource:
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: hello-1
      namespace: test-ns
- operation: create
  resource:
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: hello-2
      namespace: test-ns	  
`

const expectedActionMixedOperationObjList = `
- operation: create
  resource:
    apiVersion: batch/v1
    kind: Job
    metadata:
      name: hello-1
      namespace: test-ns
- operation: patch
  resource:
    apiVersion: batch/v1
    kind: CronJob
    metadata:
      name: hello
      namespace: test-ns	  
      labels:
        test: test  
`

const createJobActionLua = `
job = {}
job.apiVersion = "batch/v1"
job.kind = "Job"

job.metadata = {}
job.metadata.name = "hello-1"
job.metadata.namespace = "test-ns"

impactedResource = {}
impactedResource.operation = "create"
impactedResource.resource = job
result = {}
result[1] = impactedResource

return result
`

const createMultipleJobsActionLua = `
job1 = {}
job1.apiVersion = "batch/v1"
job1.kind = "Job"

job1.metadata = {}
job1.metadata.name = "hello-1"
job1.metadata.namespace = "test-ns"

impactedResource1 = {}
impactedResource1.operation = "create"
impactedResource1.resource = job1
result = {}
result[1] = impactedResource1

job2 = {}
job2.apiVersion = "batch/v1"
job2.kind = "Job"

job2.metadata = {}
job2.metadata.name = "hello-2"
job2.metadata.namespace = "test-ns"

impactedResource2 = {}
impactedResource2.operation = "create"
impactedResource2.resource = job2

result[2] = impactedResource2

return result
`
const mixedOperationActionLuaOk = `
job1 = {}
job1.apiVersion = "batch/v1"
job1.kind = "Job"

job1.metadata = {}
job1.metadata.name = "hello-1"
job1.metadata.namespace = obj.metadata.namespace

impactedResource1 = {}
impactedResource1.operation = "create"
impactedResource1.resource = job1
result = {}
result[1] = impactedResource1

obj.metadata.labels = {}
obj.metadata.labels["test"] = "test"

impactedResource2 = {}
impactedResource2.operation = "patch"
impactedResource2.resource = obj

result[2] = impactedResource2

return result
`

const createMixedOperationActionLuaFailing = `
job1 = {}
job1.apiVersion = "batch/v1"
job1.kind = "Job"

job1.metadata = {}
job1.metadata.name = "hello-1"
job1.metadata.namespace = obj.metadata.namespace

impactedResource1 = {}
impactedResource1.operation = "create"
impactedResource1.resource = job1
result = {}
result[1] = impactedResource1

obj.metadata.labels = {}
obj.metadata.labels["test"] = "test"

impactedResource2 = {}
impactedResource2.operation = "thisShouldFail"
impactedResource2.resource = obj

result[2] = impactedResource2

return result
`

func TestExecuteNewStyleCreateActionSingleResource(t *testing.T) {
	testObj := StrToUnstructured(cronJobObjYaml)
	jsonBytes, err := yaml.YAMLToJSON([]byte(expectedCreatedJobObjList))
	assert.Nil(t, err)
	t.Log(bytes.NewBuffer(jsonBytes).String())
	expectedObjects, err := UnmarshalToImpactedResources(bytes.NewBuffer(jsonBytes).String())
	assert.Nil(t, err)
	vm := VM{}
	newObjects, err := vm.ExecuteResourceAction(testObj, createJobActionLua)
	assert.Nil(t, err)
	assert.Equal(t, expectedObjects, newObjects)
}

func TestExecuteNewStyleCreateActionMultipleResources(t *testing.T) {
	testObj := StrToUnstructured(cronJobObjYaml)
	jsonBytes, err := yaml.YAMLToJSON([]byte(expectedCreatedMultipleJobsObjList))
	assert.Nil(t, err)
	// t.Log(bytes.NewBuffer(jsonBytes).String())
	expectedObjects, err := UnmarshalToImpactedResources(bytes.NewBuffer(jsonBytes).String())
	assert.Nil(t, err)
	vm := VM{}
	newObjects, err := vm.ExecuteResourceAction(testObj, createMultipleJobsActionLua)
	assert.Nil(t, err)
	assert.Equal(t, expectedObjects, newObjects)
}

func TestExecuteNewStyleActionMixedOperationsOk(t *testing.T) {
	testObj := StrToUnstructured(cronJobObjYaml)
	jsonBytes, err := yaml.YAMLToJSON([]byte(expectedActionMixedOperationObjList))
	assert.Nil(t, err)
	// t.Log(bytes.NewBuffer(jsonBytes).String())
	expectedObjects, err := UnmarshalToImpactedResources(bytes.NewBuffer(jsonBytes).String())
	assert.Nil(t, err)
	vm := VM{}
	newObjects, err := vm.ExecuteResourceAction(testObj, mixedOperationActionLuaOk)
	assert.Nil(t, err)
	assert.Equal(t, expectedObjects, newObjects)
}

func TestExecuteNewStyleActionMixedOperationsFailure(t *testing.T) {
	testObj := StrToUnstructured(cronJobObjYaml)
	vm := VM{}
	_, err := vm.ExecuteResourceAction(testObj, createMixedOperationActionLuaFailing)
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "unsupported operation")
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
	newObjects, err := vm.ExecuteResourceAction(testObj, pausedToFalseLua)
	assert.Nil(t, err)
	assert.Equal(t, len(newObjects), 1)
	assert.Equal(t, newObjects[0].K8SOperation, K8SOperation("patch"))
	assert.Equal(t, expectedObj, newObjects[0].UnstructuredObj)
}

func TestGetResourceHealth(t *testing.T) {
	const testSA = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: test
  namespace: test`

	const script = `
hs = {}
str = "Using lua standard library"
if string.find(str, "standard") then
  hs.message = "Standard lib was used"
else
  hs.message = "Standard lib was not used"
end
hs.status = "Healthy"
return hs`

	const healthWildcardOverrideScript = `
 hs = {}
 hs.status = "Healthy"
 return hs`

	getHealthOverride := func(openLibs bool) ResourceHealthOverrides {
		return ResourceHealthOverrides{
			"ServiceAccount": appv1.ResourceOverride{
				HealthLua:   script,
				UseOpenLibs: openLibs,
			},
		}
	}

	getWildcardHealthOverride := ResourceHealthOverrides{
		"*.aws.crossplane.io/*": appv1.ResourceOverride{
			HealthLua: healthWildcardOverrideScript,
		},
	}

	t.Run("Enable Lua standard lib", func(t *testing.T) {
		testObj := StrToUnstructured(testSA)
		overrides := getHealthOverride(true)
		status, err := overrides.GetResourceHealth(testObj)
		assert.Nil(t, err)
		expectedStatus := &health.HealthStatus{
			Status:  health.HealthStatusHealthy,
			Message: "Standard lib was used",
		}
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("Disable Lua standard lib", func(t *testing.T) {
		testObj := StrToUnstructured(testSA)
		overrides := getHealthOverride(false)
		status, err := overrides.GetResourceHealth(testObj)
		assert.IsType(t, &lua.ApiError{}, err)
		expectedErr := "<string>:4: attempt to index a non-table object(nil) with key 'find'\nstack traceback:\n\t<string>:4: in main chunk\n\t[G]: ?"
		assert.EqualError(t, err, expectedErr)
		assert.Nil(t, status)
	})

	t.Run("Get resource health for wildcard override", func(t *testing.T) {
		testObj := StrToUnstructured(ec2AWSCrossplaneObjJson)
		overrides := getWildcardHealthOverride
		status, err := overrides.GetResourceHealth(testObj)
		assert.Nil(t, err)
		expectedStatus := &health.HealthStatus{
			Status: health.HealthStatusHealthy,
		}
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("Resource health for wildcard override not found", func(t *testing.T) {
		testObj := StrToUnstructured(testSA)
		overrides := getWildcardHealthOverride
		status, err := overrides.GetResourceHealth(testObj)
		assert.Nil(t, err)
		assert.Nil(t, status)
	})
}

func TestMergeLuaScripts(t *testing.T) {
	overrideScript := `
    local actions = {}
    actions["action1"] = {name = "action1"}
    return actions
    `
	builtInDiscoveryScript := `
    local actions = {}
    actions["action2"] = {
        name = "action2"
    }
    return actions
    `
	expected := `
    local actions = {}
    actions["action1"] = {name = "action1"}
    actions["action2"] = {name = "action2"}
    return actions
    `

	testCases := []struct {
		name         string
		override     string
		discovery    string
		expectErr    bool
		expectedCode string
	}{
		{
			name:         "valid merge",
			override:     overrideScript,
			discovery:    builtInDiscoveryScript,
			expectErr:    false,
			expectedCode: expected,
		},
		{
			name: "missing return actions in  override script",
			override: ` actions["action1"] = {
        				name = "action1"
					}`,
			discovery:    builtInDiscoveryScript,
			expectErr:    false,
			expectedCode: "",
		},
		{
			name:         "missing return keyword override script",
			override:     `actions`,
			discovery:    builtInDiscoveryScript,
			expectErr:    true,
			expectedCode: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeLuaScript(tc.override, tc.discovery)
			L := lua.NewState()
			defer L.Close()

			err := L.DoString(result)
			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if err := L.DoString(result); err != nil {
					t.Errorf("Result is not a valid Lua script: %v", err)
				}
				expectedScripts := []string{tc.expectedCode, result}
				for i, script := range expectedScripts {
					fn, err := L.LoadString(script)
					if err != nil {
						assert.NotNil(t, err, fmt.Sprintf("Error parsing script %d: %v", i, err))
					}
					L.Push(fn)
					if err := L.PCall(0, lua.MultRet, nil); err != nil {
						assert.NotNil(t, err, fmt.Sprintf("Error executing script %d: %v", i, err))
					}
				}

				// Compare the parsed ASTs
				expectedTable := L.Get(-2)
				resultTable := L.Get(-1)
				if !assert.Equal(t, expectedTable, resultTable) {
					t.Errorf("Parsed tables do not match - expected %v but got %v", expectedTable, resultTable)
				}
			}
		})
	}
}
