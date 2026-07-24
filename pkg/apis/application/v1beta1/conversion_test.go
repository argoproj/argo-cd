package v1beta1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestConvertFromV1alpha1_BasicApplication(t *testing.T) {
	src := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL:        "https://github.com/example/repo",
					Path:           "manifests",
					TargetRevision: "main",
				},
			},
		},
	}

	dst := ConvertFromV1alpha1(src)

	assert.Equal(t, "argoproj.io/v1beta1", dst.APIVersion)
	assert.Equal(t, "Application", dst.Kind)
	assert.Equal(t, "test-app", dst.Name)
	assert.Equal(t, "argocd", dst.Namespace)
	assert.Equal(t, "default", dst.Spec.Project)
	assert.Equal(t, "https://kubernetes.default.svc", dst.Spec.Destination.Server)
	require.Len(t, dst.Spec.Sources, 1)
	assert.Equal(t, "https://github.com/example/repo", dst.Spec.Sources[0].RepoURL)
	// Note: v1beta1.ApplicationSpec does not have a Source field
}

func TestConvertFromV1alpha1_SourceToSources(t *testing.T) {
	// Test conversion of legacy Source field to Sources
	src := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/example/repo",
				Path:           "manifests",
				TargetRevision: "main",
			},
		},
	}

	dst := ConvertFromV1alpha1(src)

	assert.Equal(t, "argoproj.io/v1beta1", dst.APIVersion)
	require.Len(t, dst.Spec.Sources, 1)
	assert.Equal(t, "https://github.com/example/repo", dst.Spec.Sources[0].RepoURL)
	assert.Equal(t, "manifests", dst.Spec.Sources[0].Path)
	assert.Equal(t, "main", dst.Spec.Sources[0].TargetRevision)
	// Note: v1beta1.ApplicationSpec does not have a Source field
}

func TestConvertFromV1alpha1_SourcesPreferredOverSource(t *testing.T) {
	// If both Source and Sources are set, Sources takes precedence
	src := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/example/old-repo",
				Path:           "old-manifests",
				TargetRevision: "old-main",
			},
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL:        "https://github.com/example/new-repo",
					Path:           "new-manifests",
					TargetRevision: "new-main",
				},
			},
		},
	}

	dst := ConvertFromV1alpha1(src)

	require.Len(t, dst.Spec.Sources, 1)
	assert.Equal(t, "https://github.com/example/new-repo", dst.Spec.Sources[0].RepoURL)
}

func TestConvertFromV1alpha1_SyncPolicy(t *testing.T) {
	src := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL:        "https://github.com/example/repo",
					TargetRevision: "main",
				},
			},
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune:    new(true),
					SelfHeal: new(true),
				},
				SyncOptions: v1alpha1.SyncOptions{
					"CreateNamespace=true",
					"ServerSideApply=true",
				},
				Retry: &v1alpha1.RetryStrategy{
					Limit: 5,
				},
			},
		},
	}

	dst := ConvertFromV1alpha1(src)

	require.NotNil(t, dst.Spec.SyncPolicy)
	require.NotNil(t, dst.Spec.SyncPolicy.Automated)
	assert.True(t, *dst.Spec.SyncPolicy.Automated.Prune)
	assert.True(t, *dst.Spec.SyncPolicy.Automated.SelfHeal)
	// SyncOptions is now a struct, not a slice
	require.NotNil(t, dst.Spec.SyncPolicy.SyncOptions)
	assert.True(t, *dst.Spec.SyncPolicy.SyncOptions.CreateNamespace)
	assert.True(t, *dst.Spec.SyncPolicy.SyncOptions.ServerSideApply)
	require.NotNil(t, dst.Spec.SyncPolicy.Retry)
	assert.Equal(t, int64(5), dst.Spec.SyncPolicy.Retry.Limit)
}

