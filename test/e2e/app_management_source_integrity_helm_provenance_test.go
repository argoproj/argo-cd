package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

const (
	helmProvenanceRepoURL    = "https://prometheus-community.github.io/helm-charts"
	helmProvenanceKeyURL     = "https://prometheus-community.github.io/helm-charts/pubkey.gpg"
	helmProvenanceSigningKey = "27252B168248743B"
	helmProvenanceWrongKey   = "0000000000000000"
	helmProvenanceChart      = "alertmanager"
	helmProvenanceChartV     = "1.33.1"
	helmProvenancePassName   = "helm-prov-pass"
	helmProvenanceFailName   = "helm-prov-fail"
	helmProvenancePassRel    = "helm-prov-pass"
	helmProvenanceFailRel    = "helm-prov-fail"
)

func TestHelmSourceIntegrityProvenancePassesWithAllowedKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	ctx := Given(t)
	addHelmProvenancePublicKey(t, helmProvenanceSigningKey, helmProvenanceKeyURL)
	_, err := fixture.RunCli("repo", "add", helmProvenanceRepoURL, "--type", "helm", "--name", "prom-provenance")
	require.NoError(t, err)

	ctx.
		Name(helmProvenancePassName).
		Project("gpg").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeProvenance, helmProvenanceSigningKey)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        helmProvenanceRepoURL,
				Chart:          helmProvenanceChart,
				TargetRevision: helmProvenanceChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmProvenancePassRel},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(NoConditions())
}

func TestHelmSourceIntegrityProvenanceFailsWithWrongKey(t *testing.T) {
	fixture.SkipOnEnv(t, "HELM")
	ctx := Given(t)
	addHelmProvenancePublicKey(t, helmProvenanceSigningKey, helmProvenanceKeyURL)
	_, err := fixture.RunCli("repo", "add", helmProvenanceRepoURL, "--type", "helm", "--name", "prom-wrong-key")
	require.NoError(t, err)

	ctx.
		Name(helmProvenanceFailName).
		Project("default").
		ProjectSpec(appProjectWithHelmSourceIntegrity(SourceIntegrityHelmPolicyProvenanceModeProvenance, helmProvenanceWrongKey)).
		When().
		IgnoreErrors().
		CreateFromFile(func(app *Application) {
			app.Spec.Source = &ApplicationSource{
				RepoURL:        helmProvenanceRepoURL,
				Chart:          helmProvenanceChart,
				TargetRevision: helmProvenanceChartV,
				Helm:           &ApplicationSourceHelm{ReleaseName: helmProvenanceFailRel},
			}
		}).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationError)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(Condition(ApplicationConditionComparisonError, "HELM/PROVENANCE")).
		Expect(Condition(ApplicationConditionComparisonError, "key_id="+helmProvenanceSigningKey))
}

func addHelmProvenancePublicKey(t *testing.T, keyID string, keyURL string) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), keyID+".gpg")
	_, err := fixture.Run("", "curl", "-fsSL", keyURL, "-o", keyPath)
	require.NoError(t, err)
	_, err = fixture.RunCli("gpg", "add", "--from", keyPath)
	require.NoError(t, err)

	if fixture.IsLocal() {
		keyData, err := os.ReadFile(keyPath)
		require.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("%s/app/config/gpg/source/%s", fixture.TmpDir(), keyID), keyData, 0o644)
		require.NoError(t, err)
		return
	}
	fixture.RestartRepoServer(t)
}
