package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestNestedGeneratorDepth(t *testing.T) {
	converters := map[string]func(*apiextensionsv1.JSON) (ApplicationSetTerminalGenerators, error){
		"matrix": func(raw *apiextensionsv1.JSON) (ApplicationSetTerminalGenerators, error) {
			g, err := ToNestedMatrixGenerator(raw)
			if g == nil {
				return nil, err
			}
			return g.Generators, err
		},
		"merge": func(raw *apiextensionsv1.JSON) (ApplicationSetTerminalGenerators, error) {
			g, err := ToNestedMergeGenerator(raw)
			if g == nil {
				return nil, err
			}
			return g.Generators, err
		},
	}
	for name, convert := range converters {
		t.Run(name, func(t *testing.T) {
			for _, kind := range []string{"matrix", "merge"} {
				t.Run(kind, func(t *testing.T) {
					for _, value := range []string{`{}`, `{"generators":[]}`} {
						_, err := convert(&apiextensionsv1.JSON{Raw: []byte(`{"generators":[{"list":{"elements":[]}},{"` + kind + `":` + value + `}]}`)})
						require.EqualError(t, err, "combination-type generators can only be nested once: found "+kind+" at generators[1]")
					}
				})
			}
			t.Run("list data and null combination fields", func(t *testing.T) {
				generators, err := convert(&apiextensionsv1.JSON{Raw: []byte(`{"generators":[{"list":{"elements":[{"matrix":{"name":"app"},"merge":"value"}]},"matrix":null,"merge":null}]}`)})
				require.NoError(t, err)
				require.Len(t, generators, 1)
				require.NotNil(t, generators[0].List)
				require.JSONEq(t, `{"matrix":{"name":"app"},"merge":"value"}`, string(generators[0].List.Elements[0].Raw))
			})
			t.Run("nil", func(t *testing.T) {
				generators, err := convert(nil)
				require.NoError(t, err)
				require.Nil(t, generators)
			})
			t.Run("invalid JSON", func(t *testing.T) {
				_, err := convert(&apiextensionsv1.JSON{Raw: []byte(`{"generators":`)})
				require.Error(t, err)
			})
			t.Run("empty and unknown fields", func(t *testing.T) {
				for _, raw := range []string{`null`, `{}`, `{"generators":null}`, `{"generators":[]}`, `{"generators":[{"unknown":{}}]}`} {
					_, err := convert(&apiextensionsv1.JSON{Raw: []byte(raw)})
					require.NoError(t, err)
				}
			})
		})
	}
}
