package admin

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/common"
)

func newBackupObject(trackingValue string, trackingLabel bool, trackingAnnotation bool) *unstructured.Unstructured {
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-configmap",
			Namespace: "namespace",
		},
		Data: map[string]string{
			"foo": "bar",
		},
	}
	if trackingLabel {
		cm.SetLabels(map[string]string{
			common.LabelKeyAppInstance: trackingValue,
		})
	}
	if trackingAnnotation {
		cm.SetAnnotations(map[string]string{
			common.AnnotationKeyAppInstance: trackingValue,
		})
	}
	return kube.MustToUnstructured(&cm)
}

func Test_updateTracking(t *testing.T) {
	type args struct {
		bak  *unstructured.Unstructured
		live *unstructured.Unstructured
	}
	tests := []struct {
		name     string
		args     args
		expected *unstructured.Unstructured
	}{
		{
			name: "update annotation when present in live",
			args: args{
				bak:  newBackupObject("bak", false, true),
				live: newBackupObject("live", false, true),
			},
			expected: newBackupObject("live", false, true),
		},
		{
			name: "update default label when present in live",
			args: args{
				bak:  newBackupObject("bak", true, true),
				live: newBackupObject("live", true, true),
			},
			expected: newBackupObject("live", true, true),
		},
		{
			name: "do not update if live object does not have tracking",
			args: args{
				bak:  newBackupObject("bak", true, true),
				live: newBackupObject("live", false, false),
			},
			expected: newBackupObject("bak", true, true),
		},
		{
			name: "do not update if bak object does not have tracking",
			args: args{
				bak:  newBackupObject("bak", false, false),
				live: newBackupObject("live", true, true),
			},
			expected: newBackupObject("bak", false, false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			updateTracking(tt.args.bak, tt.args.live)
			assert.Equal(t, tt.expected, tt.args.bak)
		})
	}
}

func TestIsSkipLabelMatches(t *testing.T) {
	tests := []struct {
		name       string
		obj        *unstructured.Unstructured
		skipLabels string
		expected   bool
	}{
		{
			name: "Label matches",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"test-label": "value",
						},
					},
				},
			},
			skipLabels: "test-label=value",
			expected:   true,
		},
		{
			name: "Label does not match",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"different-label": "value",
						},
					},
				},
			},
			skipLabels: "test-label=value",
			expected:   false,
		},
		{
			name: "Empty skip labels",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"test-label": "value",
						},
					},
				},
			},
			skipLabels: "",
			expected:   false,
		},
		{
			name: "No labels value",
			obj: &unstructured.Unstructured{
				Object: map[string]any{
					"metadata": map[string]any{
						"labels": map[string]any{
							"test-label":    "value",
							"another-label": "value2",
						},
					},
				},
			},
			skipLabels: "test-label",
			expected:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSkipLabelMatches(tt.obj, tt.skipLabels)
			assert.Equal(t, tt.expected, result)
		})
	}
}
