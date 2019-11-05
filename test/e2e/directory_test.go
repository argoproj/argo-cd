package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// by default we do not recurse, so zero resources
func TestDirectory(t *testing.T) {
	Given(t).
		Path("directory").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(0))
}

// if we recurse, we should see the map
func TestDirectoryRecurse(t *testing.T) {
	Given(t).
		Path("directory").
		When().
		Create("--directory-recurse").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(1))
}

// if we recurse, we should see the map, unless we ignore it
func TestDirectoryRecurseIgnore(t *testing.T) {
	Given(t).
		Path("directory").
		When().
		Create("--directory-recurse", "--directory-ignore", "*/my-map.yaml").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(0))
}
