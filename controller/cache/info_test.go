package cache

import (
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"

	"github.com/stretchr/testify/assert"
)

func TestGetPodInfo(t *testing.T) {
	pod := strToUnstructured(`
  apiVersion: v1
  kind: Pod
  metadata:
    name: helm-guestbook-pod
    namespace: default
    ownerReferences:
    - apiVersion: extensions/v1beta1
      kind: ReplicaSet
      name: helm-guestbook-rs
    resourceVersion: "123"
    labels:
      app: guestbook
  spec:
    containers:
    - image: bar`)

	info, images, networkInfo := getNodeInfo(pod)
	assert.Equal(t, []v1alpha1.InfoItem{{Name: "Containers", Value: "0/1"}}, info)
	assert.Equal(t, []string{"bar"}, images)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{Labels: map[string]string{"app": "guestbook"}}, networkInfo)
}

func TestGetServiceInfo(t *testing.T) {
	info, _, networkInfo := getNodeInfo(testService)
	assert.Equal(t, 0, len(info))
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetLabels: map[string]string{"app": "guestbook"},
		Ingress:      []v1.LoadBalancerIngress{{Hostname: "localhost"}},
	}, networkInfo)
}

func TestGetIngressInfo(t *testing.T) {
	info, _, networkInfo := getNodeInfo(testIngress)
	assert.Equal(t, 0, len(info))
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		Ingress: []v1.LoadBalancerIngress{{IP: "107.178.210.11"}},
		TargetRefs: []v1alpha1.ResourceRef{{
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "not-found-service",
		}, {
			Namespace: "default",
			Group:     "",
			Kind:      kube.ServiceKind,
			Name:      "helm-guestbook",
		}},
	}, networkInfo)
}
