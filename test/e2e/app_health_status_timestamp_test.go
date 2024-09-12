package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/gitops-engine/pkg/health"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LastTransitionTime is updated when health state is changed.
// ObservedAt is updated periodically every 30s to reflect the freshness of state observed.
func TestAppHealthTimestamp(t *testing.T) {
	var oldLastTransitionTime, oldObservedAt metav1.Time

	Given(t).
		Timeout(120).
		Path("two-nice-pods").
		When().
		CreateApp().
		Sync().
		Wait().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			assert.False(t, app.Status.Health.LastTransitionTime.IsZero())
			assert.False(t, app.Status.Health.ObservedAt.IsZero())
			oldLastTransitionTime = *app.Status.Health.LastTransitionTime
			oldObservedAt = *app.Status.Health.ObservedAt
		}).
		When().
		And(func() {
			// Sleep for more than 30s to avoid a flaky test,
			// as the event handler might have resynced at the start of the test.
			time.Sleep(45 * time.Second)
		}).
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(app *Application) {
			// Only ObservedAt value should be updated
			assert.Equal(t, oldLastTransitionTime, *app.Status.Health.LastTransitionTime)
			assert.NotEqual(t, oldObservedAt, *app.Status.Health.ObservedAt)
		})
}
