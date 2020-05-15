package badge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/argoproj/argo-cd/engine/pkg/utils/health"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/settings"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var (
	argoCDSecret = corev1.Secret{
		ObjectMeta: v1.ObjectMeta{Name: "argocd-secret", Namespace: "default"},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	argoCDCm = corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{
			"statusbadge.enabled": "true",
		},
	}
	testApp = v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{Name: "testApp", Namespace: "default"},
		Status: v1alpha1.ApplicationStatus{
			Sync:   v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeSynced},
			Health: v1alpha1.HealthStatus{Status: health.HealthStatusHealthy},
			OperationState: &v1alpha1.OperationState{
				SyncResult: &v1alpha1.SyncOperationResult{
					Revision: "aa29b85",
				},
			},
		},
	}
)

func TestHandlerFeatureIsEnabled(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), fake.NewSimpleClientset(&argoCDCm, &argoCDSecret), "default")
	handler := NewHandler(appclientset.NewSimpleClientset(&testApp), settingsMgr, "default")
	req, err := http.NewRequest("GET", "/api/badge?name=testApp", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))

	response := rr.Body.String()
	assert.Equal(t, toRGBString(Green), leftRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, toRGBString(Green), rightRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Healthy", leftTextPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Synced", rightTextPattern.FindStringSubmatch(response)[1])
	assert.NotContains(t, response, "(aa29b85)")
}

func TestHandlerFeatureIsEnabledRevisionIsEnabled(t *testing.T) {
	settingsMgr := settings.NewSettingsManager(context.Background(), fake.NewSimpleClientset(&argoCDCm, &argoCDSecret), "default")
	handler := NewHandler(appclientset.NewSimpleClientset(&testApp), settingsMgr, "default")
	req, err := http.NewRequest("GET", "/api/badge?name=testApp&revision=true", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))

	response := rr.Body.String()
	assert.Equal(t, toRGBString(Green), leftRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, toRGBString(Green), rightRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Healthy", leftTextPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Synced", rightTextPattern.FindStringSubmatch(response)[1])
	assert.Contains(t, response, "(aa29b85)")
}

func TestHandlerRevisionIsEnabledNoOperationState(t *testing.T) {
	app := testApp.DeepCopy()
	app.Status.OperationState = nil

	settingsMgr := settings.NewSettingsManager(context.Background(), fake.NewSimpleClientset(&argoCDCm, &argoCDSecret), "default")
	handler := NewHandler(appclientset.NewSimpleClientset(app), settingsMgr, "default")
	req, err := http.NewRequest("GET", "/api/badge?name=testApp&revision=true", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))

	response := rr.Body.String()
	assert.Equal(t, toRGBString(Green), leftRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, toRGBString(Green), rightRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Healthy", leftTextPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Synced", rightTextPattern.FindStringSubmatch(response)[1])
	assert.NotContains(t, response, "(aa29b85)")
}

func TestHandlerFeatureIsDisabled(t *testing.T) {

	argoCDCmDisabled := argoCDCm.DeepCopy()
	delete(argoCDCmDisabled.Data, "statusbadge.enabled")

	settingsMgr := settings.NewSettingsManager(context.Background(), fake.NewSimpleClientset(argoCDCmDisabled, &argoCDSecret), "default")
	handler := NewHandler(appclientset.NewSimpleClientset(&testApp), settingsMgr, "default")
	req, err := http.NewRequest("GET", "/api/badge?name=testApp", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))

	response := rr.Body.String()
	assert.Equal(t, toRGBString(Purple), leftRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, toRGBString(Purple), rightRectColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Unknown", leftTextPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Unknown", rightTextPattern.FindStringSubmatch(response)[1])
}
