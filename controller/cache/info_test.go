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

	node := &node{}
	populateNodeInfo(pod, node)
	assert.Equal(t, []v1alpha1.InfoItem{{Name: "Containers", Value: "0/1"}}, node.info)
	assert.Equal(t, []string{"bar"}, node.images)
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{Labels: map[string]string{"app": "guestbook"}}, node.networkingInfo)
}

func TestGetServiceInfo(t *testing.T) {
	node := &node{}
	populateNodeInfo(testService, node)
	assert.Equal(t, 0, len(node.info))
	assert.Equal(t, &v1alpha1.ResourceNetworkingInfo{
		TargetLabels: map[string]string{"app": "guestbook"},
		Ingress:      []v1.LoadBalancerIngress{{Hostname: "localhost"}},
	}, node.networkingInfo)
}

func TestGetIngressInfo(t *testing.T) {
	node := &node{}
	populateNodeInfo(testIngress, node)
	assert.Equal(t, 0, len(node.info))
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
	}, node.networkingInfo)
}

func TestCreateHealth(t *testing.T) {
	tests := []struct {
		name   string
		status v1.PodStatus
		health v1alpha1.HealthStatus
		reason string
	}{
		{
			name:   "PodPending",
			status: v1.PodStatus{Phase: v1.PodPending, Message: "foo"},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusProgressing, Message: "foo"},
		},
		{
			name:   "PodRunning",
			status: v1.PodStatus{Phase: v1.PodRunning},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusHealthy},
		},
		{
			name:   "PodSucceeded",
			status: v1.PodStatus{Phase: v1.PodSucceeded},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusHealthy},
		},
		{
			name:   "PodFailed",
			status: v1.PodStatus{Phase: v1.PodFailed},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusDegraded},
		},
		{
			name:   "PodUnknown",
			status: v1.PodStatus{Phase: v1.PodUnknown},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusUnknown},
		},
		{
			name:   "PodReasonUnschedulable",
			status: v1.PodStatus{Phase: v1.PodReasonUnschedulable},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusMissing},
		},
		{
			name:   "PodReasonUnschedulable",
			status: v1.PodStatus{Phase: v1.PodReasonUnschedulable},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusMissing},
		},
		{
			name:   "CrashLoopBackOff",
			status: v1.PodStatus{},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusDegraded, Message: "CrashLookBackof"},
			reason: "CrashLoopBackOff",
		},
		{
			name:   "RunContainerError",
			status: v1.PodStatus{},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusDegraded, Message: "RunContainerError"},
			reason: "RunContainerError",
		},
		{
			name:   "ErrImagePull",
			status: v1.PodStatus{},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusDegraded, Message: "ErrImagePull"},
			reason: "ErrImagePull",
		},
		{
			name:   "ImagePullBackOff",
			status: v1.PodStatus{},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusDegraded, Message: "ImagePullBackOff"},
			reason: "ImagePullBackOff",
		}, {
			name:   "Message",
			status: v1.PodStatus{Phase: v1.PodPending, Message: "foo"},
			health: v1alpha1.HealthStatus{Status: v1alpha1.HealthStatusProgressing, Message: "foo"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.health, *createHealth(test.status, test.reason))
		})
	}
}
