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

func WaveGroup(obj *unstructured.Unstructured) string {
	text, ok := obj.GetAnnotations()[common.AnnotationSyncWaveGroup]
	if ok {
		return text
	}
	return "Default"
}

func WaveGroupDependencies(obj *unstructured.Unstructured) []string {
	text, ok := obj.GetAnnotations()[common.AnnotationSyncWaveDependsOn]
	if ok {
		waveGroupDependencies := strings.Split(text, ",")
		if len(waveGroupDependencies) == 1 && waveGroupDependencies[0] == "" {
			return make([]string, 0)
		}

		return waveGroupDependencies
	}
	return make([]string, 0)
}
