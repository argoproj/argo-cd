package metrics

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type fakeWrapper struct {
	t             *testing.T
	currentCount  int
	expectedCount int
}

func (f fakeWrapper) RoundTrip(r *http.Request) (*http.Response, error) {
	resp := httptest.NewRecorder()
	resp.Code = 201
	assert.Equal(f.t, f.currentCount, f.expectedCount)
	return resp.Result(), nil
}

// TestWrappingTwice Ensures that the config doesn't lose any previous wrappers and the previous wrapper
// gets executed first
func TestAddMetricsTransportWrapperWrapTwice(t *testing.T) {
	config := &rest.Config{
		Host: "",
	}
	currentCount := 0
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return fakeWrapper{
			t:             t,
			expectedCount: 0,
			currentCount:  currentCount,
		}
	}

	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		currentCount++
		return nil
	})

	client := kubernetes.NewForConfigOrDie(newConfig)
	client.AppsV1().ReplicaSets(metav1.NamespaceDefault).Get("test", metav1.GetOptions{})
	// Ensures second wrapper added by AddMetricsTransportWrapper is executed
	assert.Equal(t, 1, currentCount)

}

func newGetRequest(str string) *http.Request {
	requestURL, err := url.Parse(str)
	if err != nil {
		panic(err)
	}
	return &http.Request{
		Method: "GET",
		URL:    requestURL,
	}
}

func TestParseRequest(t *testing.T) {
	testData := []struct {
		testName string
		url      string
		expected ResourceInfo
	}{
		{
			testName: "Pod LIST",
			url:      "https://127.0.0.1/api/v1/namespaces/default/pods",
			expected: ResourceInfo{
				Verb:      List,
				Namespace: "default",
				Kind:      "pods",
			},
		},
		{
			testName: "Pod Cluster LIST",
			url:      "https://127.0.0.1/api/v1/pods",
			expected: ResourceInfo{
				Verb: List,
				Kind: "pods",
			},
		},
		{
			testName: "Pod GET",
			url:      "https://127.0.0.1/api/v1/namespaces/default/pods/pod-name-123456",
			expected: ResourceInfo{
				Verb:      Get,
				Namespace: "default",
				Kind:      "pods",
				Name:      "pod-name-123456",
			},
		},
		{
			testName: "Namespace LIST",
			url:      "https://127.0.0.1/api/v1/namespaces",
			expected: ResourceInfo{
				Verb: List,
				Kind: "namespaces",
			},
		},
		{
			testName: "Namespace GET",
			url:      "https://127.0.0.1/api/v1/namespaces/default",
			expected: ResourceInfo{
				Verb: Get,
				Kind: "namespaces",
				Name: "default",
			},
		},
		{
			testName: "ReplicaSet LIST",
			url:      "https://127.0.0.1/apis/extensions/v1beta1/namespaces/default/replicasets",
			expected: ResourceInfo{
				Verb:      List,
				Kind:      "replicasets",
				Namespace: "default",
			},
		},
		{
			testName: "ReplicaSet Cluster LIST",
			url:      "https://127.0.0.1/apis/apps/v1/replicasets",
			expected: ResourceInfo{
				Verb: List,
				Kind: "replicasets",
			},
		},
		{
			testName: "ReplicaSet GET",
			url:      "https://127.0.0.1/apis/extensions/v1beta1/namespaces/default/replicasets/rs-abc123",
			expected: ResourceInfo{
				Verb:      Get,
				Kind:      "replicasets",
				Namespace: "default",
				Name:      "rs-abc123",
			},
		},
		{
			testName: "VirtualService LIST",
			url:      "https://127.0.0.1/apis/networking.istio.io/v1alpha3/namespaces/default/virtualservices",
			expected: ResourceInfo{
				Verb:      List,
				Kind:      "virtualservices",
				Namespace: "default",
			},
		},
		{
			testName: "VirtualService GET",
			url:      "https://127.0.0.1/apis/networking.istio.io/v1alpha3/namespaces/default/virtualservices/virutal-service",
			expected: ResourceInfo{
				Verb:      Get,
				Kind:      "virtualservices",
				Namespace: "default",
				Name:      "virutal-service",
			},
		},
		{
			testName: "ClusterRole LIST",
			url:      "https://127.0.0.1/apis/rbac.authorization.k8s.io/v1/clusterroles",
			expected: ResourceInfo{
				Verb: List,
				Kind: "clusterroles",
			},
		},
		{
			testName: "ClusterRole Get",
			url:      "https://127.0.0.1/apis/rbac.authorization.k8s.io/v1/clusterroles/argo-rollouts-clusterrole",
			expected: ResourceInfo{
				Verb: Get,
				Kind: "clusterroles",
				Name: "argo-rollouts-clusterrole",
			},
		},
		{
			testName: "CRD List",
			url:      "https://127.0.0.1/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions",
			expected: ResourceInfo{
				Verb: List,
				Kind: "customresourcedefinitions",
			},
		},
		{
			testName: "CRD Get",
			url:      "https://127.0.0.1/apis/apiextensions.k8s.io/v1beta1/customresourcedefinitions/dummies.argoproj.io",
			expected: ResourceInfo{
				Verb: Get,
				Kind: "customresourcedefinitions",
				Name: "dummies.argoproj.io",
			},
		},
		{
			testName: "Resource With Periods Get",
			url:      "https://127.0.0.1/apis/argoproj.io/v1alpha1/namespaces/argocd/applications/my-cluster.cluster.k8s.local",
			expected: ResourceInfo{
				Verb:      Get,
				Kind:      "applications",
				Namespace: "argocd",
				Name:      "my-cluster.cluster.k8s.local",
			},
		},
	}

	for _, td := range testData {
		t.Run(td.testName, func(t *testing.T) {
			r := newGetRequest(td.url)
			info := parseRequest(r)
			assert.Equal(t, td.expected, info)
		})
	}
}

