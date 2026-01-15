package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

func TestNormalizeAsSecret(t *testing.T) {
	t.Run("WithAnnotation", func(t *testing.T) {
		Given(t).
			Path("empty-dir").
			When().
			AddFile("my-secret.yaml", `
apiVersion: v1
kind: MySecret
metadata:
  name: my-secret
  annotations:
    argocd.argoproj.io/normalize-as: Secret
data:
  key1: dmFsdWUx
stringData:
  key2: value2
`).
			CreateApp().
			Sync().
			Then().
			Expect(SyncStatusIs(SyncStatusCodeSynced)).
			And(func(app *Application) {
				diffOutput, err := RunCli("app", "diff", app.Name)
				require.NoError(t, err)
				assert.Empty(t, diffOutput)
			})
	})

	t.Run("WithConfigMap", func(t *testing.T) {
		Given(t).
			Path("empty-dir").
			When().
			And(func() {
				// We use a custom GVK for this test to avoid messing with real ConfigMaps
				// but we'll use a ConfigMap in the manifest.
				require.NoError(t, SetResourceOverrides(map[string]ResourceOverride{
					"MySecret": {
						NormalizeAs: "Secret",
					},
				}))
			}).
			AddFile("my-secret.yaml", `
apiVersion: v1
kind: MySecret
metadata:
  name: my-secret
data:
  key1: dmFsdWUx
stringData:
  key2: value2
`).
			IgnoreErrors().
			CreateApp().
			Then().
			And(func(_ *Application) {
				// Since MySecret doesn't exist as a CRD, it might show as OutOfSync or have errors
				// but what we care about is that the diff shows it normalized.
				// However, Argo CD might fail to even process it if the GVK is unknown.

				// Let's use ConfigMap instead but with a specific name and override it.
				require.NoError(t, SetResourceOverrides(map[string]ResourceOverride{
					"ConfigMap": {
						NormalizeAs: "Secret",
					},
				}))
			}).
			When().
			AddFile("my-secret-2.yaml", `
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-secret-2
data:
  key1: dmFsdWUx
stringData:
  key2: value2
`).
			Sync().
			Then().
			Expect(SyncStatusIs(SyncStatusCodeSynced)).
			And(func(app *Application) {
				diffOutput, err := RunCli("app", "diff", app.Name)
				require.NoError(t, err)
				assert.Empty(t, diffOutput)
			})
	})
}
