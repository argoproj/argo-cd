package syncwaves

import (
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/gitops-engine/pkg/sync/common"
	helmhook "github.com/argoproj/gitops-engine/pkg/sync/hook/helm"
)

func Wave(obj *unstructured.Unstructured) int {
	text, ok := obj.GetAnnotations()[common.AnnotationSyncWave]
	if ok {
		val, err := strconv.Atoi(text)
		if err == nil {
			return val
		}
	}
	return helmhook.Weight(obj)
}

func WaveGroup(obj *unstructured.Unstructured) int {
	text, ok := obj.GetAnnotations()[common.AnnotationSyncWaveGroup]
	if ok {
		val, err := strconv.Atoi(text)
		if err == nil {
			return val
		}
	}
	return 0
}

func WaveGroupDependencies(obj *unstructured.Unstructured) []int {
	text, ok := obj.GetAnnotations()[common.AnnotationSyncWaveGroupDependencies]
	if ok {
		waveGroup := WaveGroup(obj)
		stringWaveGroupDependencies := strings.Split(text, ",")

		waveGroupDependencies := make([]int, 0)
		for _, t := range stringWaveGroupDependencies {
			waveGroupDependency, err := strconv.Atoi(t)
			if err == nil {
				if waveGroupDependency < waveGroup {
					waveGroupDependencies = append(waveGroupDependencies, waveGroupDependency)
				}
			}
		}
		return waveGroupDependencies
	}
	return make([]int, 0)
}
