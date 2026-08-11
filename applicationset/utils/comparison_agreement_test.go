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

// SpecsEquivalent and CreateOrUpdate must agree on whether the *spec* has changed: the first decides
// whether progressive sync reports a pending change, the second whether anything is actually written.
// If the first says "changed" where the second declines to patch, the Application is never corrected
// and progressive sync loops without converging.
//
// The case exercised here is the awkward one: a JQ ignore rule selecting on a value that only exists
// after normalization (spec.project defaulting to "default"), against a generated Application that
// omits it. On this branch CreateOrUpdate applies the ignore rules *before* normalizing while its
// caller has already normalized the generated spec, so the rule sees a normalized desired side and an
// un-normalized live one. That asymmetry is easy to get wrong in one place only, which is why it is
// pinned here rather than reasoned about.
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

	// Both sides omit spec.project, so it only becomes "default" once NormalizeApplicationSpec runs.
	// That is what makes the ordering observable: the caller normalizes the generated spec before the
	// ignore rules see it, while the live side reaches them un-normalized, so the selector matches one
	// side only. Normalizing both up front instead (master's order) would make it match both and the
	// divergence would be invisible here.
	live := newApp("", "abc123")  // stored without an explicit project
	desired := newApp("", "HEAD") // generated without one either

	// 1. What the status path concludes.
	equivalent, err := SpecsEquivalent(ignore, normalizers.IgnoreNormalizerOpts{}, live, desired)
	require.NoError(t, err)

	// 2. What the write path concludes, driven through the real CreateOrUpdate.
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(live.DeepCopy()).Build()
	obj := live.DeepCopy()
	generated := desired.DeepCopy()
	// createOrUpdateInCluster normalizes the generated spec before calling CreateOrUpdate.
	// Reproducing that here is essential: the ignore rules run before normalization on this branch,
	// so omitting this step hides an ordering divergence between the two paths.
	generated.Spec = *argo.NormalizeApplicationSpec(&generated.Spec)
	op, err := CreateOrUpdate(t.Context(), log.NewEntry(log.New()), c, ignore, normalizers.IgnoreNormalizerOpts{}, obj, func() error {
		obj.Spec = generated.Spec
		return nil
	})
	require.NoError(t, err)

	writePathWouldPatch := op == controllerutil.OperationResultUpdated
	statusPathSeesChange := !equivalent

	t.Logf("statusPathSeesChange=%v  writePathWouldPatch=%v (op=%s)",
		statusPathSeesChange, writePathWouldPatch, op)

	require.Equal(t, writePathWouldPatch, statusPathSeesChange,
		"the two paths must reach the same verdict on a spec difference. If the status path reports a "+
			"change the write path will not make, the Application is never corrected and progressive "+
			"sync loops forever. (Metadata-only changes are out of scope here: CreateOrUpdate compares "+
			"whole objects and will patch those without SpecsEquivalent reporting a change, which cannot "+
			"loop.)")
}
