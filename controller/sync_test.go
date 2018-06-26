package controller

import (
	"testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

func newTestSyncCtx() *syncContext {
	return &syncContext{
		comparison: &v1alpha1.ComparisonResult{},
		config:     &rest.Config{},
		namespace:  "test-namespace",
		syncOp:     &v1alpha1.SyncOperation{},
		opState:    &v1alpha1.OperationState{},
		log:        log.WithFields(log.Fields{"application": "fake-app"}),
	}
}

func TestRunWorkflows(t *testing.T) {
	// syncCtx := newTestSyncCtx()
	// syncCtx.doWorkflowSync(nil, nil)

}
