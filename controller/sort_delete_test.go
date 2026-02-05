package controller

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	. "github.com/argoproj/gitops-engine/pkg/utils/testing"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFilterObjectsForDeletion(t *testing.T) {
	tests := []struct {
		input []string
		want  []string
	}{
		{[]string{"1", "5", "7", "7", "4"}, []string{"7", "7"}},
		{[]string{"1", "5", "2", "2", "4"}, []string{"5"}},
		{[]string{"1"}, []string{"1"}},
		{[]string{}, []string{}},
	}
	for _, tt := range tests {
		in := sliceOfObjectsWithSyncWaves(tt.input)
		need := sliceOfObjectsWithSyncWaves(tt.want)
		got := FilterObjectsForDeletion(in)
		assert.True(t, reflect.DeepEqual(got, need), "Received unexpected objects for deletion")
	}
}

func podWithSyncWave(wave string) *unstructured.Unstructured {
	return Annotate(NewPod(), common.AnnotationSyncWave, wave)
}

func sliceOfObjectsWithSyncWaves(waves []string) []*unstructured.Unstructured {
	objects := make([]*unstructured.Unstructured, 0)
	for _, wave := range waves {
		objects = append(objects, podWithSyncWave(wave))
	}
	return objects
}
