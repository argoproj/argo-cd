package badge

import (
	"fmt"
	"image/color"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"github.com/argoproj/gitops-engine/pkg/health"
)

var (
	Blue   = color.RGBA{13, 173, 234, 255}  // #0dadea
	Green  = color.RGBA{24, 190, 82, 255}   // #18be52
	Purple = color.RGBA{178, 102, 255, 255} // #b266ff
	Orange = color.RGBA{244, 192, 48, 255}  // #f4c030
	Red    = color.RGBA{233, 109, 118, 255} // #e96d76
	Grey   = color.RGBA{204, 214, 221, 255} // #ccd6dd

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