func TestConvertSyncOptions_AllFields(t *testing.T) {
	// Test conversion of all SyncOptions fields from v1alpha1 string format to v1beta1 structured format
	src := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL:        "https://github.com/example/repo",
					TargetRevision: "main",
				},
			},
			SyncPolicy: &v1alpha1.SyncPolicy{
				SyncOptions: v1alpha1.SyncOptions{
					"Validate=false",
					"CreateNamespace=true",
					"PrunePropagationPolicy=foreground",
					"Prune=confirm",
					"PruneLast=true",
					"Delete=false",
					"Replace=true",
					"Force=true",
					"ServerSideApply=true",
					"ApplyOutOfSyncOnly=true",
					"SkipDryRunOnMissingResource=true",
					"RespectIgnoreDifferences=true",
					"FailOnSharedResource=true",
					"ClientSideApplyMigration=true",
				},
			},
		},
	}

	dst := ConvertFromV1alpha1(src)

	require.NotNil(t, dst.Spec.SyncPolicy)
	require.NotNil(t, dst.Spec.SyncPolicy.SyncOptions)
	opts := dst.Spec.SyncPolicy.SyncOptions

	// Verify all structured fields
	require.NotNil(t, opts.Validate)
	assert.False(t, *opts.Validate)
	require.NotNil(t, opts.CreateNamespace)
	assert.True(t, *opts.CreateNamespace)
	require.NotNil(t, opts.PrunePropagationPolicy)
	assert.Equal(t, PrunePropagationPolicyForeground, *opts.PrunePropagationPolicy)
	require.NotNil(t, opts.Prune)
	assert.Equal(t, SyncOptionConfirm, *opts.Prune)
	require.NotNil(t, opts.PruneLast)
	assert.True(t, *opts.PruneLast)
	require.NotNil(t, opts.Delete)
	assert.Equal(t, SyncOptionDisabled, *opts.Delete)
	require.NotNil(t, opts.Replace)
	assert.True(t, *opts.Replace)
	require.NotNil(t, opts.Force)
	assert.True(t, *opts.Force)
	require.NotNil(t, opts.ServerSideApply)
	assert.True(t, *opts.ServerSideApply)
	require.NotNil(t, opts.ApplyOutOfSyncOnly)
	assert.True(t, *opts.ApplyOutOfSyncOnly)
	require.NotNil(t, opts.SkipDryRunOnMissingResource)
	assert.True(t, *opts.SkipDryRunOnMissingResource)
	require.NotNil(t, opts.RespectIgnoreDifferences)
	assert.True(t, *opts.RespectIgnoreDifferences)
	require.NotNil(t, opts.FailOnSharedResource)
	assert.True(t, *opts.FailOnSharedResource)
	require.NotNil(t, opts.ClientSideApplyMigration)
	assert.True(t, *opts.ClientSideApplyMigration)

	// Test round-trip back to v1alpha1
	roundTripped := ConvertToV1alpha1(dst)
	require.NotNil(t, roundTripped.Spec.SyncPolicy)
	require.NotNil(t, roundTripped.Spec.SyncPolicy.SyncOptions)

	// Verify all options are present in the string format
	stringOpts := roundTripped.Spec.SyncPolicy.SyncOptions
	assert.Contains(t, stringOpts, "Validate=false")
	assert.Contains(t, stringOpts, "CreateNamespace=true")
	assert.Contains(t, stringOpts, "PrunePropagationPolicy=foreground")
	assert.Contains(t, stringOpts, "Prune=confirm")
	assert.Contains(t, stringOpts, "PruneLast=true")
	assert.Contains(t, stringOpts, "Delete=false")
	assert.Contains(t, stringOpts, "Replace=true")
	assert.Contains(t, stringOpts, "Force=true")
	assert.Contains(t, stringOpts, "ServerSideApply=true")
	assert.Contains(t, stringOpts, "ApplyOutOfSyncOnly=true")
	assert.Contains(t, stringOpts, "SkipDryRunOnMissingResource=true")
	assert.Contains(t, stringOpts, "RespectIgnoreDifferences=true")
	assert.Contains(t, stringOpts, "FailOnSharedResource=true")
	assert.Contains(t, stringOpts, "ClientSideApplyMigration=true")
}

