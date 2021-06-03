package badge

import (
	"fmt"
	"image/color"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/argoproj/gitops-engine/pkg/health"
)

var (
	Blue   = color.RGBA{16, 61, 102, 255} // #103d66
	Green  = color.RGBA{11, 97, 42, 255}  // #0b612a
	Purple = color.RGBA{115, 31, 77, 255} // #731f4d
	Orange = color.RGBA{189, 115, 0, 255} // #bd7300
	Red    = color.RGBA{167, 46, 38, 255} // #a72e26
	Grey   = color.RGBA{41, 52, 61, 255}  // #29343D

	HealthStatusColors = map[health.HealthStatusCode]color.RGBA{
		health.HealthStatusDegraded:    Red,
		health.HealthStatusHealthy:     Green,
		health.HealthStatusMissing:     Purple,
		health.HealthStatusProgressing: Blue,
		health.HealthStatusSuspended:   Grey,
		health.HealthStatusUnknown:     Purple,
	}

	SyncStatusColors = map[appv1.SyncStatusCode]color.RGBA{
		appv1.SyncStatusCodeSynced:    Green,
		appv1.SyncStatusCodeOutOfSync: Orange,
		appv1.SyncStatusCodeUnknown:   Purple,
	}
)

func toRGBString(col color.RGBA) string {
	return fmt.Sprintf("rgb(%d, %d, %d)", col.R, col.G, col.B)
}
