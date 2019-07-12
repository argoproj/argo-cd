package badge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
			Health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusHealthy},
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

	response := rr.Body.String()
	assert.Equal(t, success, leftPathColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, success, rightPathColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Healthy", leftText1Pattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Synced", rightText1Pattern.FindStringSubmatch(response)[1])
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

	response := rr.Body.String()
	assert.Equal(t, unknown, leftPathColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, unknown, rightPathColorPattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Unknown", leftText1Pattern.FindStringSubmatch(response)[1])
	assert.Equal(t, "Unknown", rightText1Pattern.FindStringSubmatch(response)[1])
}
