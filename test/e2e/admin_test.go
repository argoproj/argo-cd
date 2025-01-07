package e2e

import (
	"context"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/admin"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/admin/utils"
	appfixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
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
		AndCLIOutput(func(output string, err error) {
			require.NoError(t, err, "import finished with error")
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Get(context.Background(), "exported-app1", v1.GetOptions{})
			require.NoError(t, err, "failed getting test namespace application after import")
			_, err = fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.AppNamespace()).Get(context.Background(), "exported-app-other-namespace", v1.GetOptions{})
			require.NoError(t, err, "failed getting app namespace application after import")
		})
}
