package deeplinks

import (
	"reflect"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
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
	resourceObj, err := kube.ToUnstructured(&v1.ConfigMap{
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
				Condition: ptr.To(`application.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: ptr.To("link"),
				Url:   ptr.To("http://example.com/test&value1&test-repo.git&test-cluster"),
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
				Condition: ptr.To(`app.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: ptr.To("link"),
				Url:   ptr.To("http://example.com/test&value1&test-repo.git&test-cluster"),
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
					Condition: ptr.To(`application.metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: ptr.To(`application.metadata.name matches "test1"`),
				},
				{
					Title:     "link2",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: ptr.To(`application.metadata.test matches "test"`),
				},
			},
			outputLinks: []*application.LinkInfo{{
				Title: ptr.To("link"),
				Url:   ptr.To("http://example.com/test&testns"),
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
					Condition: ptr.To(`application.metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: ptr.To(`1 + 1`),
				},
			},
			outputLinks: []*application.LinkInfo{{
				Title: ptr.To("link"),
				Url:   ptr.To("http://example.com/test&testns"),
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
				Condition: ptr.To(`application.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: ptr.To("link"),
				Url:   ptr.To("http://example.com/test_cluster&test-repo.git"),
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
					Condition: ptr.To(`false`),
				},
				{
					Title:     "link",
					URL:       "http://evaluated.com/{{ index \"invalid\" .application.metadata.labels }}",
					Condition: ptr.To(`true`),
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
