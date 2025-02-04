package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

func TestGitSemverResolutionNotUsingConstraint(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("v0.1.0").
		When().
		AddTag("v0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestGitSemverResolutionNotUsingConstraintWithLeadingZero(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("0.1.0").
		When().
		AddTag("0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestGitSemverResolutionUsingConstraint(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("v0.1.*").
		When().
		AddTag("v0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		PatchFile("deployment.yaml", `[
	{"op": "replace", "path": "/metadata/name", "value": "new-app"},
	{"op": "replace", "path": "/spec/replicas", "value": 1}
]`).
		AddTag("v0.1.2").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p corev1.Pod) bool { return strings.HasPrefix(p.Name, "new-app") }))
}

func TestGitSemverResolutionUsingConstraintWithLeadingZero(t *testing.T) {
	Given(t).
		Path("deployment").
		CustomSSHKnownHostsAdded().
		SSHRepoURLAdded(true).
		RepoURLType(fixture.RepoURLTypeSSH).
		Revision("0.1.*").
		When().
		AddTag("0.1.0").
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		PatchFile("deployment.yaml", `[
	{"op": "replace", "path": "/metadata/name", "value": "new-app"},
	{"op": "replace", "path": "/spec/replicas", "value": 1}
]`).
		AddTag("0.1.2").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Pod(func(p corev1.Pod) bool { return strings.HasPrefix(p.Name, "new-app") }))
}

func TestAnnotatedTagInStatusSyncRevision(t *testing.T) {
	Given(t).
		Path("deployment").
		RepoURLType(fixture.RepoURLTypeFile).
		Revision("my-annotated-tag").
		When().
		AddAnnotatedTag("my-annotated-tag", "Testing annotated tag resolution").
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{
					Prune:    true,
					SelfHeal: false,
				},
			}
		}).
		IgnoreErrors().
		Sync().
		Then().
		// Verify initial sync succeeds and uses commit SHA
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Verify the revision is a commit SHA, not the tag ref
			assert.Len(t, app.Status.Sync.Revision, 40)
			assert.NotContains(t, app.Status.Sync.Revision, "refs/tags/")
		}).
		When().
		// Make a manual change
		PatchFile("deployment.yaml", `[
            {"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}
        ]`).
		Refresh(RefreshTypeHard).
		Then().
		// Verify change persists (not reverted) but shows OutOfSync
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			// Verify the revision is still the commit SHA
			assert.Len(t, app.Status.Sync.Revision, 40)
			assert.NotContains(t, app.Status.Sync.Revision, "refs/tags/")
		}).
		When().
		Sync().
		Then().
		// Verify sync succeeds and changes are applied
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Verify the revision is still the commit SHA
			assert.Len(t, app.Status.Sync.Revision, 40)
			assert.NotContains(t, app.Status.Sync.Revision, "refs/tags/")
		}).
		And(func(_ *Application) {
			// Get deployment directly from k8s
			deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).
				Get(context.Background(), "nginx-deployment", metav1.GetOptions{})
			require.NoError(t, err)
			// Verify the revisionHistoryLimit is updated
			assert.Equal(t, int32(10), *deployment.Spec.RevisionHistoryLimit)
		})
}

func TestAutomatedSelfHealingAgainstAnnotatedTag(t *testing.T) {
	Given(t).
		Path("deployment").
		RepoURLType(fixture.RepoURLTypeFile).
		Revision("my-annotated-tag").
		When().
		AddAnnotatedTag("my-annotated-tag", "Initial commit").
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				Automated: &SyncPolicyAutomated{
					Prune:    true,
					SelfHeal: false,
				},
			}
		}).
		IgnoreErrors().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		// Update the annotated tag to a new commit
		PatchFile("deployment.yaml", `[
			{"op": "replace", "path": "/spec/replicas", "value": 1}
		]`).
		AddAnnotatedTag("my-annotated-tag", "Updated commit").
		Refresh(RefreshTypeHard).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		// Make a manual change to the live resource
		And(func() {
			deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).
				Get(context.Background(), "nginx-deployment", metav1.GetOptions{})
			require.NoError(t, err)
			deployment.Spec.Replicas = ptr.To(int32(2))
			_, err = fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).
				Update(context.Background(), deployment, metav1.UpdateOptions{})
			require.NoError(t, err)
		}).Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		// Verify manual change is NOT reverted (selfHeal=false)
		And(func(_ *Application) {
			assert.Eventually(t, func() bool {
				deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).
					Get(context.Background(), "nginx-deployment", metav1.GetOptions{})
				if err != nil {
					return false
				}
				return *deployment.Spec.Replicas == 2
			}, time.Second*30, time.Millisecond*500)
		})
}
