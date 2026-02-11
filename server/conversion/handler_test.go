package conversion

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestHandler_MethodNotAllowed(t *testing.T) {
	handler := NewHandler()

	req := httptest.NewRequest(http.MethodGet, "/convert", http.NoBody)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestHandler_InvalidJSON(t *testing.T) {
	handler := NewHandler()

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewBufferString("not json"))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_NilRequest(t *testing.T) {
	handler := NewHandler()

	review := apiextensionsv1.ConversionReview{
		Request: nil,
	}
	body, _ := json.Marshal(review)

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestHandler_ConvertV1alpha1ToV1beta1(t *testing.T) {
	handler := NewHandler()

	v1alpha1App := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "test-app",
			"namespace": "argocd",
		},
		"spec": map[string]any{
			"project": "default",
			"destination": map[string]any{
				"server":    "https://kubernetes.default.svc",
				"namespace": "default",
			},
			"source": map[string]any{
				"repoURL":        "https://github.com/example/repo",
				"path":           "manifests",
				"targetRevision": "main",
			},
		},
	}
	appBytes, _ := json.Marshal(v1alpha1App)

	review := apiextensionsv1.ConversionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "ConversionReview",
		},
		Request: &apiextensionsv1.ConversionRequest{
			UID:               "test-uid",
			DesiredAPIVersion: "argoproj.io/v1beta1",
			Objects: []runtime.RawExtension{
				{Raw: appBytes},
			},
		},
	}
	body, _ := json.Marshal(review)

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response apiextensionsv1.ConversionReview
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "test-uid", string(response.Response.UID))
	assert.Equal(t, metav1.StatusSuccess, response.Response.Result.Status)
	require.Len(t, response.Response.ConvertedObjects, 1)

	// Verify the converted object
	var convertedApp map[string]any
	err = json.Unmarshal(response.Response.ConvertedObjects[0].Raw, &convertedApp)
	require.NoError(t, err)

	assert.Equal(t, "argoproj.io/v1beta1", convertedApp["apiVersion"])
	assert.Equal(t, "Application", convertedApp["kind"])

	spec := convertedApp["spec"].(map[string]any)
	sources := spec["sources"].([]any)
	require.Len(t, sources, 1)
	source := sources[0].(map[string]any)
	assert.Equal(t, "https://github.com/example/repo", source["repoURL"])
}

func TestHandler_ConvertV1beta1ToV1alpha1(t *testing.T) {
	handler := NewHandler()

	v1beta1App := map[string]any{
		"apiVersion": "argoproj.io/v1beta1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "test-app",
			"namespace": "argocd",
		},
		"spec": map[string]any{
			"project": "default",
			"destination": map[string]any{
				"server":    "https://kubernetes.default.svc",
				"namespace": "default",
			},
			"sources": []map[string]any{
				{
					"repoURL":        "https://github.com/example/repo",
					"path":           "manifests",
					"targetRevision": "main",
				},
			},
		},
	}
	appBytes, _ := json.Marshal(v1beta1App)

	review := apiextensionsv1.ConversionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apiextensions.k8s.io/v1",
			Kind:       "ConversionReview",
		},
		Request: &apiextensionsv1.ConversionRequest{
			UID:               "test-uid",
			DesiredAPIVersion: "argoproj.io/v1alpha1",
			Objects: []runtime.RawExtension{
				{Raw: appBytes},
			},
		},
	}
	body, _ := json.Marshal(review)

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response apiextensionsv1.ConversionReview
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, metav1.StatusSuccess, response.Response.Result.Status)
	require.Len(t, response.Response.ConvertedObjects, 1)

	// Verify the converted object
	var convertedApp map[string]any
	err = json.Unmarshal(response.Response.ConvertedObjects[0].Raw, &convertedApp)
	require.NoError(t, err)

	assert.Equal(t, "argoproj.io/v1alpha1", convertedApp["apiVersion"])

	spec := convertedApp["spec"].(map[string]any)
	// Single source should also populate the source field
	source := spec["source"].(map[string]any)
	assert.Equal(t, "https://github.com/example/repo", source["repoURL"])
}

