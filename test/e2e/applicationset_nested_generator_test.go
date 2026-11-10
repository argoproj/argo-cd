package e2e

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/applicationset"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

func TestApplicationSetGenerateNestingDepth(t *testing.T) {
	fixture.EnsureCleanState(t)
	closer, client := fixture.ArgoCDClientset.NewApplicationSetClientOrDie()
	defer utilio.Close(closer)

	for _, kind := range []string{"list", "matrix", "merge"} {
		t.Run(kind, func(t *testing.T) {
			child := `{"list":{"elements":[{"name":"nested-depth"}]}}`
			if kind != "list" {
				child = fmt.Sprintf(`{%q:{"mergeKeys":["name"],"generators":[%s,%s]}}`, kind, child, child)
			}
			var appSet v1alpha1.ApplicationSet
			require.NoError(t, json.Unmarshal(fmt.Appendf(nil, `{
				"metadata":{"name":"nested-depth"},
				"spec":{
					"goTemplate":true,
					"generators":[{"matrix":{"generators":[
						{"list":{"elements":[{"name":"nested-depth"}]}},
						{"matrix":{"generators":[{"list":{"elements":[{"name":"nested-depth"}]}},%s]}}
					]}}],
					"template":{"metadata":{"name":"{{.name}}"},"spec":{
						"project":"default",
						"source":{"repoURL":"https://github.com/argoproj/argo-cd.git","path":"applicationset/examples/list-generator/guestbook/engineering-dev","targetRevision":"HEAD"},
						"destination":{"server":"https://kubernetes.default.svc","namespace":"default"}
					}}
				}
			}`, child), &appSet))
			response, err := client.Generate(t.Context(), &applicationset.ApplicationSetGenerateRequest{ApplicationSet: &appSet})
			if kind == "list" {
				require.NoError(t, err)
				require.Len(t, response.Applications, 1)
				require.Equal(t, "nested-depth", response.Applications[0].Name)
			} else {
				require.ErrorContains(t, err, "combination-type generators can only be nested once: found "+kind+" at generators[1]")
			}
		})
	}
}
