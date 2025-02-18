package admin

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/common"
)

func newBackupObject(trackingValue string, trackingLabel bool, trackingAnnotation bool) *unstructured.Unstructured {
	cm := v1.ConfigMap{
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
