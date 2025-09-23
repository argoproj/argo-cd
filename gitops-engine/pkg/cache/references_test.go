package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
)

func Test_isStatefulSetChild(t *testing.T) {
	type args struct {
		un *unstructured.Unstructured
	}

	statefulSet := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "sw-broker",
		},
		Spec: appsv1.StatefulSetSpec{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "emqx-data",
					},
				},
			},
		},
	}

	// Create a new unstructured object from the JSON string
	un, err := kube.ToUnstructured(statefulSet)
	require.NoErrorf(t, err, "Failed to convert StatefulSet to unstructured: %v", err)

	tests := []struct {
		name      string
		args      args
		wantErr   bool
		checkFunc func(func(kube.ResourceKey) bool) bool
	}{
		{
			name:    "Valid PVC for sw-broker",
			args:    args{un: un},
			wantErr: false,
			checkFunc: func(fn func(kube.ResourceKey) bool) bool {
				// Check a valid PVC name for "sw-broker"
				return fn(kube.ResourceKey{Kind: "PersistentVolumeClaim", Name: "emqx-data-sw-broker-0"})
			},
		},
		{
			name:    "Invalid PVC for sw-broker",
			args:    args{un: un},
			wantErr: false,
			checkFunc: func(fn func(kube.ResourceKey) bool) bool {
				// Check an invalid PVC name that should belong to "sw-broker-internal"
				return !fn(kube.ResourceKey{Kind: "PersistentVolumeClaim", Name: "emqx-data-sw-broker-internal-0"})
			},
		},
		{
			name: "Mismatch PVC for sw-broker",
			args: args{un: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "apps/v1",
					"kind":       "StatefulSet",
					"metadata": map[string]any{
						"name": "sw-broker",
					},
					"spec": map[string]any{
						"volumeClaimTemplates": []any{
							map[string]any{
								"metadata": map[string]any{
									"name": "volume-2",
								},
							},
						},
					},
				},
			}},
			wantErr: false,
			checkFunc: func(fn func(kube.ResourceKey) bool) bool {
				// Check an invalid PVC name for "api-test"
				return !fn(kube.ResourceKey{Kind: "PersistentVolumeClaim", Name: "volume-2"})
			},
		},
	}

	// Execute test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isStatefulSetChild(tt.args.un)
			assert.Equal(t, tt.wantErr, err != nil, "isStatefulSetChild() error = %v, wantErr %v", err, tt.wantErr)
			if err == nil {
				assert.True(t, tt.checkFunc(got), "Check function failed for %v", tt.name)
			}
		})
	}
}
