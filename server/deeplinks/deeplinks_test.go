package deeplinks

import (
	"reflect"
	"strings"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

type deepLinkTC struct {
	name        string
	appObj      *unstructured.Unstructured
	clusterObj  *unstructured.Unstructured
	resourceObj *unstructured.Unstructured
	projectObj  *unstructured.Unstructured
	inputLinks  []settings.DeepLink
	outputLinks []*application.LinkInfo
	error       []string
}

func TestDeepLinks(t *testing.T) {
	t.Parallel()

	appObj, err := kube.ToUnstructured(&v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "test",
		},
		Spec: v1alpha1.ApplicationSpec{
			Destination: v1alpha1.ApplicationDestination{
				Server:    "test.example.com",
				Namespace: "testns",
			},
		},
	})
	require.NoError(t, err)
	resourceObj, err := kube.ToUnstructured(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cm",
			Namespace: "test-cm",
			Labels:    map[string]string{"test-label": "cm-value"},
		},
		Data: map[string]string{
			"key": "value1",
		},
	})
	require.NoError(t, err)
	clusterObj, err := kube.ToUnstructured(&ClusterLinksData{
		Server: "test-svc.com",
		Name:   "test-cluster",
	})
	require.NoError(t, err)
	projectObj, err := kube.ToUnstructured(&v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "test-project",
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"test-repo.git"},
		},
	})
	require.NoError(t, err)
	testTable := []deepLinkTC{
		{
			name:        "link to git repo per cluster",
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			clusterObj:  clusterObj,
			inputLinks: []settings.DeepLink{{
				Title:     "link",
				URL:       "http://example.com/{{ .application.metadata.name }}&{{ .resource.data.key }}&{{ index .project.spec.sourceRepos 0}}&{{ .cluster.name }}",
				Condition: new(`application.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: new("link"),
				Url:   new("http://example.com/test&value1&test-repo.git&test-cluster"),
			}},
			error: []string{},
		},
		{
			name:        "link to git repo per cluster with abbreviated name",
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			clusterObj:  clusterObj,
			inputLinks: []settings.DeepLink{{
				Title:     "link",
				URL:       "http://example.com/{{ .app.metadata.name }}&{{ .resource.data.key }}&{{ index .project.spec.sourceRepos 0}}&{{ .cluster.name }}",
				Condition: new(`app.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: new("link"),
				Url:   new("http://example.com/test&value1&test-repo.git&test-cluster"),
			}},
			error: []string{},
		},
		{
			name:        "condition on missing key",
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			inputLinks: []settings.DeepLink{
				{
					Title:     "link",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: new(`application.metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: new(`application.metadata.name matches "test1"`),
				},
				{
					Title:     "link2",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: new(`application.metadata.test matches "test"`),
				},
			},
			outputLinks: []*application.LinkInfo{{
				Title: new("link"),
				Url:   new("http://example.com/test&testns"),
			}},
			error: []string{},
		},
		{
			name:        "condition on invalid expression",
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			inputLinks: []settings.DeepLink{
				{
					Title:     "link",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: new(`application.metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: new(`1 + 1`),
				},
			},
			outputLinks: []*application.LinkInfo{{
				Title: new("link"),
				Url:   new("http://example.com/test&testns"),
			}},
			error: []string{"link condition '1 + 1' evaluated to non-boolean value for resource test"},
		},
		{
			name:        "condition on app and project name",
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			clusterObj:  clusterObj,
			inputLinks: []settings.DeepLink{{
				Title:     "link",
				URL:       "http://example.com/{{ .cluster.name | replace \"-\" \"_\" }}&{{ first .project.spec.sourceRepos }}",
				Condition: new(`application.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: new("link"),
				Url:   new("http://example.com/test_cluster&test-repo.git"),
			}},
			error: []string{},
		},
		{
			name:        "evaluate template for valid condition",
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			inputLinks: []settings.DeepLink{
				{
					Title:     "link",
					URL:       "http://not-evaluated.com/{{ index \"invalid\" .application.metadata.labels }}",
					Condition: new(`false`),
				},
				{
					Title:     "link",
					URL:       "http://evaluated.com/{{ index \"invalid\" .application.metadata.labels }}",
					Condition: new(`true`),
				},
			},
			outputLinks: []*application.LinkInfo{},
			error: []string{
				"failed to evaluate link template 'http://evaluated.com/{{ index \"invalid\" .application.metadata.labels }}' with resource test, error=template: deep-link:1:24: executing \"deep-link\" at <index \"invalid\" .application.metadata.labels>: error calling index: cannot index slice/array with nil",
			},
		},
	}

	for _, tc := range testTable {
		tcc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			objs := CreateDeepLinksObject(tcc.resourceObj, tcc.appObj, tcc.clusterObj, tcc.projectObj)
			output, err := EvaluateDeepLinksResponse(objs, tcc.appObj.GetName(), tcc.inputLinks)
			assert.Equal(t, tcc.error, err, strings.Join(err, ","))
			assert.True(t, reflect.DeepEqual(output.Items, tcc.outputLinks))
		})
	}
}

// TestManagedByURLAnnotation tests the managed-by-url annotation logic
func TestManagedByURLAnnotation(t *testing.T) {
	t.Run("application with managed-by-url annotation", func(t *testing.T) {
		managedByURL := "https://argocd-instance-b.example.com"

		// Create an application with managed-by-url annotation
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-app",
				Annotations: map[string]string{
					v1alpha1.AnnotationKeyManagedByURL: managedByURL,
				},
			},
		}

		// Convert to unstructured for the deeplinks function
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(app)
		require.NoError(t, err)
		unstructuredObj := &unstructured.Unstructured{Object: obj}

		// Test the deeplinks logic
		deeplinksObj := CreateDeepLinksObject(nil, unstructuredObj, nil, nil)

		// Verify that the managed-by-url is included
		assert.Equal(t, managedByURL, deeplinksObj[ManagedByURLKey])
	})

	t.Run("application without managed-by-url annotation", func(t *testing.T) {
		// Create an application without managed-by-url annotation
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-app",
			},
		}

		// Convert to unstructured for the deeplinks function
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(app)
		require.NoError(t, err)
		unstructuredObj := &unstructured.Unstructured{Object: obj}

		// Test the deeplinks logic
		deeplinksObj := CreateDeepLinksObject(nil, unstructuredObj, nil, nil)

		// Verify that no managed-by-url is set
		assert.Empty(t, deeplinksObj[ManagedByURLKey])
	})

	t.Run("application with empty managed-by-url annotation", func(t *testing.T) {
		// Create an application with empty managed-by-url annotation
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-app",
				Annotations: map[string]string{
					v1alpha1.AnnotationKeyManagedByURL: "",
				},
			},
		}

		// Convert to unstructured for the deeplinks function
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(app)
		require.NoError(t, err)
		unstructuredObj := &unstructured.Unstructured{Object: obj}

		// Test the deeplinks logic
		deeplinksObj := CreateDeepLinksObject(nil, unstructuredObj, nil, nil)

		// Verify that empty managed-by-url is set
		assert.Empty(t, deeplinksObj[ManagedByURLKey])
	})

	t.Run("application with managed-by-url and other annotations", func(t *testing.T) {
		managedByURL := "https://argocd-instance-b.example.com"

		// Create an application with managed-by-url and other annotations
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-app",
				Annotations: map[string]string{
					v1alpha1.AnnotationKeyManagedByURL: managedByURL,
					"argocd.argoproj.io/deep-link-1":   "https://grafana.example.com/d/argo/argo-cd-application-dashboard",
					"argocd.argoproj.io/deep-link-2":   "https://kibana.example.com/app/kibana#/discover",
				},
			},
		}

		// Convert to unstructured for the deeplinks function
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(app)
		require.NoError(t, err)
		unstructuredObj := &unstructured.Unstructured{Object: obj}

		// Test the deeplinks logic
		deeplinksObj := CreateDeepLinksObject(nil, unstructuredObj, nil, nil)

		// Verify that managed-by-url is included
		assert.Equal(t, managedByURL, deeplinksObj[ManagedByURLKey])

		// Note: Other deep link annotations are not processed by CreateDeepLinksObject
		// They are processed separately by the EvaluateDeepLinksResponse function
	})
}
