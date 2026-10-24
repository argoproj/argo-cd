package e2e

import (
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

type Params struct {
	ObservedGeneration int64
}

func GenerateSuccessfulApplicationSetConditions(p Params) []v1alpha1.ApplicationSetCondition {
	return []v1alpha1.ApplicationSetCondition{
		{
			Type:               v1alpha1.ApplicationSetConditionErrorOccurred,
			Status:             v1alpha1.ApplicationSetConditionStatusFalse,
			Message:            "All applications have been generated successfully",
			Reason:             v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
			ObservedGeneration: p.ObservedGeneration,
		},
		{
			Type:               v1alpha1.ApplicationSetConditionParametersGenerated,
			Status:             v1alpha1.ApplicationSetConditionStatusTrue,
			Message:            "Successfully generated parameters for all Applications",
			Reason:             v1alpha1.ApplicationSetReasonParametersGenerated,
			ObservedGeneration: p.ObservedGeneration,
		},
		{
			Type:               v1alpha1.ApplicationSetConditionResourcesUpToDate,
			Status:             v1alpha1.ApplicationSetConditionStatusTrue,
			Message:            "All applications have been generated successfully",
			Reason:             v1alpha1.ApplicationSetReasonApplicationSetUpToDate,
			ObservedGeneration: p.ObservedGeneration,
		},
	}
}
