package controller

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	. "github.com/argoproj/gitops-engine/pkg/utils/testing"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSoryBySyncWave(t *testing.T) {
	input := sliceOfObjectsWithSyncWaves([]string{"5", "4", "6", "8"})
	expectedOutput := sliceOfObjectsWithSyncWaves([]string{"8", "6", "5", "4"})

	sortBySyncWave(objects)

	assert.Equal(t, expectedOutput, input)
}

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
		if got := FilterObjectsForDeletion(in); got != need {
			t.Errorf("Received unexpected objects for deletion = %v, want %v", got, need)
		}
	}
}

func podWithSyncWave(wave string) *unstructured.Unstructured {
	pod := NewPod()
	pod.SetAnnotations(map[string]string{common.AnnotationSyncWave: wave})
	return pod
}

func sliceOfObjectsWithSyncWaves(waves []string) []*unstructured.Unstructured {
	objects := make([]*unstructured.Unstructured, len(waves))
	for _, wave := range waves {
		objects = append(objects, podWithSyncWave(wave))
	}
	return objects
}
