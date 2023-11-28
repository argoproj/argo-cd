package application

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/security"
)

func TestPodExists(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		podName        string
		namespace      string
		treeNodes      []appv1.ResourceNode
		expectedResult bool
	}{
		{
			name:           "empty tree nodes",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{},
			expectedResult: false,
		},
		{
			name:           "matched Pod but empty UID",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{ResourceRef: appv1.ResourceRef{Name: "test-pod", Namespace: "test", UID: "", Kind: kube.PodKind}}},
			expectedResult: false,
		},
		{
			name:           "matched Pod",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{ResourceRef: appv1.ResourceRef{Name: "test-pod", Namespace: "test", UID: "testUID", Kind: kube.PodKind}}},
			expectedResult: true,
		},
		{
			name:           "unmatched Pod Namespace",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{ResourceRef: appv1.ResourceRef{Name: "test-pod", Namespace: "test-A", UID: "testUID", Kind: kube.PodKind}}},
			expectedResult: false,
		},
		{
			name:           "unmatched Kind",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{ResourceRef: appv1.ResourceRef{Name: "test-pod", Namespace: "test-A", UID: "testUID", Kind: kube.DeploymentKind}}},
			expectedResult: false,
		},
		{
			name:           "unmatched Group",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{ResourceRef: appv1.ResourceRef{Name: "test-pod", Namespace: "test", UID: "testUID", Group: "A", Kind: kube.PodKind}}},
			expectedResult: false,
		},
		{
			name:           "unmatched Pod Name",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{ResourceRef: appv1.ResourceRef{Name: "test", Namespace: "test", UID: "testUID", Kind: kube.PodKind}}},
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := podExists(tcase.treeNodes, tcase.podName, tcase.namespace)
			if result != tcase.expectedResult {
				t.Errorf("Expected result %v, but got %v", tcase.expectedResult, result)
			}
		})
	}
}

func TestIsValidPodName(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		resourceName   string
		expectedResult bool
	}{
		{
			name:           "valid pod name",
			resourceName:   "argocd-server-794644486d-r8v9d",
			expectedResult: true,
		},
		{
			name:           "not valid contains spaces",
			resourceName:   "kubectl delete pods",
			expectedResult: false,
		},
		{
			name:           "not valid",
			resourceName:   "kubectl -n kube-system delete pods --all",
			expectedResult: false,
		},
		{
			name:           "not valid contains special characters",
			resourceName:   "delete+*+from+etcd%3b",
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := argo.IsValidPodName(tcase.resourceName)
			if result != tcase.expectedResult {
				t.Errorf("Expected result %v, but got %v", tcase.expectedResult, result)
			}
		})
	}
}

func TestIsValidNamespaceName(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		resourceName   string
		expectedResult bool
	}{
		{
			name:           "valid pod namespace name",
			resourceName:   "argocd",
			expectedResult: true,
		},
		{
			name:           "not valid contains spaces",
			resourceName:   "kubectl delete ns argocd",
			expectedResult: false,
		},
		{
			name:           "not valid contains special characters",
			resourceName:   "delete+*+from+etcd%3b",
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := argo.IsValidNamespaceName(tcase.resourceName)
			if result != tcase.expectedResult {
				t.Errorf("Expected result %v, but got %v", tcase.expectedResult, result)
			}
		})
	}
}

func TestIsValidContainerNameName(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		resourceName   string
		expectedResult bool
	}{
		{
			name:           "valid container name",
			resourceName:   "argocd-server",
			expectedResult: true,
		},
		{
			name:           "not valid contains spaces",
			resourceName:   "kubectl delete pods",
			expectedResult: false,
		},
		{
			name:           "not valid contains special characters",
			resourceName:   "delete+*+from+etcd%3b",
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := argo.IsValidContainerName(tcase.resourceName)
			if result != tcase.expectedResult {
				t.Errorf("Expected result %v, but got %v", tcase.expectedResult, result)
			}
		})
	}
}

func TestTerminalHandler_ServeHTTP_empty_params(t *testing.T) {
	testKeys := []string{
		"pod",
		"container",
		"app",
		"project",
		"namespace",
	}

	// test both empty and invalid
	testValues := []string{"", "invalid%20name"}

	for _, testKey := range testKeys {
		testKeyCopy := testKey

		for _, testValue := range testValues {
			testValueCopy := testValue

			t.Run(testKeyCopy+" "+testValueCopy, func(t *testing.T) {
				t.Parallel()

				handler := terminalHandler{}
				params := map[string]string{
					"pod":       "valid",
					"container": "valid",
					"app":       "valid",
					"project":   "valid",
					"namespace": "valid",
				}
				params[testKeyCopy] = testValueCopy
				var paramsArray []string
				for key, value := range params {
					paramsArray = append(paramsArray, key+"="+value)
				}
				paramsString := strings.Join(paramsArray, "&")
				request := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/api/v1/terminal?"+paramsString, nil)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				response := recorder.Result()
				assert.Equal(t, http.StatusBadRequest, response.StatusCode)
			})
		}
	}
}

func TestTerminalHandler_ServeHTTP_disallowed_namespace(t *testing.T) {
	handler := terminalHandler{namespace: "argocd", enabledNamespaces: []string{"allowed"}}
	request := httptest.NewRequest(http.MethodGet, "https://argocd.example.com/api/v1/terminal?pod=valid&container=valid&appName=valid&projectName=valid&namespace=test&appNamespace=disallowed", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
	assert.Equal(t, security.NamespaceNotPermittedError("disallowed").Error()+"\n", recorder.Body.String())
}
