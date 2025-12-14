package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// TestApplicationDestinationValidationBothServerAndName verifies that the CRD validation
// rejects an Application with both server and name set in the destination
func TestApplicationDestinationValidationBothServerAndName(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			// Set both server and name - this should be rejected by CEL validation
			app.Spec.Destination.Server = KubernetesInternalAPIServerAddr
			app.Spec.Destination.Name = "in-cluster"
		}).
		Then().
		Expect(Error("", "can't have both name and server defined"))
}

// TestApplicationDestinationValidationValidServerOnly verifies that an Application
// with only server set is accepted
func TestApplicationDestinationValidationValidServerOnly(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Only server is set - this should be valid
			app.Spec.Destination.Server = KubernetesInternalAPIServerAddr
			app.Spec.Destination.Name = ""
		}).
		Then().
		Expect(Success(""))
}

// TestApplicationValidationBothSourceAndSources verifies that the CRD validation
// rejects an Application with both source and sources set
func TestApplicationValidationBothSourceAndSources(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			// Set both source and sources - this should be rejected by CEL validation
			app.Spec.Source = &ApplicationSource{
				RepoURL: RepoURL(RepoURLTypeFile),
				Path:    guestbookPath,
			}
			app.Spec.Sources = ApplicationSources{
				{
					RepoURL: RepoURL(RepoURLTypeFile),
					Path:    "helm-guestbook",
				},
			}
		}).
		Then().
		Expect(Error("", "can't have both source and sources defined"))
}

// TestApplicationSourceValidationValidSourceOnly verifies that an Application
// with only source set is accepted
func TestApplicationSourceValidationValidSourceOnly(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Only source is set - this should be valid
			app.Spec.Source = &ApplicationSource{
				RepoURL: RepoURL(RepoURLTypeFile),
				Path:    guestbookPath,
			}
			app.Spec.Sources = nil
		}).
		Then().
		Expect(Success(""))
}

// TestApplicationSourceValidationValidSourcesOnly verifies that an Application
// with only sources set is accepted
func TestApplicationSourceValidationValidSourcesOnly(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Only sources is set - this should be valid
			app.Spec.Source = nil
			app.Spec.Sources = ApplicationSources{
				{
					RepoURL: RepoURL(RepoURLTypeFile),
					Path:    guestbookPath,
				},
			}
		}).
		Then().
		Expect(Success(""))
}

// TestApplicationSourceValidationValidMultipleSources verifies that an Application
// with multiple sources is accepted
func TestApplicationSourceValidationValidMultipleSources(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Multiple sources set - this should be valid
			app.Spec.Source = nil
			app.Spec.Sources = ApplicationSources{
				{
					RepoURL: RepoURL(RepoURLTypeFile),
					Path:    guestbookPath,
				}, {
					RepoURL: RepoURL(RepoURLTypeFile),
					Path:    "two-nice-pods",
				},
			}
		}).
		Then().
		Expect(Success(""))
}

// TestApplicationDestinationValidationUpdateToBothServerAndName verifies that updating
// an Application to have both server and name is rejected
func TestApplicationDestinationValidationUpdateToBothServerAndName(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(Success("")).
		When().
		IgnoreErrors().
		PatchApp(`[{"op": "add", "path": "/spec/destination/name", "value": "in-cluster"}]`).
		Then().
		Expect(Error("", "can't have both name and server defined"))
}

// TestApplicationDestinationValidationUpdateServerToName verifies that updating
// from server to name works correctly
func TestApplicationDestinationValidationUpdateServerToName(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(Success("")).
		When().
		PatchApp(`[
			{"op": "remove", "path": "/spec/destination/server"},
			{"op": "add", "path": "/spec/destination/name", "value": "in-cluster"}
		]`).
		Then().
		Expect(Success(""))
}

// TestApplicationSourceValidationUpdateToBothSourceAndSources verifies that updating
// an Application to have both source and sources is rejected
func TestApplicationSourceValidationUpdateToBothSourceAndSources(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(Success("")).
		When().
		IgnoreErrors().
		PatchApp(`[{"op": "add", "path": "/spec/sources", "value": [{"repoURL": "` + RepoURL(RepoURLTypeFile) + `", "path": "helm-guestbook"}]}]`).
		Then().
		Expect(Error("", "can't have both source and sources defined"))
}

// TestApplicationSourceValidationUpdateSourceToSources verifies that updating
// from source to sources works correctly
func TestApplicationSourceValidationUpdateSourceToSources(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(Success("")).
		When().
		PatchApp(`[
			{"op": "remove", "path": "/spec/source"},
			{"op": "add", "path": "/spec/sources", "value": [{"repoURL": "` + RepoURL(RepoURLTypeFile) + `", "path": "` + guestbookPath + `"}]}
		]`).
		Then().
		Expect(Success(""))
}

// TestApplicationSourceValidationUpdateSourcesToSource verifies that updating
// from sources to source works correctly
func TestApplicationSourceValidationUpdateSourcesToSource(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Start with sources
			app.Spec.Source = nil
			app.Spec.Sources = ApplicationSources{
				{
					RepoURL: RepoURL(RepoURLTypeFile),
					Path:    guestbookPath,
				},
			}
		}).
		Then().
		Expect(Success("")).
		When().
		PatchApp(`[
			{"op": "remove", "path": "/spec/sources"},
			{"op": "add", "path": "/spec/source", "value": {"repoURL": "` + RepoURL(RepoURLTypeFile) + `", "path": "` + guestbookPath + `"}}
		]`).
		Then().
		Expect(Success(""))
}
