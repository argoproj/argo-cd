package e2e

import (
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/diff"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/account"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	projectFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/project"
)

// waitForRBACPermission polls "argocd account can-i" until the RBAC policy
// propagates to the server, ensuring SetPermissions has taken effect before
// the actual API call. Without this, the informer update race causes flaky failures.
func waitForRBACPermission(t *testing.T, action, resource, scope string, expectAllow bool) {
	t.Helper()
	timeout := 15 * time.Second
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output, _ := fixture.RunCli("account", "can-i", action, resource, scope)
		if expectAllow && strings.Contains(output, "yes") {
			return
		}
		if !expectAllow && strings.Contains(output, "no") {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("RBAC policy for %s/%s/%s (expectAllow=%v) did not propagate within %v", action, resource, scope, expectAllow, timeout)
}

func TestRollbackWithExplicitRollbackPermission(t *testing.T) {
	appCtx := Given(t)

	var appName string
	GivenWithSameState(appCtx).
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			appName = app.Name
			appWithHistory := app.DeepCopy()
			appWithHistory.Status.History = []RevisionHistory{
				{
					ID:         1,
					Revision:   app.Status.Sync.Revision,
					DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-1 * time.Minute)},
					Source:     app.Spec.GetSource(),
				},
				{
					ID:         2,
					Revision:   "test-revision-2",
					DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-2 * time.Minute)},
					Source:     app.Spec.GetSource(),
				},
			}
			patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &Application{})
			require.NoError(t, err)
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Patch(
				t.Context(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			require.NoError(t, err)
		})

	accountFixture.GivenWithSameState(appCtx).
		Name("rollback-user").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{Resource: "applications", Action: "get", Scope: "*"},
			{Resource: "applications", Action: "rollback", Scope: "*"},
			{Resource: "projects", Action: "get", Scope: "*"},
		}, "rollback-user")

	require.NoError(t, fixture.SetParamInSettingConfigMap("server.rbac.rollback.enforce.enable", "true"))
	waitForRBACPermission(t, "rollback", "applications", "*", true)
	_, err := fixture.RunCli("app", "rollback", appName, "1")
	assert.NoError(t, err, "user with rollback permission should be able to rollback")
}

// TestRollbackWithoutExplicitRollbackPermission verifies that users with neither rollback
// nor sync permission are denied.
func TestRollbackWithoutExplicitRollbackPermission(t *testing.T) {
	appCtx := Given(t)

	var appName string
	GivenWithSameState(appCtx).
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			appName = app.Name
			appWithHistory := app.DeepCopy()
			appWithHistory.Status.History = []RevisionHistory{
				{
					ID:         1,
					Revision:   app.Status.Sync.Revision,
					DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-1 * time.Minute)},
					Source:     app.Spec.GetSource(),
				},
			}
			patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &Application{})
			require.NoError(t, err)
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Patch(
				t.Context(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			require.NoError(t, err)
		})

	accountFixture.GivenWithSameState(appCtx).
		Name("readonly-user").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{Resource: "applications", Action: "get", Scope: "*"},
			{Resource: "projects", Action: "get", Scope: "*"},
		}, "readonly-user")

	require.NoError(t, fixture.SetParamInSettingConfigMap("server.rbac.rollback.enforce.enable", "true"))
	waitForRBACPermission(t, "rollback", "applications", "*", false)
	_, err := fixture.RunCli("app", "rollback", appName, "1")
	require.Error(t, err, "user without rollback or sync permission should not be able to rollback")
	assert.ErrorContains(t, err, "permission denied")
}

// TestRollbackWithSyncPermission tests that users with sync permission can still rollback
// for backwards compatibility
func TestRollbackWithSyncPermission(t *testing.T) {
	appCtx := Given(t)

	var appName string
	GivenWithSameState(appCtx).
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			appName = app.Name
			appWithHistory := app.DeepCopy()
			appWithHistory.Status.History = []RevisionHistory{
				{
					ID:         1,
					Revision:   app.Status.Sync.Revision,
					DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-1 * time.Minute)},
					Source:     app.Spec.GetSource(),
				},
			}
			patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &Application{})
			require.NoError(t, err)
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Patch(
				t.Context(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			require.NoError(t, err)
		})

	accountFixture.GivenWithSameState(appCtx).
		Name("sync-user").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{Resource: "applications", Action: "get", Scope: "*"},
			{Resource: "applications", Action: "sync", Scope: "*"},
			{Resource: "projects", Action: "get", Scope: "*"},
		}, "sync-user")

	waitForRBACPermission(t, "sync", "applications", "*", true)
	_, err := fixture.RunCli("app", "rollback", appName, "1")
	assert.NoError(t, err, "user with sync permission should be able to rollback for backwards compatibility")
}

// TestRollbackWithProjectScopedRollbackPermission tests that rollback permission can be scoped to specific projects
func TestRollbackWithProjectScopedRollbackPermission(t *testing.T) {
	appCtx := Given(t)
	projCtx := projectFixture.GivenWithSameState(appCtx)
	projCtx.Destination("*,*").
		When().
		Create().
		AddSource("*")

	projName := projCtx.GetName()

	var appName string
	GivenWithSameState(appCtx).
		Path("guestbook").
		Project(projName).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			appName = app.Name
			appWithHistory := app.DeepCopy()
			appWithHistory.Status.History = []RevisionHistory{
				{
					ID:         1,
					Revision:   app.Status.Sync.Revision,
					DeployedAt: metav1.Time{Time: metav1.Now().UTC().Add(-1 * time.Minute)},
					Source:     app.Spec.GetSource(),
				},
			}
			patch, _, err := diff.CreateTwoWayMergePatch(app, appWithHistory, &Application{})
			require.NoError(t, err)
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Patch(
				t.Context(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
			require.NoError(t, err)
		})

	accountFixture.GivenWithSameState(appCtx).
		Name("scoped-rollback-user").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{Resource: "applications", Action: "get", Scope: projName + "/*"},
			{Resource: "applications", Action: "rollback", Scope: projName + "/*"},
			{Resource: "projects", Action: "get", Scope: "*"},
		}, "scoped-rollback-user")

	require.NoError(t, fixture.SetParamInSettingConfigMap("server.rbac.rollback.enforce.enable", "true"))
	waitForRBACPermission(t, "rollback", "applications", projName+"/*", true)
	_, err := fixture.RunCli("app", "rollback", appName, "1")
	assert.NoError(t, err, "user with rollback permission scoped to default project should be able to rollback")
}
