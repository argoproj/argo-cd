package controller

import (
	"fmt"
	"testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

type kubectlOutput struct {
	output string
	err    error
}

type mockKubectlCmd struct {
	commands map[string]kubectlOutput
}

func (k mockKubectlCmd) DeleteResource(config *rest.Config, obj *unstructured.Unstructured, namespace string) error {
	command, ok := k.commands[obj.GetName()]
	if !ok {
		return nil
	}
	return command.err
}

func (k mockKubectlCmd) ApplyResource(config *rest.Config, obj *unstructured.Unstructured, namespace string, dryRun, force bool) (string, error) {
	command, ok := k.commands[obj.GetName()]
	if !ok {
		return "", nil
	}
	return command.output, command.err
}

// ConvertToVersion converts an unstructured object into the specified group/version
func (k mockKubectlCmd) ConvertToVersion(obj *unstructured.Unstructured, group, version string) (*unstructured.Unstructured, error) {
	return obj, nil
}

func newTestSyncCtx() *syncContext {
	return &syncContext{
		comparison: &v1alpha1.ComparisonResult{},
		config:     &rest.Config{},
		namespace:  "test-namespace",
		syncRes:    &v1alpha1.SyncOperationResult{},
		syncOp: &v1alpha1.SyncOperation{
			Prune: true,
			SyncStrategy: &v1alpha1.SyncStrategy{
				Apply: &v1alpha1.SyncStrategyApply{},
			},
		},
		opState: &v1alpha1.OperationState{},
		log:     log.WithFields(log.Fields{"application": "fake-app"}),
	}
}

func TestSyncSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = mockKubectlCmd{}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "",
			TargetState: "{\"kind\":\"service\"}",
		}, {
			LiveState:   "{\"kind\":\"pod\"}",
			TargetState: "",
		},
		},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	assert.Equal(t, "service", syncCtx.syncRes.Resources[0].Kind)
	assert.Equal(t, v1alpha1.ResourceDetailsSynced, syncCtx.syncRes.Resources[0].Status)
	assert.Equal(t, "pod", syncCtx.syncRes.Resources[1].Kind)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncedAndPruned, syncCtx.syncRes.Resources[1].Status)

	syncCtx.sync()
	assert.Equal(t, syncCtx.opState.Phase, v1alpha1.OperationSucceeded)
}

func TestSyncDeleteSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = mockKubectlCmd{}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "{\"kind\":\"service\"}",
			TargetState: "",
		}, {
			LiveState:   "{\"kind\":\"pod\"}",
			TargetState: "",
		},
		},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncedAndPruned, syncCtx.syncRes.Resources[0].Status)
	assert.Equal(t, "pod", syncCtx.syncRes.Resources[0].Kind)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncedAndPruned, syncCtx.syncRes.Resources[1].Status)
	assert.Equal(t, "service", syncCtx.syncRes.Resources[1].Kind)

	syncCtx.sync()
	assert.Equal(t, syncCtx.opState.Phase, v1alpha1.OperationSucceeded)
}

func TestSyncCreateFailure(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = mockKubectlCmd{
		commands: map[string]kubectlOutput{
			"test-service": kubectlOutput{
				output: "",
				err:    fmt.Errorf("error: error validating \"test.yaml\": error validating data: apiVersion not set; if you choose to ignore these errors, turn validation off with --validate=false"),
			},
		},
	}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "",
			TargetState: "{\"kind\":\"service\", \"metadata\":{\"name\":\"test-service\"}}",
		},
		},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
}

func TestSyncPruneFailure(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = mockKubectlCmd{
		commands: map[string]kubectlOutput{
			"test-service": kubectlOutput{
				output: "",
				err:    fmt.Errorf(" error: timed out waiting for \"test-service\" to be synced"),
			},
		},
	}
	syncCtx.comparison = &v1alpha1.ComparisonResult{
		Resources: []v1alpha1.ResourceState{{
			LiveState:   "{\"kind\":\"service\", \"metadata\":{\"name\":\"test-service\"}}",
			TargetState: "",
		},
		},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, v1alpha1.ResourceDetailsSyncFailed, syncCtx.syncRes.Resources[0].Status)
}

func TestRunWorkflows(t *testing.T) {
	// syncCtx := newTestSyncCtx()
	// syncCtx.doWorkflowSync(nil, nil)

}
