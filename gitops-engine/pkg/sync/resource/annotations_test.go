package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
)

func TestHasAnnotationOption(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
		key string
		val string
	}
	tests := []struct {
		name     string
		args     args
		wantVals []string
		want     bool
	}{
		{"Nil", args{testingutils.NewPod(), "foo", "bar"}, nil, false},
		{"Empty", args{example(""), "foo", "bar"}, nil, false},
		{"Single", args{example("bar"), "foo", "bar"}, []string{"bar"}, true},
		{"DeDup", args{example("bar,bar"), "foo", "bar"}, []string{"bar"}, true},
		{"Double", args{example("bar,baz"), "foo", "baz"}, []string{"bar", "baz"}, true},
		{"Spaces", args{example("bar "), "foo", "bar"}, []string{"bar"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.wantVals, GetAnnotationCSVs(tt.args.obj, tt.args.key))
			assert.Equal(t, tt.want, HasAnnotationOption(tt.args.obj, tt.args.key, tt.args.val))
		})
	}
}

func example(val string) *unstructured.Unstructured {
	return testingutils.Annotate(testingutils.NewPod(), "foo", val)
}
