package resource

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/utils/ptr"

	testingutils "github.com/argoproj/argo-cd/gitops-engine/pkg/utils/testing"
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

func TestGetAnnotationOptionValue(t *testing.T) {
	type args struct {
		obj *unstructured.Unstructured
		key string
		val string
	}
	tests := []struct {
		name string
		args args
		want *string
	}{
		{"Nil", args{testingutils.NewPod(), "foo", "bar"}, nil},
		{"Empty", args{example(""), "foo", "bar"}, nil},
		{"Standalone", args{example("bar"), "foo", "bar"}, nil},
		{"Single", args{example("bar=baz"), "foo", "bar"}, ptr.To("baz")},
		{"DeDup", args{example("bar=baz1,bar=baz2"), "foo", "bar"}, ptr.To("baz1")},
		{"Double", args{example("bar=qux,baz=quux"), "foo", "baz"}, ptr.To("quux")},
		{"Spaces", args{example("bar=baz "), "foo", "bar"}, ptr.To("baz")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetAnnotationOptionValue(tt.args.obj, tt.args.key, tt.args.val)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.Equal(t, *tt.want, *got)
			}
		})
	}
}

func example(val string) *unstructured.Unstructured {
	return testingutils.Annotate(testingutils.NewPod(), "foo", val)
}
