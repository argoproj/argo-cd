package controller

import (
	"reflect"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	. "github.com/argoproj/gitops-engine/pkg/utils/testing"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFilterObjectsForDeletionForWave(t *testing.T) {
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
		if got := FilterObjectsForDeletion(in); !reflect.DeepEqual(got, need) {
			t.Errorf("Received unexpected objects for deletion = %v, want %v", got, need)
		}
	}
}

func TestFilterObjectsForDeletionForKind(t *testing.T) {
	tests := []struct {
		input []*unstructured.Unstructured
		want  []*unstructured.Unstructured
	}{
		{[]*unstructured.Unstructured{NewPod(), NewService()}, []*unstructured.Unstructured{NewService()}},
		{[]*unstructured.Unstructured{NewPod()}, []*unstructured.Unstructured{NewPod()}},
		{[]*unstructured.Unstructured{}, []*unstructured.Unstructured{}},
	}
	for _, tt := range tests {
		if got := FilterObjectsForDeletion(tt.input); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Received unexpected objects for deletion = %v, want %v", got, tt.want)
		}
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