func TestHandler_SameVersion(t *testing.T) {
	handler := NewHandler()

	v1alpha1App := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata": map[string]any{
			"name":      "test-app",
			"namespace": "argocd",
		},
		"spec": map[string]any{
			"project": "default",
			"destination": map[string]any{
				"server":    "https://kubernetes.default.svc",
				"namespace": "default",
			},
		},
	}
	appBytes, _ := json.Marshal(v1alpha1App)

	review := apiextensionsv1.ConversionReview{
		Request: &apiextensionsv1.ConversionRequest{
			UID:               "test-uid",
			DesiredAPIVersion: "argoproj.io/v1alpha1", // Same version
			Objects: []runtime.RawExtension{
				{Raw: appBytes},
			},
		},
	}
	body, _ := json.Marshal(review)

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response apiextensionsv1.ConversionReview
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, metav1.StatusSuccess, response.Response.Result.Status)
	// Should return the same object unchanged
	assert.Equal(t, appBytes, response.Response.ConvertedObjects[0].Raw)
}

func TestHandler_UnsupportedKind(t *testing.T) {
	handler := NewHandler()

	unsupportedObj := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "AppProject", // Not supported
		"metadata": map[string]any{
			"name": "test-project",
		},
	}
	objBytes, _ := json.Marshal(unsupportedObj)

	review := apiextensionsv1.ConversionReview{
		Request: &apiextensionsv1.ConversionRequest{
			UID:               "test-uid",
			DesiredAPIVersion: "argoproj.io/v1beta1",
			Objects: []runtime.RawExtension{
				{Raw: objBytes},
			},
		},
	}
	body, _ := json.Marshal(review)

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response apiextensionsv1.ConversionReview
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, metav1.StatusFailure, response.Response.Result.Status)
	assert.Contains(t, response.Response.Result.Message, "unsupported kind")
}

func TestHandler_MultipleObjects(t *testing.T) {
	handler := NewHandler()

	app1 := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": "app1", "namespace": "argocd"},
		"spec": map[string]any{
			"project":     "default",
			"destination": map[string]any{"server": "https://kubernetes.default.svc"},
			"source":      map[string]any{"repoURL": "https://github.com/example/repo1", "targetRevision": "main"},
		},
	}
	app2 := map[string]any{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Application",
		"metadata":   map[string]any{"name": "app2", "namespace": "argocd"},
		"spec": map[string]any{
			"project":     "default",
			"destination": map[string]any{"server": "https://kubernetes.default.svc"},
			"source":      map[string]any{"repoURL": "https://github.com/example/repo2", "targetRevision": "main"},
		},
	}
	app1Bytes, _ := json.Marshal(app1)
	app2Bytes, _ := json.Marshal(app2)

	review := apiextensionsv1.ConversionReview{
		Request: &apiextensionsv1.ConversionRequest{
			UID:               "test-uid",
			DesiredAPIVersion: "argoproj.io/v1beta1",
			Objects: []runtime.RawExtension{
				{Raw: app1Bytes},
				{Raw: app2Bytes},
			},
		},
	}
	body, _ := json.Marshal(review)

	req := httptest.NewRequest(http.MethodPost, "/convert", bytes.NewBuffer(body))
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var response apiextensionsv1.ConversionReview
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, metav1.StatusSuccess, response.Response.Result.Status)
	require.Len(t, response.Response.ConvertedObjects, 2)

	// Verify both objects were converted
	for i, obj := range response.Response.ConvertedObjects {
		var convertedApp map[string]any
		err = json.Unmarshal(obj.Raw, &convertedApp)
		require.NoError(t, err)
		assert.Equal(t, "argoproj.io/v1beta1", convertedApp["apiVersion"], "object %d should be v1beta1", i)
	}
}
