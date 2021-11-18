package controller

import (
	"context"
	"testing"

	"github.com/argoproj/notifications-engine/pkg/services"

	. "github.com/argoproj-labs/argocd-notifications/testing"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/controller"
	"github.com/argoproj/notifications-engine/pkg/mocks"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	logEntry = logrus.NewEntry(logrus.New())
)

func newController(t *testing.T, ctx context.Context, client dynamic.Interface) (*notificationController, *mocks.MockAPI, error) {
	mockCtrl := gomock.NewController(t)
	go func() {
		<-ctx.Done()
		mockCtrl.Finish()
	}()
	mockAPI := mocks.NewMockAPI(mockCtrl)
	mockAPI.EXPECT().GetConfig().Return(api.Config{}).AnyTimes()
	clientset := fake.NewSimpleClientset()
	c := NewController(clientset, client, nil, TestNamespace, "", controller.NewMetricsRegistry("argocd"))
	c.apiFactory = &mocks.FakeFactory{Api: mockAPI}
	err := c.Init(ctx)
	if err != nil {
		return nil, nil, err
	}
	return c, mockAPI, err
}

func TestSendsNotificationIfProjectTriggered(t *testing.T) {
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	appProj := NewProject("default", WithAnnotations(map[string]string{
		subscriptions.SubscribeAnnotationKey("my-trigger", "mock"): "recipient",
	}))
	app := NewApp("test", WithProject("default"))

	ctrl, _, err := newController(t, ctx, NewFakeClient(app, appProj))
	assert.NoError(t, err)

	dests := ctrl.alterDestinations(app, services.Destinations{}, api.Config{})

	assert.NoError(t, err)
	assert.NotEmpty(t, dests)
}

func TestAppSyncStatusRefreshed(t *testing.T) {
	for name, tc := range testsAppSyncStatusRefreshed {
		t.Run(name, func(t *testing.T) {
			if tc.result {
				assert.True(t, isAppSyncStatusRefreshed(&unstructured.Unstructured{Object: tc.app}, logEntry))
			} else {
				assert.False(t, isAppSyncStatusRefreshed(&unstructured.Unstructured{Object: tc.app}, logEntry))
			}
		})
	}
}

var testsAppSyncStatusRefreshed = map[string]struct {
	app    map[string]interface{}
	result bool
}{
	"MissingOperationState": {app: map[string]interface{}{"status": map[string]interface{}{}}, result: true},
	"MissingOperationStatePhase": {app: map[string]interface{}{
		"status": map[string]interface{}{
			"operationState": map[string]interface{}{},
		},
	}, result: true},
	"RunningOperation": {app: map[string]interface{}{
		"status": map[string]interface{}{
			"operationState": map[string]interface{}{
				"phase": "Running",
			},
		},
	}, result: true},
	"MissingFinishedAt": {app: map[string]interface{}{
		"status": map[string]interface{}{
			"operationState": map[string]interface{}{
				"phase": "Succeeded",
			},
		},
	}, result: false},
	"Reconciled": {app: map[string]interface{}{
		"status": map[string]interface{}{
			"reconciledAt": "2020-03-01T13:37:00Z",
			"observedAt":   "2020-03-01T13:37:00Z",
			"operationState": map[string]interface{}{
				"phase":      "Succeeded",
				"finishedAt": "2020-03-01T13:37:00Z",
			},
		},
	}, result: true},
	"NotYetReconciled": {app: map[string]interface{}{
		"status": map[string]interface{}{
			"reconciledAt": "2020-03-01T00:13:37Z",
			"observedAt":   "2020-03-01T00:13:37Z",
			"operationState": map[string]interface{}{
				"phase":      "Succeeded",
				"finishedAt": "2020-03-01T13:37:00Z",
			},
		},
	}, result: false},
}
