package deeplinks

import (
	"reflect"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type deepLinkTC struct {
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
	assert.NoError(t, err)
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
	assert.NoError(t, err)
	clusterObj, err := kube.ToUnstructured(&ClusterLinksData{
		Server: "test-svc.com",
		Name:   "test-cluster",
	})
	assert.NoError(t, err)
	projectObj, err := kube.ToUnstructured(&v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-project",
			Namespace: "test-project",
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"test-repo.git"},
		},
	})
	assert.NoError(t, err)
	testTable := []deepLinkTC{
		{
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			clusterObj:  clusterObj,
			inputLinks: []settings.DeepLink{{
				Title:     "link",
				URL:       "http://example.com/{{ .application.metadata.name }}&{{ .resource.data.key }}&{{ index .project.spec.sourceRepos 0}}&{{ .cluster.name }}",
				Condition: pointer.String(`application.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test&value1&test-repo.git&test-cluster"),
			}},
			error: []string{},
		},
		{
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			clusterObj:  clusterObj,
			inputLinks: []settings.DeepLink{{
				Title:     "link",
				URL:       "http://example.com/{{ .app.metadata.name }}&{{ .resource.data.key }}&{{ index .project.spec.sourceRepos 0}}&{{ .cluster.name }}",
				Condition: pointer.String(`app.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test&value1&test-repo.git&test-cluster"),
			}},
			error: []string{},
		},
		{
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			inputLinks: []settings.DeepLink{
				{
					Title:     "link",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: pointer.String(`application.metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: pointer.String(`application.metadata.name matches "test1"`),
				},
				{
					Title:     "link2",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: pointer.String(`application.metadata.test matches "test"`),
				}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test&testns"),
			}},
			error: []string{"failed to evaluate link condition 'application.metadata.test matches \"test\"' with resource test, error=interface conversion: interface {} is nil, not string (1:27)\n | application.metadata.test matches \"test\"\n | ..........................^"},
		},
		{
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			inputLinks: []settings.DeepLink{
				{
					Title:     "link",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: pointer.String(`application.metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .application.metadata.name }}&{{ .application.spec.destination.namespace }}",
					Condition: pointer.String(`1 + 1`),
				}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test&testns"),
			}},
			error: []string{"link condition '1 + 1' evaluated to non-boolean value for resource test"},
		},
		{
			appObj:      appObj,
			resourceObj: resourceObj,
			projectObj:  projectObj,
			clusterObj:  clusterObj,
			inputLinks: []settings.DeepLink{{
				Title:     "link",
				URL:       "http://example.com/{{ .cluster.name | replace \"-\" \"_\" }}&{{ first .project.spec.sourceRepos }}",
				Condition: pointer.String(`application.metadata.name == "test" && project.metadata.name == "test-project"`),
			}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test_cluster&test-repo.git"),
			}},
			error: []string{},
		},
	}

	for _, tc := range testTable {
		objs := CreateDeepLinksObject(tc.resourceObj, tc.appObj, tc.clusterObj, tc.projectObj)
		output, err := EvaluateDeepLinksResponse(objs, tc.appObj.GetName(), tc.inputLinks)
		assert.Equal(t, tc.error, err, strings.Join(err, ","))
		assert.Equal(t, reflect.DeepEqual(output.Items, tc.outputLinks), true)
	}
}