func TestConvertSyncOptions_ValidateTrueRoundTrip(t *testing.T) {
	// Validate=true is explicitly emitted on round-trip — otherwise a
	// v1alpha1 app with ["Validate=true"] would silently lose the option.
	src := &v1alpha1.Application{
		Spec: v1alpha1.ApplicationSpec{
			SyncPolicy: &v1alpha1.SyncPolicy{
				SyncOptions: v1alpha1.SyncOptions{"Validate=true"},
			},
		},
	}

	dst := ConvertFromV1alpha1(src)
	require.NotNil(t, dst.Spec.SyncPolicy)
	require.NotNil(t, dst.Spec.SyncPolicy.SyncOptions)
	require.NotNil(t, dst.Spec.SyncPolicy.SyncOptions.Validate)
	assert.True(t, *dst.Spec.SyncPolicy.SyncOptions.Validate)

	roundTripped := ConvertToV1alpha1(dst)
	require.NotNil(t, roundTripped.Spec.SyncPolicy)
	assert.Contains(t, roundTripped.Spec.SyncPolicy.SyncOptions, "Validate=true")
}

func TestConvertDoesNotAliasInput(t *testing.T) {
	// Mutating the converted v1alpha1 result must not mutate the v1beta1 input
	// (previously dst.Source aliased src.Sources[0]).
	src := &Application{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{SourceFormatAnnotation: SourceFormatSingular},
		},
		Spec: ApplicationSpec{
			Sources: ApplicationSources{
				{RepoURL: "https://original.example.com/repo", Path: "manifests"},
			},
		},
	}

	dst := ConvertToV1alpha1(src)
	require.NotNil(t, dst.Spec.Source)
	dst.Spec.Source.RepoURL = "https://mutated.example.com/repo"

	assert.Equal(t, "https://original.example.com/repo", src.Spec.Sources[0].RepoURL,
		"input v1beta1 source must not be mutated through the converted v1alpha1 result")
}

func TestConvertToV1alpha1_BasicApplication(t *testing.T) {
	src := &Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1beta1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Sources: ApplicationSources{
				{
					RepoURL:        "https://github.com/example/repo",
					Path:           "manifests",
					TargetRevision: "main",
				},
			},
		},
	}

	dst := ConvertToV1alpha1(src)

	assert.Equal(t, "argoproj.io/v1alpha1", dst.APIVersion)
	assert.Equal(t, "Application", dst.Kind)
	assert.Equal(t, "test-app", dst.Name)
	assert.Equal(t, "argocd", dst.Namespace)
	assert.Equal(t, "default", dst.Spec.Project)
	// A v1beta1 object without a source-format marker (e.g. created natively via
	// v1beta1) keeps the plural sources form in v1alpha1: collapsing a
	// single-element list into the singular field would make conversion
	// non-idempotent and rewrite stored specs.
	assert.Nil(t, dst.Spec.Source, "Source must not be synthesized without a source-format marker")
	require.Len(t, dst.Spec.Sources, 1)
	assert.Equal(t, "https://github.com/example/repo", dst.Spec.Sources[0].RepoURL)
}

func TestConvertToV1alpha1_MultipleSources(t *testing.T) {
	src := &Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1beta1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Sources: ApplicationSources{
				{
					RepoURL:        "https://github.com/example/repo1",
					Path:           "manifests1",
					TargetRevision: "main",
				},
				{
					RepoURL:        "https://github.com/example/repo2",
					Path:           "manifests2",
					TargetRevision: "main",
				},
			},
		},
	}

	dst := ConvertToV1alpha1(src)

	require.Len(t, dst.Spec.Sources, 2)
	// Source should NOT be set for multi-source apps - only Sources is used
	assert.Nil(t, dst.Spec.Source, "Source should not be set for multi-source apps")
}

