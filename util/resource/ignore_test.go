package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
)

func TestIgnore(t *testing.T) {
	tests := []struct {
		name string
		obj  *unstructured.Unstructured
		want bool
	}{
		{"IgnoreHelmHook", test.HelmHook(test.NewPod(), "pre-install"), true},
		{"DoNotIgnoreEmpty", test.NewPod(), false},
		{"DoNotIgnoreHook", test.Hook(test.NewPod(), v1alpha1.HookTypeSync), false},
		{"DoNotIgnoreHelmCrdInstall", test.HelmHook(test.NewPod(), "crd-install"), false},
		{"DoNotIgnoreHelmPlusArgoHook", test.HelmHook(test.Hook(test.NewPod(), v1alpha1.HookTypeSync), "pre-install"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Ignore(tt.obj))
		})
	}
}
