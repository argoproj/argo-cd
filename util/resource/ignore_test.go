package resource

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/test"
)

func TestIgnore(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"DoNotIgnoreEmpty", args{test.NewPod()}, false},
		{"IgnoreHelmHook", args{test.NewHelmHook("pre-install")}, true},
		{"DoNotIgnoreHelmCrdInstall", args{test.NewHelmHook("crd-install")}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Ignore(tt.args.obj); got != tt.want {
				t.Errorf("Ignore() = %v, want %v", got, tt.want)
			}
		})
	}
}
