package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
)

// applyIgnoreDifferences must neutralise an ignored field on BOTH the live and the generated
// Application, regardless of the TypeMeta each happens to arrive with. Its rules are GVK-scoped, but
// the live object is decoded from the API server while the generated one is built in Go with TypeMeta
// zero, so a rule matching one side only leaves the field stripped there and intact on the other --
// and the specs then never compare equal. See applyIgnoreDifferences for the mechanism.
func TestApplyIgnoreDifferencesIsTypeMetaIndependent(t *testing.T) {
	t.Parallel()

	withTypeMeta := metav1.TypeMeta{APIVersion: "argoproj.io/v1alpha1", Kind: "Application"}

	newApp := func(tm metav1.TypeMeta, rev string) *argov1alpha1.Application {
		return &argov1alpha1.Application{
			TypeMeta:   tm,
			ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "argocd"},
			Spec: argov1alpha1.ApplicationSpec{
				Project: "default",
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
		{JSONPointers: []string{"/spec/source/targetRevision"}},
	}
	diffConfig, err := BuildIgnoreDiffConfig(ignore, normalizers.IgnoreNormalizerOpts{})
	if err != nil {
		t.Fatalf("BuildIgnoreDiffConfig: %v", err)
	}

	for _, tc := range []struct {
		name      string
		liveTM    metav1.TypeMeta
		desiredTM metav1.TypeMeta
	}{
		{"both have TypeMeta", withTypeMeta, withTypeMeta},
		{"live has TypeMeta, desired does not (what the controller actually sees)", withTypeMeta, metav1.TypeMeta{}},
		{"neither has TypeMeta", metav1.TypeMeta{}, metav1.TypeMeta{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			live := newApp(tc.liveTM, "refs/heads/does-not-exist")
			desired := newApp(tc.desiredTM, "HEAD")

			if err := applyIgnoreDifferences(diffConfig, live, desired); err != nil {
				t.Fatalf("applyIgnoreDifferences: %v", err)
			}

			t.Logf("live.targetRevision=%q  desired.targetRevision=%q",
				live.Spec.Source.TargetRevision, desired.Spec.Source.TargetRevision)

			if live.Spec.Source.TargetRevision != desired.Spec.Source.TargetRevision {
				t.Errorf("ignored field must be neutralised on BOTH sides irrespective of TypeMeta; "+
					"got live=%q desired=%q -- the two specs can never compare equal, so progressive sync "+
					"reports a spec change on every reconcile",
					live.Spec.Source.TargetRevision, desired.Spec.Source.TargetRevision)
			}
		})
	}
}
