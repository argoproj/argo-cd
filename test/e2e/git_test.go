package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
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
		Path(guestbookPath).
		When().
		// Create annotated tag name 'annotated-tag'
		AddAnnotatedTag("annotated-tag", "my-generic-tag-message").
		// Create Application targeting annotated-tag, with automatedSync: true
		CreateFromFile(func(app *Application) {
			app.Spec.Source.TargetRevision = "annotated-tag"
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{Prune: true, SelfHeal: false}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			annotatedTagIDOutput, err := fixture.Run(fixture.TmpDir+"/testdata.git", "git", "show-ref", "annotated-tag")
			require.NoError(t, err)
			require.NotEmpty(t, annotatedTagIDOutput)
			// example command output:
			// "569798c430515ffe170bdb23e3aafaf8ae24b9ff refs/tags/annotated-tag"
			annotatedTagIDFields := strings.Fields(string(annotatedTagIDOutput))
			require.Len(t, annotatedTagIDFields, 2)

			targetCommitID, err := fixture.Run(fixture.TmpDir+"/testdata.git", "git", "rev-parse", "--verify", "annotated-tag^{commit}")
			// example command output:
			// "bcd35965e494273355265b9f0bf85075b6bc5163"
			require.NoError(t, err)
			require.NotEmpty(t, targetCommitID)

			require.NotEmpty(t, app.Status.Sync.Revision, "revision in sync status should be set by sync operation")

			require.NotEqual(t, app.Status.Sync.Revision, annotatedTagIDFields[0], "revision should not match the annotated tag id")
			require.Equal(t, app.Status.Sync.Revision, strings.TrimSpace(string(targetCommitID)), "revision SHOULD match the target commit SHA")
		})
}

// Test updates to K8s resources should not trigger a self-heal when self-heal is false.
func TestAutomatedSelfHealingAgainstAnnotatedTag(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		AddAnnotatedTag("annotated-tag", "my-generic-tag-message").
		// App should be auto-synced once created
		CreateFromFile(func(app *Application) {
			app.Spec.Source.TargetRevision = "annotated-tag"
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{Prune: true, SelfHeal: false}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		ExpectConsistently(SyncStatusIs(SyncStatusCodeSynced), WaitDuration, time.Second*10).
		When().
		// Update the annotated tag to a new git commit, that has a new revisionHistoryLimit.
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`).
		AddAnnotatedTag("annotated-tag", "my-generic-tag-message").
		Refresh(RefreshTypeHard).
		// The Application should update to the new annotated tag value within 10 seconds.
		And(func() {
			// Deployment revisionHistoryLimit should switch to 10
			timeoutErr := wait.PollUntilContextTimeout(t.Context(), 1*time.Second, 10*time.Second, true, func(context.Context) (done bool, err error) {
				deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
				if err != nil {
					return false, nil
				}

				revisionHistoryLimit := deployment.Spec.RevisionHistoryLimit
				return revisionHistoryLimit != nil && *revisionHistoryLimit == 10, nil
			})
			require.NoError(t, timeoutErr)
		}).
		// Update the Deployment to a different revisionHistoryLimit
		And(func() {
			errors.NewHandler(t).FailOnErr(fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.MergePatchType, []byte(`{"spec": {"revisionHistoryLimit": 9}}`), metav1.PatchOptions{}))
		}).
		// The revisionHistoryLimit should NOT be self-healed, because selfHealing: false. It should remain at 9.
		And(func() {
			// Wait up to 10 seconds to ensure that deployment revisionHistoryLimit does NOT should switch to 10, it should remain at 9.
			waitErr := wait.PollUntilContextTimeout(t.Context(), 1*time.Second, 10*time.Second, true, func(context.Context) (done bool, err error) {
				deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
				if err != nil {
					return false, nil
				}

				revisionHistoryLimit := deployment.Spec.RevisionHistoryLimit
				return revisionHistoryLimit != nil && *revisionHistoryLimit != 9, nil
			})
			require.Error(t, waitErr, "A timeout error should occur, indicating that revisionHistoryLimit never changed from 9")
		})
}

func TestAutomatedSelfHealingAgainstLightweightTag(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		AddTag("annotated-tag").
		// App should be auto-synced once created
		CreateFromFile(func(app *Application) {
			app.Spec.Source.TargetRevision = "annotated-tag"
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{Prune: true, SelfHeal: false}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		ExpectConsistently(SyncStatusIs(SyncStatusCodeSynced), WaitDuration, time.Second*10).
		When().
		// Update the annotated tag to a new git commit, that has a new revisionHistoryLimit.
		PatchFile("guestbook-ui-deployment.yaml", `[{"op": "replace", "path": "/spec/revisionHistoryLimit", "value": 10}]`).
		AddTagWithForce("annotated-tag").
		Refresh(RefreshTypeHard).
		// The Application should update to the new annotated tag value within 10 seconds.
		And(func() {
			// Deployment revisionHistoryLimit should switch to 10
			timeoutErr := wait.PollUntilContextTimeout(t.Context(), 1*time.Second, 10*time.Second, true, func(context.Context) (done bool, err error) {
				deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
				if err != nil {
					return false, nil
				}

				revisionHistoryLimit := deployment.Spec.RevisionHistoryLimit
				return revisionHistoryLimit != nil && *revisionHistoryLimit == 10, nil
			})
			require.NoError(t, timeoutErr)
		}).
		// Update the Deployment to a different revisionHistoryLimit
		And(func() {
			errors.NewHandler(t).FailOnErr(fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Patch(t.Context(),
				"guestbook-ui", types.MergePatchType, []byte(`{"spec": {"revisionHistoryLimit": 9}}`), metav1.PatchOptions{}))
		}).
		// The revisionHistoryLimit should NOT be self-healed, because selfHealing: false
		And(func() {
			// Wait up to 10 seconds to ensure that deployment revisionHistoryLimit does NOT should switch to 10, it should remain at 9.
			waitErr := wait.PollUntilContextTimeout(t.Context(), 1*time.Second, 10*time.Second, true, func(context.Context) (done bool, err error) {
				deployment, err := fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Get(t.Context(), "guestbook-ui", metav1.GetOptions{})
				if err != nil {
					return false, nil
				}

				revisionHistoryLimit := deployment.Spec.RevisionHistoryLimit
				return revisionHistoryLimit != nil && *revisionHistoryLimit != 9, nil
			})
			require.Error(t, waitErr, "A timeout error should occur, indicating that revisionHistoryLimit never changed from 9")
		})
}
