package badge

import (
	"context"
	"fmt"
	"image/color"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/util/settings"

	"github.com/argoproj/gitops-engine/pkg/health"
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
	testProject = v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{Name: "testProject", Namespace: "default"},
		Spec:       v1alpha1.AppProjectSpec{},
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

func TestHandlerFeatureProjectIsEnabled(t *testing.T) {
	projectTests := []struct {
		testApp     []*v1alpha1.Application
		apiEndPoint string
		namespace   string
		health      string
		status      string
		healthColor color.RGBA
		statusColor color.RGBA
	}{
		{createApplications([]string{"Healthy:Synced", "Healthy:Synced"}, []string{"default", "default"}, "test"),
			"/api/badge?project=default", "test", "Healthy", "Synced", Green, Green},
		{createApplications([]string{"Healthy:Synced", "Healthy:OutOfSync"}, []string{"testProject", "testProject"}, "default"),
			"/api/badge?project=testProject", "default", "Healthy", "OutOfSync", Green, Orange},
		{createApplications([]string{"Healthy:Synced", "Degraded:Synced"}, []string{"default", "default"}, "test"),
			"/api/badge?project=default", "test", "Degraded", "Synced", Red, Green},
		{createApplications([]string{"Healthy:Synced", "Degraded:OutOfSync"}, []string{"testProject", "testProject"}, "default"),
			"/api/badge?project=testProject", "default", "Degraded", "OutOfSync", Red, Orange},
		{createApplications([]string{"Healthy:Synced", "Healthy:Synced"}, []string{"testProject", "default"}, "test"),
			"/api/badge?project=default&project=testProject", "test", "Healthy", "Synced", Green, Green},
		{createApplications([]string{"Healthy:OutOfSync", "Healthy:Synced"}, []string{"testProject", "default"}, "default"),
			"/api/badge?project=default&project=testProject", "default", "Healthy", "OutOfSync", Green, Orange},
		{createApplications([]string{"Degraded:Synced", "Healthy:Synced"}, []string{"testProject", "default"}, "test"),
			"/api/badge?project=default&project=testProject", "test", "Degraded", "Synced", Red, Green},
		{createApplications([]string{"Degraded:OutOfSync", "Healthy:OutOfSync"}, []string{"testProject", "default"}, "default"),
			"/api/badge?project=default&project=testProject", "default", "Degraded", "OutOfSync", Red, Orange},
		{createApplications([]string{"Unknown:Unknown", "Unknown:Unknown"}, []string{"testProject", "default"}, "default"),
			"/api/badge?project=", "default", "Unknown", "Unknown", Purple, Purple},
	}
	for _, tt := range projectTests {
		argoCDCm.ObjectMeta.Namespace = tt.namespace
		argoCDSecret.ObjectMeta.Namespace = tt.namespace
		settingsMgr := settings.NewSettingsManager(context.Background(), fake.NewSimpleClientset(&argoCDCm, &argoCDSecret), tt.namespace)
		handler := NewHandler(appclientset.NewSimpleClientset(&testProject, tt.testApp[0], tt.testApp[1]), settingsMgr, tt.namespace)
		rr := httptest.NewRecorder()
		req, err := http.NewRequest("GET", tt.apiEndPoint, nil)
		assert.NoError(t, err)
		handler.ServeHTTP(rr, req)
		assert.Equal(t, "private, no-store", rr.Header().Get("Cache-Control"))
		response := rr.Body.String()
		assert.Equal(t, toRGBString(tt.healthColor), leftRectColorPattern.FindStringSubmatch(response)[1])
		assert.Equal(t, toRGBString(tt.statusColor), rightRectColorPattern.FindStringSubmatch(response)[1])
		assert.Equal(t, tt.health, leftTextPattern.FindStringSubmatch(response)[1])
		assert.Equal(t, tt.status, rightTextPattern.FindStringSubmatch(response)[1])

	}
}

func createApplicationFeatureProjectIsEnabled(healthStatus health.HealthStatusCode, syncStatus v1alpha1.SyncStatusCode, appName, projectName, namespace string) *v1alpha1.Application {
	return &v1alpha1.Application{
		ObjectMeta: v1.ObjectMeta{Name: appName, Namespace: namespace},
		Status: v1alpha1.ApplicationStatus{
			Sync:   v1alpha1.SyncStatus{Status: syncStatus},
			Health: v1alpha1.HealthStatus{Status: healthStatus},
			OperationState: &v1alpha1.OperationState{
				SyncResult: &v1alpha1.SyncOperationResult{},
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: projectName,
		},
	}
}

func createApplications(appCombo, projectName []string, namespace string) []*v1alpha1.Application {
	apps := make([]*v1alpha1.Application, len(appCombo))
	healthStatus := func(healthType string) health.HealthStatusCode {
		switch healthType {
		case "Healthy":
			return health.HealthStatusHealthy
		case "Degraded":
			return health.HealthStatusDegraded
		default:
			return health.HealthStatusUnknown
		}
	}
	syncStatus := func(syncType string) v1alpha1.SyncStatusCode {
		switch syncType {
		case "Synced":
			return v1alpha1.SyncStatusCodeSynced
		case "OutOfSync":
			return v1alpha1.SyncStatusCodeOutOfSync
		default:
			return v1alpha1.SyncStatusCodeUnknown
		}
	}
	for k, v := range appCombo {
		a := strings.Split(v, ":")
		healthApp := healthStatus(a[0])
		syncApp := syncStatus(a[1])
		appName := fmt.Sprintf("App %v", k)
		apps[k] = createApplicationFeatureProjectIsEnabled(healthApp, syncApp, appName, projectName[k], namespace)
	}
	return apps
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

func TestHandlerRevisionIsEnabledShortCommitSHA(t *testing.T) {
	app := testApp.DeepCopy()
	app.Status.OperationState.SyncResult.Revision = "abc"

	settingsMgr := settings.NewSettingsManager(context.Background(), fake.NewSimpleClientset(&argoCDCm, &argoCDSecret), "default")
	handler := NewHandler(appclientset.NewSimpleClientset(app), settingsMgr, "default")
	req, err := http.NewRequest("GET", "/api/badge?name=testApp&revision=true", nil)
	assert.NoError(t, err)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	response := rr.Body.String()
	assert.Contains(t, response, "(abc)")
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
