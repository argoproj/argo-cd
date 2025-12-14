package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// TestApplicationDestinationValidation_BothServerAndName verifies that the CRD validation
// rejects an Application with both server and name set in the destination
func TestApplicationDestinationValidation_BothServerAndName(t *testing.T) {
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
		Expect(Error("", "mutually exclusive"))
}

// TestApplicationDestinationValidation_NeitherServerNorName verifies that an Application
// with neither server nor name is allowed (for ApplicationSet templates)
func TestApplicationDestinationValidation_NeitherServerNorName(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Clear both server and name - this is valid for templates
			app.Spec.Destination.Server = ""
			app.Spec.Destination.Name = ""
		}).
		Then().
		Expect(Success(""))
}

// TestApplicationDestinationValidation_ValidServerOnly verifies that an Application
// with only server set is accepted
func TestApplicationDestinationValidation_ValidServerOnly(t *testing.T) {
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

// TestApplicationSourceValidation_BothSourceAndSources verifies that the CRD validation
// rejects an Application with both source and sources set
func TestApplicationSourceValidation_BothSourceAndSources(t *testing.T) {
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
		Expect(Error("", "mutually exclusive"))
}

// TestApplicationSourceValidation_NeitherSourceNorSources verifies that an Application
// with neither source nor sources is allowed (for ApplicationSet templates)
func TestApplicationSourceValidation_NeitherSourceNorSources(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Clear both source and sources - this is valid for templates
			app.Spec.Source = nil
			app.Spec.Sources = nil
		}).
		Then().
		Expect(Success(""))
}

// TestApplicationSourceValidation_EmptySourcesArray verifies that an Application
// with an empty sources array is allowed (for ApplicationSet templates)
func TestApplicationSourceValidation_EmptySourcesArray(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			// Set sources to empty array - this is valid for templates
			app.Spec.Source = nil
			app.Spec.Sources = ApplicationSources{}
		}).
		Then().
		Expect(Success(""))
}

// TestApplicationSourceValidation_ValidSourceOnly verifies that an Application
// with only source set is accepted
func TestApplicationSourceValidation_ValidSourceOnly(t *testing.T) {
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

// TestApplicationSourceValidation_ValidSourcesOnly verifies that an Application
// with only sources set is accepted
func TestApplicationSourceValidation_ValidSourcesOnly(t *testing.T) {
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

// TestApplicationSourceValidation_ValidMultipleSources verifies that an Application
// with multiple sources is accepted
func TestApplicationSourceValidation_ValidMultipleSources(t *testing.T) {
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
				},
				{
					RepoURL: RepoURL(RepoURLTypeFile),
					Path:    "kustomize-guestbook",
				},
			}
		}).
		Then().
		Expect(Success(""))
}
