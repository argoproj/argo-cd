package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/git"
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
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Verify the revision is a commit SHA, not the tag's ID
			assert.True(t, git.IsCommitSHA(app.Status.Sync.Revision))
			assert.NotContains(t, app.Status.Sync.Revision, "refs/tags/")
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
		}).
		And(func(app *Application) {
			// Verify the revision is a commit SHA, not the tag's ID
			assert.True(t, git.IsCommitSHA(app.Status.Sync.Revision))
			assert.NotContains(t, app.Status.Sync.Revision, "refs/tags/")
		})
}
