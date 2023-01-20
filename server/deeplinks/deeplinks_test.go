package deeplinks

import (
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/pointer"
	"reflect"
	"strings"
	"testing"
)

type deepLinkTC struct {
	obj         *unstructured.Unstructured
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
				Server:    "test.com",
				Namespace: "testns",
			},
		},
	})
	assert.NoError(t, err)
	testTable := []deepLinkTC{
		{
			obj: appObj,
			inputLinks: []settings.DeepLink{{
				Title: "link",
				URL:   "http://example.com/{{ .metadata.name }}&{{ .spec.destination.namespace }}",
			}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test&testns"),
			}},
			error: []string{},
		},
		{
			obj: appObj,
			inputLinks: []settings.DeepLink{
				{
					Title:     "link",
					URL:       "http://example.com/{{ .metadata.name }}&{{ .spec.destination.namespace }}",
					Condition: pointer.String(`metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .metadata.name }}&{{ .spec.destination.namespace }}",
					Condition: pointer.String(`metadata.name matches "test1"`),
				},
				{
					Title:     "link2",
					URL:       "http://example.com/{{ .metadata.name }}&{{ .spec.destination.namespace }}",
					Condition: pointer.String(`metadata.test matches "test"`),
				}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test&testns"),
			}},
			error: []string{"failed to evaluate link condition 'metadata.test matches \"test\"' with resource test, error=interface conversion: interface {} is nil, not string (1:15)\n | metadata.test matches \"test\"\n | ..............^"},
		},
		{
			obj: appObj,
			inputLinks: []settings.DeepLink{
				{
					Title:     "link",
					URL:       "http://example.com/{{ .metadata.name }}&{{ .spec.destination.namespace }}",
					Condition: pointer.String(`metadata.name matches "test"`),
				},
				{
					Title:     "link1",
					URL:       "http://example.com/{{ .metadata.name }}&{{ .spec.destination.namespace }}",
					Condition: pointer.String(`1 + 1`),
				}},
			outputLinks: []*application.LinkInfo{{
				Title: pointer.String("link"),
				Url:   pointer.String("http://example.com/test&testns"),
			}},
			error: []string{"link condition '1 + 1' evaluated to non-boolean value for resource test"},
		},
	}

	for _, tc := range testTable {
		output, err := EvaluateDeepLinksResponse(*tc.obj, tc.inputLinks)
		assert.Equal(t, tc.error, err, strings.Join(err, ","))
		assert.Equal(t, reflect.DeepEqual(output.Items, tc.outputLinks), true)
	}
}
