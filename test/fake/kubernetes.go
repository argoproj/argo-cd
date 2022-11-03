package fake

import (
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func NewFakeKubeClient() *kubefake.Clientset {
	clientset := kubefake.NewSimpleClientset()
	return clientset
}

func NewFakeClientsetWithResources(objects ...runtime.Object) *kubefake.Clientset {
	clientset := kubefake.NewSimpleClientset(objects...)
	return clientset
}