func TestConvertRoundTrip_V1alpha1ToV1beta1ToV1alpha1(t *testing.T) {
	// Test with Source (not Sources) - the common case for single-source apps
	original := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-app",
			Namespace:  "argocd",
			Generation: 5,
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Source: &v1alpha1.ApplicationSource{
				RepoURL:        "https://github.com/example/repo",
				Path:           "manifests",
				TargetRevision: "main",
			},
			SyncPolicy: &v1alpha1.SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune:    new(true),
					SelfHeal: new(true),
				},
				SyncOptions: v1alpha1.SyncOptions{
					"CreateNamespace=true",
					"ServerSideApply=true",
				},
			},
			RevisionHistoryLimit: new(int64(10)),
			IgnoreDifferences: v1alpha1.IgnoreDifferences{
				{
					Group: "apps",
					Kind:  "Deployment",
					JSONPointers: []string{
						"/spec/replicas",
					},
				},
			},
		},
		Status: v1alpha1.ApplicationStatus{
			Sync: v1alpha1.SyncStatus{
				Status: v1alpha1.SyncStatusCodeSynced,
			},
			Health: v1alpha1.AppHealthStatus{
				Status: "Healthy",
			},
		},
	}

	// Convert to v1beta1
	v1beta1App := ConvertFromV1alpha1(original)

	// Convert back to v1alpha1
	roundTripped := ConvertToV1alpha1(v1beta1App)

	// Verify key fields are preserved
	assert.Equal(t, "argoproj.io/v1alpha1", roundTripped.APIVersion)
	assert.Equal(t, original.Name, roundTripped.Name)
	assert.Equal(t, original.Namespace, roundTripped.Namespace)
	assert.Equal(t, original.Labels, roundTripped.Labels)
	assert.Equal(t, original.Spec.Project, roundTripped.Spec.Project)
	assert.Equal(t, original.Spec.Destination, roundTripped.Spec.Destination)
	// The original used the singular `source` form; the round-trip must restore
	// exactly that form (recorded via SourceFormatAnnotation).
	require.NotNil(t, roundTripped.Spec.Source)
	assert.Equal(t, original.Spec.Source.RepoURL, roundTripped.Spec.Source.RepoURL)
	assert.Empty(t, roundTripped.Spec.Sources, "Sources should not be set for single-source apps")
	assert.False(t, roundTripped.Spec.HasMultipleSources(), "Single-source app should have HasMultipleSources false")
	assert.NotContains(t, roundTripped.Annotations, SourceFormatAnnotation,
		"the source-format marker is conversion metadata and must be stripped from the v1alpha1 form")
	assert.Equal(t, original.Spec.SyncPolicy.Automated.Prune, roundTripped.Spec.SyncPolicy.Automated.Prune)
	assert.Equal(t, original.Spec.SyncPolicy.Automated.SelfHeal, roundTripped.Spec.SyncPolicy.Automated.SelfHeal)
	assert.Equal(t, original.Spec.SyncPolicy.SyncOptions, roundTripped.Spec.SyncPolicy.SyncOptions)
	assert.Equal(t, *original.Spec.RevisionHistoryLimit, *roundTripped.Spec.RevisionHistoryLimit)
	assert.Equal(t, original.Spec.IgnoreDifferences, roundTripped.Spec.IgnoreDifferences)
	assert.Equal(t, original.Status.Sync.Status, roundTripped.Status.Sync.Status)
	assert.Equal(t, original.Status.Health.Status, roundTripped.Status.Health.Status)
}

