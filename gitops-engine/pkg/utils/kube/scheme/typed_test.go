package scheme

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"
	smdschema "sigs.k8s.io/structured-merge-diff/v6/schema"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

func TestResolveParseableType_NilParser(t *testing.T) {
	t.Run("built in kinds use the static schema", func(t *testing.T) {
		pt := ResolveParseableType(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, nil)
		require.NotNil(t, pt)
		require.NotNil(t, pt.TypeRef.NamedType)
		assert.Equal(t, "io.k8s.api.apps.v1.Deployment", *pt.TypeRef.NamedType)

		// The schema is real: containers is a list keyed by name.
		containers := field(t, pt, "spec", "template", "spec", "containers")
		assert.Equal(t, smdschema.Associative, containers.List.ElementRelationship)
		assert.Equal(t, []string{"name"}, containers.Keys)
	})
	t.Run("core kinds use the static schema", func(t *testing.T) {
		pt := ResolveParseableType(schema.GroupVersionKind{Version: "v1", Kind: "Service"}, nil)
		require.NotNil(t, pt.TypeRef.NamedType)
		assert.Equal(t, "io.k8s.api.core.v1.Service", *pt.TypeRef.NamedType)
	})
	t.Run("other kinds are deduced", func(t *testing.T) {
		pt := ResolveParseableType(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Rollout"}, nil)
		assert.Same(t, &typed.DeducedParseableType, pt)
	})
}

// field walks struct fields of a parseable type and returns the atom of the last one.
func field(t *testing.T, pt *typed.ParseableType, names ...string) smdschema.Atom {
	t.Helper()
	ref := pt.TypeRef
	var atom smdschema.Atom
	for _, name := range names {
		a, ok := pt.Schema.Resolve(ref)
		require.True(t, ok, "resolve %v", ref)
		require.NotNil(t, a.Map, "%s is not a struct", name)
		f, ok := a.FindField(name)
		require.True(t, ok, "no field %s", name)
		ref = f.Type
	}
	atom, ok := pt.Schema.Resolve(ref)
	require.True(t, ok)
	return atom
}
