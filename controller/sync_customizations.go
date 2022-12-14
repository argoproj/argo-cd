package controller

import (
	"encoding/json"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	kubeutil "github.com/argoproj/gitops-engine/pkg/utils/kube"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strings"
)

// for some resource kubectl apply returns 'configured' in message but in fact it was unchanged
// this func intented to fix this behaviour
func (m *appStateManager) FixWrongKubectlMessage(resState []common.ResourceSyncResult, state *v1alpha1.OperationState, compareResult *comparisonResult) {
	diffResultsMap := groupDiffResults(compareResult.diffResultList)

	for _, res := range resState {
		message := res.Message
		resDiff := diffResultsMap[res.ResourceKey]

		configured := "configured"
		unchanged := "unchanged"

		if message != "" {
			if strings.HasSuffix(message, configured) && !resDiff.Modified {
				message = strings.TrimSuffix(message, configured)
				message += unchanged
			}
		}
		state.SyncResult.Resources = append(state.SyncResult.Resources, &v1alpha1.ResourceResult{
			HookType:  res.HookType,
			Group:     res.ResourceKey.Group,
			Kind:      res.ResourceKey.Kind,
			Namespace: res.ResourceKey.Namespace,
			Name:      res.ResourceKey.Name,
			Version:   res.Version,
			SyncPhase: res.SyncPhase,
			HookPhase: res.HookPhase,
			Status:    res.Status,
			Message:   message,
		})
	}
}

// generates a map of resource and its modification result based on diffResultList
func groupDiffResults(diffResultList *diff.DiffResultList) map[kubeutil.ResourceKey]diff.DiffResult {
	modifiedResources := make(map[kube.ResourceKey]diff.DiffResult)
	for _, res := range diffResultList.Diffs {
		var obj unstructured.Unstructured
		var err error
		if string(res.NormalizedLive) != "null" {
			err = json.Unmarshal(res.NormalizedLive, &obj)
		} else {
			err = json.Unmarshal(res.PredictedLive, &obj)
		}
		if err != nil {
			continue
		}
		modifiedResources[kube.GetResourceKey(&obj)] = res
	}
	return modifiedResources
}
