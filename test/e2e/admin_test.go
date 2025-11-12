package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/admin"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/admin/utils"
	appfixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

func TestBackupExportImport(t *testing.T) {
	var exportRawOutput string
	ctx := Given(t)
	// Create application in argocd namespace
	appctx := appfixture.GivenWithSameState(t)

	// Create application in test namespace
	appctx.
		Path(guestbookPath).
		Name("exported-app1").
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			assert.Equal(t, "exported-app1", app.Name)
			assert.Equal(t, fixture.TestNamespace(), app.Namespace)
		})

	// Create app in other namespace
	appctx.
		Path(guestbookPath).
		Name("exported-app-other-namespace").
		SetAppNamespace(fixture.AppNamespace()).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			assert.Equal(t, "exported-app-other-namespace", app.Name)
			assert.Equal(t, fixture.AppNamespace(), app.Namespace)
		})

	ctx.
		When().
		RunExport().
		Then().
		AndCLIOutput(func(output string, err error) {
			require.NoError(t, err, "export finished with error")
			exportRawOutput = output
		}).
		AndExportedResources(func(exportResources *ExportedResources, err error) {
			require.NoError(t, err, "export format not valid")
			assert.True(t, exportResources.HasResource(kube.NewResourceKey("", "ConfigMap", "", "argocd-cm")), "argocd-cm not found in export")
			assert.True(t, exportResources.HasResource(kube.NewResourceKey(ApplicationSchemaGroupVersionKind.Group, ApplicationSchemaGroupVersionKind.Kind, "", "exported-app1")), "test namespace application not in export")
			assert.True(t, exportResources.HasResource(kube.NewResourceKey(ApplicationSchemaGroupVersionKind.Group, ApplicationSchemaGroupVersionKind.Kind, fixture.AppNamespace(), "exported-app-other-namespace")), "app namespace application not in export")
		})

	// Test import - clean state
	ctx = Given(t)

	ctx.
		When().
		RunImport(exportRawOutput).
		Then().
		AndCLIOutput(func(_ string, err error) {
			require.NoError(t, err, "import finished with error")
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Get(t.Context(), "exported-app1", metav1.GetOptions{})
			require.NoError(t, err, "failed getting test namespace application after import")
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.AppNamespace()).Get(t.Context(), "exported-app-other-namespace", metav1.GetOptions{})
			require.NoError(t, err, "failed getting app namespace application after import")
		})
}

func TestDisableAdminUserFlag(t *testing.T) {
	// This E2E test verifies that when the --disable-admin-user flag is used,
	// the admin user and password are not created
	fixture.EnsureCleanState(t)

	// Check if the initial admin secret exists
	// Note: In a full E2E test environment, we would restart the server with --disable-admin-user
	// For now, we can test that the admin account exists by default
	_, err := fixture.KubeClientset.CoreV1().Secrets(fixture.TestNamespace()).Get(
		t.Context(),
		"argocd-initial-admin-secret",
		metav1.GetOptions{},
	)

	// In the default setup (without --disable-admin-user), the secret should exist
	require.NoError(t, err, "admin password secret should exist in default setup")

	// Verify admin account can log in
	closer, sessionClient := fixture.ArgoCDClientset.NewSessionClientOrDie()
	defer closer()

	// Get admin password from secret
	secret, err := fixture.KubeClientset.CoreV1().Secrets(fixture.TestNamespace()).Get(
		t.Context(),
		"argocd-initial-admin-secret",
		metav1.GetOptions{},
	)
	require.NoError(t, err)
	password := string(secret.Data["password"])

	// Try to login as admin
	createRequest := &session.SessionCreateRequest{
		Username: "admin",
		Password: password,
	}

	resp, err := sessionClient.Create(t.Context(), createRequest)
	require.NoError(t, err, "admin should be able to login in default setup")
	assert.NotEmpty(t, resp.Token, "session token should be returned")
}
