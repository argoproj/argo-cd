package kube

import (
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_selectPodForPortForward(t *testing.T) {
	// Mock the Kubernetes client
	client := fake.NewSimpleClientset(
		&v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
				Labels: map[string]string{
					"app": "test-app",
				},
			},
		},
	)

	// Test selecting the pod
	selectedPod, err := selectPodForPortForward(client, "default", "app=test-app")
	require.NoError(t, err)
	require.Equal(t, "test-pod", selectedPod.Name)
}
