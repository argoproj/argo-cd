package controller

import (
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var podManifest = []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - image: nginx:1.7.9
    name: nginx
    resources:
      requests:
        cpu: 0.2
`)

func newPod() *unstructured.Unstructured {
	var un unstructured.Unstructured
	err := yaml.Unmarshal(podManifest, &un)
	if err != nil {
		panic(err)
	}
	return &un
}

func TestIsHook(t *testing.T) {
	pod := newPod()
	assert.False(t, isHook(pod))

	pod.SetAnnotations(map[string]string{"helm.sh/hook": "post-install"})
	assert.True(t, isHook(pod))

	pod = newPod()
	pod.SetAnnotations(map[string]string{"argocd.argoproj.io/hook": "PreSync"})
	assert.True(t, isHook(pod))

	pod = newPod()
	pod.SetAnnotations(map[string]string{"argocd.argoproj.io/hook": "Skip"})
	assert.False(t, isHook(pod))

	pod = newPod()
	pod.SetAnnotations(map[string]string{"argocd.argoproj.io/hook": "Unknown"})
	assert.False(t, isHook(pod))
}
