package generators

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestCombinationGeneratorNestingDepth(t *testing.T) {
	terminal := map[string]Generator{"List": NewListGenerator()}
	nested := map[string]Generator{
		"List":   NewListGenerator(),
		"Matrix": NewMatrixGenerator(terminal),
		"Merge":  NewMergeGenerator(terminal),
	}
	outer := map[string]Generator{
		"matrix": NewMatrixGenerator(nested),
		"merge":  NewMergeGenerator(nested),
	}
	for outerKind, generator := range outer {
		for _, innerKind := range []string{"matrix", "merge"} {
			for _, childKind := range []string{"list", "matrix", "merge"} {
				t.Run(outerKind+"/"+innerKind+"/"+childKind, func(t *testing.T) {
					child := `{"list":{"elements":[{"name":"app"}]}}`
					if childKind != "list" {
						child = fmt.Sprintf(`{%q:{"mergeKeys":["name"],"generators":[%s,%s]}}`, childKind, child, child)
					}
					inner := fmt.Sprintf(`{%q:{"mergeKeys":["name"],"generators":[{"list":{"elements":[{"name":"app"}]}},%s]}}`, innerKind, child)
					raw := fmt.Sprintf(`{%q:{"mergeKeys":["name"],"generators":[{"list":{"elements":[{"name":"app"}]}},%s]}}`, outerKind, inner)
					var spec v1alpha1.ApplicationSetGenerator
					require.NoError(t, json.Unmarshal([]byte(raw), &spec))
					params, err := generator.GenerateParams(&spec, &v1alpha1.ApplicationSet{Spec: v1alpha1.ApplicationSetSpec{GoTemplate: true}}, nil)
					if childKind == "list" {
						require.NoError(t, err)
						require.Equal(t, []map[string]any{{"name": "app"}}, params)
					} else {
						require.ErrorContains(t, err, "combination-type generators can only be nested once: found "+childKind+" at generators[1]")
						require.Nil(t, params)
					}
				})
			}
		}
	}
}