func TestConvertRoundTrip_V1beta1ToV1alpha1ToV1beta1(t *testing.T) {
	original := &Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1beta1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-app",
			Namespace: "argocd",
		},
		Spec: ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			Sources: ApplicationSources{
				{
					RepoURL:        "https://github.com/example/repo",
					Path:           "manifests",
					TargetRevision: "main",
				},
			},
			SyncPolicy: &SyncPolicy{
				Automated: &v1alpha1.SyncPolicyAutomated{
					Prune:    new(true),
					SelfHeal: new(true),
				},
				SyncOptions: &SyncOptions{
					CreateNamespace: new(true),
				},
			},
		},
	}

	// Convert to v1alpha1
	v1alpha1App := ConvertToV1alpha1(original)

	// Convert back to v1beta1
	roundTripped := ConvertFromV1alpha1(v1alpha1App)

	// Verify key fields are preserved
	assert.Equal(t, "argoproj.io/v1beta1", roundTripped.APIVersion)
	assert.Equal(t, original.Name, roundTripped.Name)
	assert.Equal(t, original.Namespace, roundTripped.Namespace)
	assert.Equal(t, original.Spec.Project, roundTripped.Spec.Project)
	assert.Equal(t, original.Spec.Destination, roundTripped.Spec.Destination)
	assert.Equal(t, original.Spec.Sources, roundTripped.Spec.Sources)
	// Note: v1beta1.ApplicationSpec does not have a Source field
	assert.Equal(t, original.Spec.SyncPolicy.Automated.Prune, roundTripped.Spec.SyncPolicy.Automated.Prune)
	// Verify SyncOptions round-tripped correctly
	require.NotNil(t, roundTripped.Spec.SyncPolicy.SyncOptions)
	assert.True(t, *roundTripped.Spec.SyncPolicy.SyncOptions.CreateNamespace)
}

func TestConvertStatus_ObservedGeneration(t *testing.T) {
	// ObservedGeneration lives on the shared (embedded) v1alpha1 status and must
	// be preserved in both directions and across a round-trip.

	t.Run("v1alpha1 to v1beta1 - ObservedGeneration is preserved", func(t *testing.T) {
		src := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-app",
				Namespace:  "argocd",
				Generation: 5,
			},
			Spec: v1alpha1.ApplicationSpec{
				Project: "default",
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "default",
				},
				Source: &v1alpha1.ApplicationSource{
					RepoURL: "https://github.com/example/repo",
				},
			},
			Status: v1alpha1.ApplicationStatus{
				ObservedGeneration: 3,
				Sync: v1alpha1.SyncStatus{
					Status: v1alpha1.SyncStatusCodeSynced,
				},
			},
		}

		dst := ConvertFromV1alpha1(src)

		assert.Equal(t, int64(3), dst.Status.ObservedGeneration)
		assert.Equal(t, v1alpha1.SyncStatusCodeSynced, dst.Status.Sync.Status)
	})

	t.Run("v1beta1 to v1alpha1 - ObservedGeneration is preserved", func(t *testing.T) {
		src := &Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-app",
				Namespace:  "argocd",
				Generation: 5,
			},
			Spec: ApplicationSpec{
				Project: "default",
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "default",
				},
				Sources: ApplicationSources{
					{RepoURL: "https://github.com/example/repo"},
				},
			},
			Status: ApplicationStatus{
				ApplicationStatus: v1alpha1.ApplicationStatus{
					ObservedGeneration: 3,
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeSynced,
					},
				},
			},
		}

		dst := ConvertToV1alpha1(src)

		assert.Equal(t, int64(3), dst.Status.ObservedGeneration)
		assert.Equal(t, v1alpha1.SyncStatusCodeSynced, dst.Status.Sync.Status)
	})

	t.Run("round-trip preserves ObservedGeneration", func(t *testing.T) {
		src := &Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-app",
				Namespace:  "argocd",
				Generation: 5,
			},
			Spec: ApplicationSpec{
				Project: "default",
				Destination: v1alpha1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "default",
				},
				Sources: ApplicationSources{
					{RepoURL: "https://github.com/example/repo"},
				},
			},
			Status: ApplicationStatus{
				ApplicationStatus: v1alpha1.ApplicationStatus{
					ObservedGeneration: 3,
					Sync: v1alpha1.SyncStatus{
						Status: v1alpha1.SyncStatusCodeSynced,
					},
				},
			},
		}

		roundTripped := ConvertFromV1alpha1(ConvertToV1alpha1(src))

		assert.Equal(t, int64(3), roundTripped.Status.ObservedGeneration)
		assert.Equal(t, v1alpha1.SyncStatusCodeSynced, roundTripped.Status.Sync.Status)
	})
}

