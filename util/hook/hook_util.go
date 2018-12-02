package hook

import (
	"testing"

	"github.com/stretchr/testify/assert"

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
