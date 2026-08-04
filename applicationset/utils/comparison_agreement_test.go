package utils

import (
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

// SpecsEquivalent and CreateOrUpdate must always agree on whether a spec has changed.
//
// That agreement is the whole contract: progressive sync uses SpecsEquivalent to decide whether to
// report a pending change, and CreateOrUpdate decides whether to actually write. If the first says
// "changed" where the second declines to patch, the Application is never corrected, the change is
// still pending on the next reconcile, and the ApplicationSet reconciles in a tight loop that never
// converges. If the reverse, rollout ordering is decided from stale status.
//
// The case exercised here is the awkward one: a JQ ignore rule that selects on a value which only
// exists after normalization (spec.project defaulting to "default"), against a generated Application
// that omits it. The relative order of normalization and ignore rules is load bearing and easy to
// change in one place only, which is why this is pinned.
func TestSpecComparisonAgreesWithWritePath(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	require.NoError(t, argov1alpha1.AddToScheme(scheme))

	newApp := func(project, rev string) *argov1alpha1.Application {
		return &argov1alpha1.Application{
			TypeMeta:   metav1.TypeMeta{APIVersion: "argoproj.io/v1alpha1", Kind: "Application"},
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "argocd"},
			Spec: argov1alpha1.ApplicationSpec{
				Project: project,
				Source: &argov1alpha1.ApplicationSource{
					RepoURL:        "git://example/repo",
					Path:           "x",
					TargetRevision: rev,
				},
				Destination: argov1alpha1.ApplicationDestination{
					Server: "https://kubernetes.default.svc", Namespace: "argocd",
				},
			},
		}
	}

	ignore := argov1alpha1.ApplicationSetIgnoreDifferences{
		{JQPathExpressions: []string{`.spec | select(.project == "default") | .source.targetRevision`}},
	}
	diffConfig, err := BuildIgnoreDiffConfig(ignore, normalizers.IgnoreNormalizerOpts{})
	require.NoError(t, err)

	live := newApp("default", "abc123") // stored: project already defaulted
	desired := newApp("", "HEAD")       // generated: project omitted

	// 1. What the status path concludes.
	equivalent, err := SpecsEquivalent(diffConfig, live, desired)
	require.NoError(t, err)

	// 2. What the write path concludes, driven through the real CreateOrUpdate.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live.DeepCopy()).Build()
	obj := live.DeepCopy()
	generated := desired.DeepCopy()
	// createOrUpdateInCluster normalizes the generated spec before calling CreateOrUpdate, which
	// documents that its caller must do so. Reproducing that here is essential: an ignore rule
	// selecting on a normalized value only matches once normalization has happened, so omitting this
	// step hides any ordering divergence between the two paths.
	generated.Spec = *argo.NormalizeApplicationSpec(&generated.Spec)
	op, err := CreateOrUpdate(t.Context(), log.NewEntry(log.New()), c, diffConfig, obj, func() error {
		obj.Spec = generated.Spec
		return nil
	})
	require.NoError(t, err)

	writePathWouldPatch := op == controllerutil.OperationResultUpdated
	statusPathSeesChange := !equivalent

	t.Logf("statusPathSeesChange=%v  writePathWouldPatch=%v (op=%s)",
		statusPathSeesChange, writePathWouldPatch, op)

	require.Equal(t, writePathWouldPatch, statusPathSeesChange,
		"the two paths must agree. If the status path reports a change the write path will not make, "+
			"the Application is never corrected and progressive sync loops forever; if the write path "+
			"patches while the status path sees nothing, ordering decisions are made on stale status.")
}
