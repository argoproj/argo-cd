package syncwindow

import (
	"errors"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// EffectiveWindows holds the sync windows that apply to an application, split into two
// independently enforced tiers. Neither tier can override the other.
type EffectiveWindows struct {
	// Project holds the project's inline windows and resolved project refs that match the app.
	Project *v1alpha1.SyncWindows
	// App holds the windows resolved from the app's own refs, which apply unconditionally.
	App *v1alpha1.SyncWindows
	// ProjectRefsErr and AppRefsErr report refs that could not be resolved and were skipped.
	ProjectRefsErr error
	AppRefsErr     error
}

// NewEffectiveWindows builds the two tiers from already-resolved project and app ref windows.
func NewEffectiveWindows(app *v1alpha1.Application, proj *v1alpha1.AppProject, projRefWindows, appRefWindows v1alpha1.SyncWindows) EffectiveWindows {
	projWindows := make(v1alpha1.SyncWindows, 0, len(proj.Spec.SyncWindows)+len(projRefWindows))
	projWindows = append(projWindows, proj.Spec.SyncWindows...)
	projWindows = append(projWindows, projRefWindows...)

	ew := EffectiveWindows{Project: projWindows.Matches(app)}
	if len(appRefWindows) > 0 {
		ew.App = &appRefWindows
	}
	return ew
}

// Resolve resolves the project's and the app's SyncWindow refs and returns the effective windows.
// Unresolvable refs are skipped and reported in ProjectRefsErr / AppRefsErr.
func (r *Resolver) Resolve(app *v1alpha1.Application, proj *v1alpha1.AppProject) EffectiveWindows {
	var projRefWindows, appRefWindows v1alpha1.SyncWindows
	var projErr, appErr error
	if len(proj.Spec.SyncWindowRefs) > 0 {
		projRefWindows, projErr = r.ResolveProjectRefs(proj.Spec.SyncWindowRefs)
	}
	if len(app.Spec.SyncWindowRefs) > 0 {
		appRefWindows, appErr = r.ResolveAppRefs(app.Spec.SyncWindowRefs, app.Namespace)
	}
	ew := NewEffectiveWindows(app, proj, projRefWindows, appRefWindows)
	ew.ProjectRefsErr = projErr
	ew.AppRefsErr = appErr
	return ew
}

// CanSync reports whether both tiers permit a sync. isManual and operationStartTime follow
// the same semantics as SyncWindows.CanSync.
func (ew EffectiveWindows) CanSync(isManual bool, operationStartTime *time.Time) (bool, error) {
	if ok, err := ew.Project.CanSync(isManual, operationStartTime); err != nil || !ok {
		return false, err
	}
	return ew.App.CanSync(isManual, operationStartTime)
}

// All returns the windows of both tiers merged, for display only.
func (ew EffectiveWindows) All() v1alpha1.SyncWindows {
	var all v1alpha1.SyncWindows
	if ew.Project != nil {
		all = append(all, *ew.Project...)
	}
	if ew.App != nil {
		all = append(all, *ew.App...)
	}
	return all
}

// RefsErr returns the combined error of all unresolvable refs, or nil.
func (ew EffectiveWindows) RefsErr() error {
	return errors.Join(ew.ProjectRefsErr, ew.AppRefsErr)
}