func TestConvertOperation_RelocatedUnderStatus(t *testing.T) {
	// In v1alpha1 `operation` is a top-level field; in v1beta1 it is relocated
	// under status (so requesting a sync is gated by the status subresource). The
	// conversion must move it between the two locations without altering its value.
	op := &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{Revision: "abc123", Prune: true},
	}

	t.Run("v1alpha1 top-level operation -> v1beta1 status.operation", func(t *testing.T) {
		src := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "argocd"},
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
				Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo"},
			},
			Operation: op,
		}

		dst := ConvertFromV1alpha1(src)

		require.NotNil(t, dst.Status.Operation, "operation must land under status in v1beta1")
		assert.Equal(t, op, dst.Status.Operation)
	})

	t.Run("v1beta1 status.operation -> v1alpha1 top-level operation", func(t *testing.T) {
		src := &Application{
			ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "argocd"},
			Spec: ApplicationSpec{
				Project:     "default",
				Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
				Sources:     ApplicationSources{{RepoURL: "https://github.com/example/repo"}},
			},
			Status: ApplicationStatus{Operation: op},
		}

		dst := ConvertToV1alpha1(src)

		require.NotNil(t, dst.Operation, "operation must land top-level in v1alpha1")
		assert.Equal(t, op, dst.Operation)
	})

	t.Run("nil operation round-trips as nil", func(t *testing.T) {
		src := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "argocd"},
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
				Source:      &v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo"},
			},
		}

		roundTripped := ConvertToV1alpha1(ConvertFromV1alpha1(src))

		assert.Nil(t, roundTripped.Operation)
	})
}

func TestConvertFromV1alpha1_SourceHydratorDoesNotSetSources(t *testing.T) {
	// A hydrator-only app (no source/sources in v1alpha1) converts with empty
	// Sources — there is nothing to merge.
	src := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hydrator-app",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			SourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://github.com/example/repo",
					Path:           "configs",
					TargetRevision: "main",
				},
				SyncSource: v1alpha1.SyncSource{
					Path:         "hydrated",
					TargetBranch: "env/dev",
				},
			},
		},
	}

	dst := ConvertFromV1alpha1(src)

	// SourceHydrator should be preserved
	require.NotNil(t, dst.Spec.SourceHydrator)
	assert.Equal(t, "https://github.com/example/repo", dst.Spec.SourceHydrator.DrySource.RepoURL)
	assert.Equal(t, "env/dev", dst.Spec.SourceHydrator.SyncSource.TargetBranch)

	// Nothing to merge, so Sources stays empty
	assert.Empty(t, dst.Spec.Sources, "Sources should be empty when the v1alpha1 app has no source/sources")
}

func TestConvertFromV1alpha1_SourceHydratorKeepsSourcesIfBothSet(t *testing.T) {
	// v1alpha1 legally stores both SourceHydrator and Sources. Conversion must
	// be lossless: dropping sources here would permanently erase them from
	// storage on any v1beta1 round-trip.
	src := &v1alpha1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1alpha1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hydrator-app",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			SourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://github.com/example/repo",
					Path:           "configs",
					TargetRevision: "main",
				},
				SyncSource: v1alpha1.SyncSource{
					Path:         "hydrated",
					TargetBranch: "env/dev",
				},
			},
			Sources: v1alpha1.ApplicationSources{
				{
					RepoURL:        "https://github.com/example/other-repo",
					Path:           "manifests",
					TargetRevision: "main",
				},
			},
		},
	}

	dst := ConvertFromV1alpha1(src)

	// SourceHydrator should be preserved
	require.NotNil(t, dst.Spec.SourceHydrator)

	// Sources must be preserved alongside the hydrator
	require.Len(t, dst.Spec.Sources, 1)
	assert.Equal(t, "https://github.com/example/other-repo", dst.Spec.Sources[0].RepoURL)

	// And the round-trip back to v1alpha1 must be identity
	roundTripped := ConvertToV1alpha1(dst)
	assert.Equal(t, src.Spec.SourceHydrator, roundTripped.Spec.SourceHydrator)
	assert.Equal(t, src.Spec.Sources, roundTripped.Spec.Sources)
	assert.Nil(t, roundTripped.Spec.Source)
}