func TestGetRequest(t *testing.T) {
	expectedStatusCode := 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	}))
	defer ts.Close()
	executed := false
	config := &rest.Config{
		Host: ts.URL,
	}
	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		assert.Equal(t, expectedStatusCode, info.StatusCode)
		assert.Equal(t, "replicasets", info.Kind)
		assert.Equal(t, metav1.NamespaceDefault, info.Namespace)
		assert.Equal(t, "test", info.Name)
		assert.Equal(t, Get, info.Verb)
		executed = true
		return nil
	})
	client := kubernetes.NewForConfigOrDie(newConfig)
	client.AppsV1().ReplicaSets(metav1.NamespaceDefault).Get("test", metav1.GetOptions{})
	assert.True(t, executed)
}

func TestListRequest(t *testing.T) {
	expectedStatusCode := 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	}))
	defer ts.Close()
	executed := false
	config := &rest.Config{
		Host: ts.URL,
	}
	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		assert.Equal(t, expectedStatusCode, info.StatusCode)
		assert.Equal(t, "replicasets", info.Kind)
		assert.Equal(t, metav1.NamespaceDefault, info.Namespace)
		assert.Equal(t, "", info.Name)
		assert.Equal(t, List, info.Verb)
		executed = true
		return nil
	})
	client := kubernetes.NewForConfigOrDie(newConfig)
	client.AppsV1().ReplicaSets(metav1.NamespaceDefault).List(metav1.ListOptions{})
	assert.True(t, executed)
}

func TestCreateRequest(t *testing.T) {
	expectedStatusCode := 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	}))
	defer ts.Close()
	executed := false
	config := &rest.Config{
		Host: ts.URL,
	}
	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		assert.Equal(t, expectedStatusCode, info.StatusCode)
		assert.Equal(t, "replicasets", info.Kind)
		assert.Equal(t, metav1.NamespaceDefault, info.Namespace)
		assert.Equal(t, "test", info.Name)
		assert.Equal(t, Create, info.Verb)
		executed = true
		return nil
	})
	client := kubernetes.NewForConfigOrDie(newConfig)
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
		},
	}
	client.AppsV1().ReplicaSets(metav1.NamespaceDefault).Create(rs)
	assert.True(t, executed)
}

func TestDeleteRequest(t *testing.T) {
	expectedStatusCode := 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	}))
	defer ts.Close()
	executed := false
	config := &rest.Config{
		Host: ts.URL,
	}
	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		assert.Equal(t, expectedStatusCode, info.StatusCode)
		assert.Equal(t, "replicasets", info.Kind)
		assert.Equal(t, metav1.NamespaceDefault, info.Namespace)
		assert.Equal(t, "test", info.Name)
		assert.Equal(t, Delete, info.Verb)
		executed = true
		return nil
	})
	client := kubernetes.NewForConfigOrDie(newConfig)
	client.AppsV1().ReplicaSets(metav1.NamespaceDefault).Delete("test", &metav1.DeleteOptions{})
	assert.True(t, executed)
}

func TestPatchRequest(t *testing.T) {
	expectedStatusCode := 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	}))
	defer ts.Close()
	executed := false
	config := &rest.Config{
		Host: ts.URL,
	}
	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		assert.Equal(t, expectedStatusCode, info.StatusCode)
		assert.Equal(t, "replicasets", info.Kind)
		assert.Equal(t, metav1.NamespaceDefault, info.Namespace)
		assert.Equal(t, "test", info.Name)
		assert.Equal(t, Patch, info.Verb)
		executed = true
		return nil
	})
	client := kubernetes.NewForConfigOrDie(newConfig)
	client.AppsV1().ReplicaSets(metav1.NamespaceDefault).Patch("test", types.MergePatchType, []byte("{}"))
	assert.True(t, executed)
}

func TestUpdateRequest(t *testing.T) {
	expectedStatusCode := 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	}))
	defer ts.Close()
	executed := false
	config := &rest.Config{
		Host: ts.URL,
	}
	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		assert.Equal(t, expectedStatusCode, info.StatusCode)
		assert.Equal(t, "replicasets", info.Kind)
		assert.Equal(t, metav1.NamespaceDefault, info.Namespace)
		assert.Equal(t, "test", info.Name)
		assert.Equal(t, Update, info.Verb)
		executed = true
		return nil
	})
	client := kubernetes.NewForConfigOrDie(newConfig)
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	client.AppsV1().ReplicaSets(metav1.NamespaceDefault).Update(rs)
	assert.True(t, executed)
}

func TestUnknownRequest(t *testing.T) {
	expectedStatusCode := 201
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(expectedStatusCode)
	}))
	defer ts.Close()
	executed := false
	config := &rest.Config{
		Host: ts.URL,
	}
	newConfig := AddMetricsTransportWrapper(config, func(info ResourceInfo) error {
		assert.Equal(t, expectedStatusCode, info.StatusCode)
		assert.Equal(t, Unknown, info.Verb)
		executed = true
		return nil
	})
	client := kubernetes.NewForConfigOrDie(newConfig)
	client.Discovery().RESTClient().Verb("invalid-verb").Do()
	assert.True(t, executed)
}
