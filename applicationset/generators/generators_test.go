package generators

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/pkg/apis/applicationset/v1alpha1"
)

func TestGetRelevantGenerators(t *testing.T) {
	requestedGenerator := &v1alpha1.ApplicationSetGenerator{
		List: &v1alpha1.ListGenerator{},
	}
	allGenerators := map[string]Generator{
		"List": NewListGenerator(),
	}
	relevantGenerators := GetRelevantGenerators(requestedGenerator, allGenerators)

	for _, generator := range relevantGenerators {
		if generator == nil {
			t.Fatal(`GetRelevantGenerators produced a nil generator`)
		}
	}

	numRelevantGenerators := len(relevantGenerators)
	if numRelevantGenerators != 1 {
		t.Fatalf(`GetRelevantGenerators produced %d generators instead of the expected 1`, numRelevantGenerators)
	}
}

func TestNoGeneratorNilReferenceError(t *testing.T) {
	generators := []Generator{
		&ClusterGenerator{},
		&DuckTypeGenerator{},
		&GitGenerator{},
		&ListGenerator{},
		&MatrixGenerator{},
		&MergeGenerator{},
		&PullRequestGenerator{},
		&SCMProviderGenerator{},
	}

	for _, generator := range generators {
		testCaseCopy := generator // since tests may run in parallel

		generatorName := reflect.TypeOf(testCaseCopy).Elem().Name()
		t.Run(fmt.Sprintf("%s does not throw a nil reference error when all generator fields are nil", generatorName), func(t *testing.T) {
			t.Parallel()

			params, err := generator.GenerateParams(&v1alpha1.ApplicationSetGenerator{}, &v1alpha1.ApplicationSet{})

			assert.ErrorIs(t, err, EmptyAppSetGeneratorError)
			assert.Nil(t, params)
		})
	}
}
