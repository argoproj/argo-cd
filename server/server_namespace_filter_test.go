package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func Test_newNamespaceFilterTransform(t *testing.T) {
	const serverNS = "argocd"

	app := func(ns string) *v1alpha1.Application {
		return &v1alpha1.Application{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: ns}}
	}
	appSet := func(ns string) *v1alpha1.ApplicationSet {
		return &v1alpha1.ApplicationSet{ObjectMeta: metav1.ObjectMeta{Name: "appset", Namespace: ns}}
	}

	tests := []struct {
		name            string
		allowed         []string
		obj             any
		wantKept        bool
		wantPassthrough bool // for non-meta objects we expect the original obj returned unchanged
	}{
		{
			name:     "control-plane namespace is always kept",
			allowed:  []string{"team-*"},
			obj:      app(serverNS),
			wantKept: true,
		},
		{
			name:     "namespace matching an allowed glob pattern is kept",
			allowed:  []string{"team-*"},
			obj:      app("team-alpha"),
			wantKept: true,
		},
		{
			name:     "namespace not matching any pattern is dropped",
			allowed:  []string{"team-*"},
			obj:      app("other"),
			wantKept: false,
		},
		{
			name:     "exact namespace match is kept",
			allowed:  []string{"foo", "bar"},
			obj:      app("bar"),
			wantKept: true,
		},
		{
			name:     "applicationset in disallowed namespace is dropped",
			allowed:  []string{"team-a"},
			obj:      appSet("team-b"),
			wantKept: false,
		},
		{
			name:     "applicationset in allowed namespace is kept",
			allowed:  []string{"team-a"},
			obj:      appSet("team-a"),
			wantKept: true,
		},
		{
			name:            "tombstone-like non-meta object is passed through",
			allowed:         []string{"foo"},
			obj:             cache.DeletedFinalStateUnknown{Key: "ns/name", Obj: nil},
			wantKept:        true,
			wantPassthrough: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			transform := newNamespaceFilterTransform(serverNS, tc.allowed)
			got, err := transform(tc.obj)
			require.NoError(t, err)
			if tc.wantKept {
				assert.NotNil(t, got, "object should be kept")
				if !tc.wantPassthrough {
					assert.Same(t, tc.obj, got, "kept object should be returned unchanged")
				}
			} else {
				assert.Nil(t, got, "object should be dropped")
			}
		})
	}
}
