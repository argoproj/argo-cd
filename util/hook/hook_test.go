package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/test"
)

func TestIsHook(t *testing.T) {
	pod := test.NewPod()
	assert.False(t, IsHook(pod))

	pod.SetAnnotations(map[string]string{"helm.sh/hook": "post-install"})
	assert.True(t, IsHook(pod))

	pod.SetAnnotations(map[string]string{"helm.sh/hook": "crd-install"})
	assert.False(t, IsHook(pod))

	pod = test.NewPod()
	pod.SetAnnotations(map[string]string{"argocd.argoproj.io/hook": "PreSync"})
	assert.True(t, IsHook(pod))

	pod = test.NewPod()
	pod.SetAnnotations(map[string]string{"argocd.argoproj.io/hook": "Skip"})
	assert.False(t, IsHook(pod))

	pod = test.NewPod()
	pod.SetAnnotations(map[string]string{"argocd.argoproj.io/hook": "Unknown"})
	assert.False(t, IsHook(pod))
}

func TestWeight(t *testing.T) {
	tests := []struct {
		name string
		obj  *unstructured.Unstructured
		want int
	}{
		{"TestDefaultWeight", test.NewHook(), 0},
		{"TestPositiveWeight", test.NewHookWithWeight("1"), 1},
		{"TestNegativeWeight", test.NewHookWithWeight("-1"), -1},
		{"TestGarbageWeight", test.NewHookWithWeight("foo"), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Weight(tt.obj); got != tt.want {
				t.Errorf("Weight() = %v, want %v", got, tt.want)
			}
		})
	}
}
