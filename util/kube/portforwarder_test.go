package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_selectPodForPortForward(t *testing.T) {
	// Mock the Kubernetes client
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test-app",
				},
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
					},
				},
				Phase: corev1.PodRunning,
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2-pod-broken",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test2",
				},
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionFalse,
						LastTransitionTime: metav1.Now(),
					},
				},
				Phase: corev1.PodFailed,
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test2-pod-working",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test2",
				},
			},
			Status: corev1.PodStatus{
				Conditions: []corev1.PodCondition{
					{
						Type:               corev1.PodReady,
						Status:             corev1.ConditionTrue,
						LastTransitionTime: metav1.Now(),
					},
				},
				Phase: corev1.PodRunning,
			},
		},
	)

	// Test selecting the pod
	selectedPod, err := selectPodForPortForward(client, "default", "app=test-app")
	require.NoError(t, err)
	require.Equal(t, "test-pod", selectedPod.Name)

	// Test selecting the working pod
	selectedPod2, err := selectPodForPortForward(client, "default", "app=test2")
	require.NoError(t, err)
	require.Equal(t, "test2-pod-working", selectedPod2.Name)
}
