package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestHooks(t *testing.T) {
	tests := []struct {
		name           string
		obj            *unstructured.Unstructured
		wantIsArgoHook bool
		wantHookTypes  []HookType
	}{
		{"TestNoHooks", &unstructured.Unstructured{}, false, nil},
		{"TestOneHook", example("Sync"), true, []HookType{HookTypeSync}},
		// peculiar test, it IS a hook (because we don't  want to treat it as a resource), but does not have any valid types
		{"TestGarbageHook", example("Garbage"), true, nil},
		{"TestTwoHooks", example("PreSync,PostSync"), true, []HookType{HookTypePreSync, HookTypePostSync}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantIsArgoHook, IsArgoHook(tt.obj))
			assert.Equal(t, tt.wantHookTypes, Hooks(tt.obj))
		})
	}
}

func example(hook string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": map[string]interface{}{
					"argocd.argoproj.io/hook": hook,
				},
			},
		},
	}
}
