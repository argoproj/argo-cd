package resource

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/test"
)

func TestGetRevision(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name string
		args args
		want int64
	}{
		{"Nil", args{}, 0},
		{"Empty", args{obj: test.NewPod()}, 0},
		{"Invalid", args{obj: revisionExample("garbage")}, 0},
		{"Valid", args{obj: revisionExample("1")}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetRevision(tt.args.obj); got != tt.want {
				t.Errorf("GetRevision() = %v, want %v", got, tt.want)
			}
		})
	}
}

func revisionExample(value string) *unstructured.Unstructured {
	pod := test.NewPod()
	pod.SetAnnotations(map[string]string{"deployment.kubernetes.io/revision": value})
	return pod
}
