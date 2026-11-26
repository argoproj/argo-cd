package syncwindow

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// activeSchedule fires every minute, so a window with a duration of at least a minute is open now.
func activeSchedule() string { return "* * * * *" }

// inactiveSchedule fires 10 hours from now, so a short window is closed now.
func inactiveSchedule() string {
	h, _, _ := time.Now().UTC().Add(10 * time.Hour).Clock()
	return fmt.Sprintf("0 %d * * *", h)
}

// makeSyncWindow builds an InlineSyncWindow with wildcard filters so it matches any app.
func makeSyncWindow(kind, schedule, duration string) *v1alpha1.InlineSyncWindow {
	return &v1alpha1.InlineSyncWindow{
		Kind:         kind,
		Schedule:     schedule,
		Duration:     duration,
		Applications: []string{"*"},
		Namespaces:   []string{"*"},
		Clusters:     []string{"*"},
	}
}

func TestEffectiveWindowsCanSync(t *testing.T) {
	t.Parallel()

	app := &v1alpha1.Application{
		Name: "my-app", Namespace: "default",
		Spec: v1alpha1.ApplicationSpec{
			Destination: v1alpha1.ApplicationDestination{
				Namespace: "default",
				Server:    "https://kubernetes.default.svc",
			},
		},
	}

	activeAllow := func() *v1alpha1.InlineSyncWindow { return makeSyncWindow("allow", activeSchedule(), "2h") }
	activeDeny := func() *v1alpha1.InlineSyncWindow { return makeSyncWindow("deny", activeSchedule(), "2h") }
	inactiveAllow := func() *v1alpha1.InlineSyncWindow { return makeSyncWindow("allow", inactiveSchedule(), "2h") }
	inactiveDeny := func() *v1alpha1.InlineSyncWindow { return makeSyncWindow("deny", inactiveSchedule(), "2h") }

	tests := []struct {
		name           string
		projInline     v1alpha1.SyncWindows
		projRefWindows v1alpha1.SyncWindows
		appRefWindows  v1alpha1.SyncWindows
		expected       bool
	}{
		{name: "both tiers allow", projInline: v1alpha1.SyncWindows{activeAllow()}, appRefWindows: v1alpha1.SyncWindows{activeAllow()}, expected: true},
		{name: "project open, app closed", projInline: v1alpha1.SyncWindows{activeAllow()}, appRefWindows: v1alpha1.SyncWindows{inactiveAllow()}, expected: false},
		{name: "project closed, app open", projInline: v1alpha1.SyncWindows{inactiveAllow()}, appRefWindows: v1alpha1.SyncWindows{activeAllow()}, expected: false},
		{name: "app allow cannot override project deny", projInline: v1alpha1.SyncWindows{activeDeny()}, appRefWindows: v1alpha1.SyncWindows{activeAllow()}, expected: false},
		{name: "project deny closed, app allow open", projInline: v1alpha1.SyncWindows{inactiveDeny()}, appRefWindows: v1alpha1.SyncWindows{activeAllow()}, expected: true},
		{name: "only app allow, open", appRefWindows: v1alpha1.SyncWindows{activeAllow()}, expected: true},
		{name: "only app allow, closed", appRefWindows: v1alpha1.SyncWindows{inactiveAllow()}, expected: false},
		{name: "project ref deny is in project tier", projInline: v1alpha1.SyncWindows{activeAllow()}, projRefWindows: v1alpha1.SyncWindows{activeDeny()}, expected: false},
		{name: "both tiers empty", expected: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			proj := &v1alpha1.AppProject{Spec: v1alpha1.AppProjectSpec{SyncWindows: tt.projInline}}
			canSync, err := NewEffectiveWindows(app, proj, tt.projRefWindows, tt.appRefWindows).CanSync(false, nil)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, canSync)
		})
	}
}

func TestEffectiveWindowsProjectRefFilters(t *testing.T) {
	app := &v1alpha1.Application{Name: "my-app", Spec: v1alpha1.ApplicationSpec{Destination: v1alpha1.ApplicationDestination{Namespace: "default"}}}
	proj := &v1alpha1.AppProject{}
	other := makeSyncWindow("deny", activeSchedule(), "2h")
	other.Applications, other.Namespaces, other.Clusters = []string{"other-app"}, nil, nil

	ew := NewEffectiveWindows(app, proj, v1alpha1.SyncWindows{other}, nil)

	assert.Nil(t, ew.Project, "project ref windows must be filtered by app")
	canSync, err := ew.CanSync(false, nil)
	require.NoError(t, err)
	assert.True(t, canSync)
}

func TestResolve(t *testing.T) {
	sw := newSyncWindow("freeze", "argocd", nil, []v1alpha1.SyncWindowDefinition{
		{Kind: "deny", Schedule: activeSchedule(), Duration: "2h"},
	})
	resolver := NewResolver(newFakeLister(sw), "argocd")
	app := &v1alpha1.Application{
		Name: "my-app", Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{SyncWindowRefs: []v1alpha1.SyncWindowRef{{Name: "freeze"}}},
	}

	t.Run("resolved app ref denies sync", func(t *testing.T) {
		ew := resolver.Resolve(app, &v1alpha1.AppProject{})
		require.NoError(t, ew.RefsErr())
		require.NotNil(t, ew.App)
		assert.Len(t, ew.All(), 1)
		canSync, err := ew.CanSync(false, nil)
		require.NoError(t, err)
		assert.False(t, canSync)
	})

	t.Run("unresolved refs are reported per tier", func(t *testing.T) {
		proj := &v1alpha1.AppProject{Spec: v1alpha1.AppProjectSpec{
			SyncWindowRefs: []v1alpha1.SyncWindowProjectRef{{Ref: v1alpha1.SyncWindowRef{Name: "missing"}}},
		}}
		missingApp := app.DeepCopy()
		missingApp.Spec.SyncWindowRefs = []v1alpha1.SyncWindowRef{{Name: "missing"}}

		ew := resolver.Resolve(missingApp, proj)
		require.Error(t, ew.ProjectRefsErr)
		require.Error(t, ew.AppRefsErr)
		require.Error(t, ew.RefsErr())
		assert.Empty(t, ew.All())
	})
}