func TestConvertToV1alpha1_SourceHydratorPreserved(t *testing.T) {
	// Test that SourceHydrator is preserved when converting from v1beta1 to v1alpha1
	src := &Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1beta1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-hydrator-app",
			Namespace: "argocd",
		},
		Spec: ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "default",
			},
			SourceHydrator: &v1alpha1.SourceHydrator{
				DrySource: v1alpha1.DrySource{
					RepoURL:        "https://github.com/example/repo",
					Path:           "configs",
					TargetRevision: "main",
				},
				SyncSource: v1alpha1.SyncSource{
					Path:         "hydrated",
					TargetBranch: "env/dev",
				},
			},
			// Sources should be empty for hydrator apps
		},
	}

	dst := ConvertToV1alpha1(src)

	// SourceHydrator should be preserved
	require.NotNil(t, dst.Spec.SourceHydrator)
	assert.Equal(t, "https://github.com/example/repo", dst.Spec.SourceHydrator.DrySource.RepoURL)
	assert.Equal(t, "env/dev", dst.Spec.SourceHydrator.SyncSource.TargetBranch)

	// Source and Sources should NOT be set for hydrator apps
	assert.Nil(t, dst.Spec.Source, "Source should not be set for hydrator apps")
	assert.Empty(t, dst.Spec.Sources, "Sources should be empty for hydrator apps")
}

// TestConvertSyncOptions_AllKnownOptionsRoundTrip guards against silent data loss in
// SyncOptions conversion. Every option string with a structured representation must
// survive a v1alpha1 -> v1beta1 -> v1alpha1 round trip via its structured field; if
// this list drifts from the switch in conversion.go, this test fails. Strings without
// a structured field are dropped deliberately: new options are added to the v1beta1
// type, not backported to v1alpha1's string list, so unknown strings are typos or
// dead data (see TestConvertSyncOptions_UnknownOptionsDropped).
func TestConvertSyncOptions_AllKnownOptionsRoundTrip(t *testing.T) {
	knownOptions := []string{
		"Validate=true",
		"Validate=false",
		"CreateNamespace=true",
		"CreateNamespace=false",
		"PruneLast=true",
		"PruneLast=false",
		"Replace=true",
		"Replace=false",
		"Force=true",
		"Force=false",
		"ServerSideApply=true",
		"ServerSideApply=false",
		"ApplyOutOfSyncOnly=true",
		"ApplyOutOfSyncOnly=false",
		"SkipDryRunOnMissingResource=true",
		"SkipDryRunOnMissingResource=false",
		"RespectIgnoreDifferences=true",
		"RespectIgnoreDifferences=false",
		"FailOnSharedResource=true",
		"FailOnSharedResource=false",
		"ClientSideApplyMigration=true",
		"ClientSideApplyMigration=false",
		"Prune=false",
		"Prune=confirm",
		"Delete=false",
		"Delete=confirm",
		"PrunePropagationPolicy=background",
		"PrunePropagationPolicy=foreground",
		"PrunePropagationPolicy=orphan",
	}

	for _, opt := range knownOptions {
		t.Run(opt, func(t *testing.T) {
			structured := convertSyncOptionsFromStrings(v1alpha1.SyncOptions{opt})
			roundTripped := convertSyncOptionsToStrings(structured)
			assert.Contains(t, roundTripped, opt, "sync option %q was lost during round-trip conversion; add a case for it in convertSyncOptionsFromStrings/convertSyncOptionsToStrings", opt)
		})
	}
}

