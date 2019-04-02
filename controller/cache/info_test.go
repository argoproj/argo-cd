package cache

import (
	"testing"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/kube"
	v1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
)

func TestGetPodInfo(t *testing.T) {
	pod := testPod.DeepCopy()
	pod.SetLabels(map[string]string{"app": "guestbook"})

	info, networkInfo := getNodeInfo(pod)
	assert.Equal(t, []v1alpha1.InfoItem{{Name: "Containers", Value: "0/0"}}, info)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{Labels: map[string]string{"app": "guestbook"}}, networkInfo)
}

func TestGetServiceInfo(t *testing.T) {
	info, networkInfo := getNodeInfo(testService)
	assert.Equal(t, 0, len(info))
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetLabels: map[string]string{"app": "guestbook"},
		Ingress:      []v1.LoadBalancerIngress{{Hostname: "localhost"}},
	}, networkInfo)
}

func TestGetIngressInfo(t *testing.T) {
	info, networkInfo := getNodeInfo(testIngress)
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
