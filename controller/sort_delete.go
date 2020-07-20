package controller

import (
	"sort"

	"github.com/argoproj/gitops-engine/pkg/sync/syncwaves"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type syncWaveSorter []*unstructured.Unstructured

func (s syncWaveSorter) Len() int {
	return len(s)
}

func (s syncWaveSorter) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s syncWaveSorter) Less(i, j int) bool {
	return syncwaves.Wave(s[i]) < syncwaves.Wave(s[j])
}

func sortBySyncWave(objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	sort.Sort(sort.Reverse(syncWaveSorter(objs)))
	return []*unstructured.Unstructured(objs)
}

func FilterObjectsForDeletion(objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	if len(objs) <= 1 {
		return objs
	}
	sortBySyncWave(objs)
	currentSyncWave := syncwaves.Wave(objs[0])
	sortedObjs := make([]*unstructured.Unstructured, 0)
	for _, obj := range objs {
		if syncwaves.Wave(obj) != currentSyncWave {
			break
		}
		sortedObjs = append(sortedObjs, obj)
	}
	return sortedObjs
}
