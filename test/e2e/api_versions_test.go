package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

func TestAppSyncWrongVersion(t *testing.T) {
	// Make sure the error messages are good when there are group or version mismatches between CRDs and resources.
	ctx := Given(t)
	ctx.
		Path("crd-version-differences").
		When().
		CreateApp().
		// Install CRD and one instance of it on v1alpha1
		AppSet("--directory-include", "crd-v1alpha1.yaml").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		AppSet("--directory-include", "crd-v1alpha2-instance.yaml").
		IgnoreErrors(). // Ignore errors because we are testing the error message.
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		DoNotIgnoreErrors().
		Get().
		Then().
		// Technically it's a "success" because we're just doing a "get," but the get output contains the error message.
		Expect(SuccessRegex(`The Kubernetes API could not find version "v1alpha2" of argoproj\.io/Fake for requested resource [a-z0-9-]+/fake-crd-instance\. Version "v1alpha1" of argoproj\.io/Fake is installed on the destination cluster\.`)).
		When().
		AppSet("--directory-include", "crd-wronggroup-instance.yaml", "--directory-exclude", "crd-v1alpha2-instance.yaml").
		IgnoreErrors(). // Ignore errors because we are testing the error message.
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		DoNotIgnoreErrors().
		Get().
		Then().
		Expect(SuccessRegex(`The Kubernetes API could not find version "v1alpha1" of wrong\.group/Fake for requested resource [a-z0-9-]+/fake-crd-instance-wronggroup\. Version "v1alpha1" of argoproj\.io/Fake is installed on the destination cluster\.`)).
		When().
		AppSet("--directory-include", "crd-does-not-exist-instance.yaml", "--directory-exclude", "crd-wronggroup-instance.yaml").
		IgnoreErrors(). // Ignore errors because we are testing the error message.
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		DoNotIgnoreErrors().
		Get().
		Then().
		// Not the best error message, but good enough.
		Expect(Success(`DoesNotExist.argoproj.io "" not found`))
}