// TestConvertSyncOptions_UnknownOptionsDropped documents that option strings without a
// structured field are dropped on conversion. This is deliberate: new sync options are
// added to the v1beta1 structured type and are not backported to v1alpha1's string
// list, so an unknown string in a v1alpha1 object is a typo or dead data.
func TestConvertSyncOptions_UnknownOptionsDropped(t *testing.T) {
	structured := convertSyncOptionsFromStrings(v1alpha1.SyncOptions{"NotARealOption=42", "Validate=true"})

	roundTripped := convertSyncOptionsToStrings(structured)
	assert.NotContains(t, roundTripped, "NotARealOption=42")
	assert.Contains(t, roundTripped, "Validate=true")
}

// TestConvertRoundTrip_SourceFormPreserved verifies that the three v1alpha1 ways of
// expressing sources — singular `source`, plural `sources`, or both — each survive a
// v1alpha1 -> v1beta1 -> v1alpha1 round trip in their exact original form. Anything
// else flips HasMultipleSources() and rewrites stored specs on conversion.
func TestConvertRoundTrip_SourceFormPreserved(t *testing.T) {
	source := v1alpha1.ApplicationSource{RepoURL: "https://github.com/example/repo", Path: "manifests"}

	newApp := func(mutate func(*v1alpha1.ApplicationSpec)) *v1alpha1.Application {
		app := &v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{Name: "test-app", Namespace: "argocd"},
			Spec: v1alpha1.ApplicationSpec{
				Project:     "default",
				Destination: v1alpha1.ApplicationDestination{Server: "https://kubernetes.default.svc"},
			},
		}
		mutate(&app.Spec)
		return app
	}

	t.Run("singular source", func(t *testing.T) {
		original := newApp(func(spec *v1alpha1.ApplicationSpec) { spec.Source = &source })
		roundTripped := ConvertToV1alpha1(ConvertFromV1alpha1(original))
		assert.Equal(t, original.Spec.Source, roundTripped.Spec.Source)
		assert.Empty(t, roundTripped.Spec.Sources)
		assert.False(t, roundTripped.Spec.HasMultipleSources())
	})

	t.Run("single-element sources", func(t *testing.T) {
		original := newApp(func(spec *v1alpha1.ApplicationSpec) {
			spec.Sources = v1alpha1.ApplicationSources{source}
		})
		roundTripped := ConvertToV1alpha1(ConvertFromV1alpha1(original))
		assert.Nil(t, roundTripped.Spec.Source, "singular source must not be synthesized for a sources-form app")
		assert.Equal(t, original.Spec.Sources, roundTripped.Spec.Sources)
		assert.True(t, roundTripped.Spec.HasMultipleSources(), "HasMultipleSources must not flip on round-trip")
	})

	t.Run("both source and sources", func(t *testing.T) {
		original := newApp(func(spec *v1alpha1.ApplicationSpec) {
			spec.Source = &source
			spec.Sources = v1alpha1.ApplicationSources{source}
		})
		roundTripped := ConvertToV1alpha1(ConvertFromV1alpha1(original))
		assert.Equal(t, original.Spec.Source, roundTripped.Spec.Source)
		assert.Equal(t, original.Spec.Sources, roundTripped.Spec.Sources)
	})

	t.Run("marker annotation is stripped from the v1alpha1 form", func(t *testing.T) {
		original := newApp(func(spec *v1alpha1.ApplicationSpec) { spec.Source = &source })
		converted := ConvertFromV1alpha1(original)
		assert.Equal(t, SourceFormatSingular, converted.Annotations[SourceFormatAnnotation])
		roundTripped := ConvertToV1alpha1(converted)
		assert.NotContains(t, roundTripped.Annotations, SourceFormatAnnotation)
	})
}
