package argo

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test"
)

func TestNewAuditLogger(t *testing.T) {
	logger := NewAuditLogger("default", fake.NewSimpleClientset(), "somecomponent")
	assert.NotNil(t, logger)
}

func TestLogAppProjEvent(t *testing.T) {

	logger := NewAuditLogger("default", fake.NewSimpleClientset(), "somecomponent")
	assert.NotNil(t, logger)

	proj := argoappv1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			Name:            "default",
			Namespace:       "argocd",
			ResourceVersion: "1",
			UID:             "a-b-c-d-e",
		},
		Spec: argoappv1.AppProjectSpec{
			Description: "Test project",
		},
	}

	ei := EventInfo{
		Reason: "test",
		Type:   "info",
	}

	output := test.CaptureLogEntries(func() {
		logger.LogAppProjEvent(&proj, ei, "This is a test message")
	})

	assert.Contains(t, output, "level=info")
	assert.Contains(t, output, "project=default")
	assert.Contains(t, output, "reason=test")
	assert.Contains(t, output, "type=info")
	assert.Contains(t, output, "msg=\"This is a test message\"")
}

func TestLogAppEvent(t *testing.T) {
	logger := NewAuditLogger("default", fake.NewSimpleClientset(), "somecomponent")
	assert.NotNil(t, logger)

	app := argoappv1.Application{
		ObjectMeta: v1.ObjectMeta{
			Name:            "testapp",
			Namespace:       "argocd",
			ResourceVersion: "1",
			UID:             "a-b-c-d-e",
		},
		Spec: argoappv1.ApplicationSpec{
			Destination: argoappv1.ApplicationDestination{
				Server:    "https://127.0.0.1:6443",
				Namespace: "testns",
			},
		},
	}

	ei := EventInfo{
		Reason: "test",
		Type:   "info",
	}

	output := test.CaptureLogEntries(func() {
		logger.LogAppEvent(&app, ei, "This is a test message")
	})

	assert.Contains(t, output, "level=info")
	assert.Contains(t, output, "application=testapp")
	assert.Contains(t, output, "dest-namespace=testns")
	assert.Contains(t, output, "dest-server=\"https://127.0.0.1:6443\"")
	assert.Contains(t, output, "reason=test")
	assert.Contains(t, output, "type=info")
	assert.Contains(t, output, "msg=\"This is a test message\"")

}

func TestLogResourceEvent(t *testing.T) {
	logger := NewAuditLogger("default", fake.NewSimpleClientset(), "somecomponent")
	assert.NotNil(t, logger)

	res := argoappv1.ResourceNode{
		ResourceRef: argoappv1.ResourceRef{
			Group:     "argocd.argoproj.io",
			Version:   "v1alpha1",
			Kind:      "SignatureKey",
			Name:      "testapp",
			Namespace: "argocd",
			UID:       "a-b-c-d-e",
		},
	}

	ei := EventInfo{
		Reason: "test",
		Type:   "info",
	}

	output := test.CaptureLogEntries(func() {
		logger.LogResourceEvent(&res, ei, "This is a test message")
	})

	assert.Contains(t, output, "level=info")
	assert.Contains(t, output, "name=testapp")
	assert.Contains(t, output, "reason=test")
	assert.Contains(t, output, "type=info")
	assert.Contains(t, output, "msg=\"This is a test message\"")
}
