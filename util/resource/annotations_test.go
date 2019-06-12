package resource

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/test"
)

func TestHasAnnotationOption(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
		key string
		val string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Nil", args{test.NewPod(), "foo", "bar"}, false},
		{"Empty", args{example(""), "foo", "bar"}, false},
		{"Single", args{example("bar"), "foo", "bar"}, true},
		{"Double", args{example("bar,baz"), "foo", "baz"}, true},
		{"Spaces", args{example("bar "), "foo", "bar"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasAnnotationOption(tt.args.obj, tt.args.key, tt.args.val); got != tt.want {
				t.Errorf("HasAnnotationOption() = %v, want %v", got, tt.want)
			}
		})
	}
}

func example(val string) *unstructured.Unstructured {
	obj := test.NewPod()
	obj.SetAnnotations(map[string]string{"foo": val})
	return obj
}
