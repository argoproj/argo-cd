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
					Prune:    true,
					SelfHeal: true,
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
	assert.True(t, dst.Spec.SyncPolicy.Automated.Prune)
	assert.True(t, dst.Spec.SyncPolicy.Automated.SelfHeal)
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
	// Single source from v1beta1 should only set Source (not Sources)
	// to preserve HasMultipleSources() returning false for single-source apps
	assert.Empty(t, dst.Spec.Sources, "Sources should be empty for single-source apps")
	require.NotNil(t, dst.Spec.Source)
	assert.Equal(t, "https://github.com/example/repo", dst.Spec.Source.RepoURL)
	assert.False(t, dst.Spec.HasMultipleSources(), "Single-source app should not have multiple sources")
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
	// Multiple sources should NOT populate the Source field
	assert.Nil(t, dst.Spec.Source, "Source should not be set when there are multiple sources")
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
					Prune:    true,
					SelfHeal: true,
				},
				SyncOptions: v1alpha1.SyncOptions{
					"CreateNamespace=true",
					"ServerSideApply=true",
				},
			},
			RevisionHistoryLimit: int64Ptr(10),
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
	// Single-source apps should preserve Source and keep Sources empty
	require.NotNil(t, roundTripped.Spec.Source)
	assert.Equal(t, original.Spec.Source.RepoURL, roundTripped.Spec.Source.RepoURL)
	assert.Empty(t, roundTripped.Spec.Sources, "Sources should be empty for single-source apps")
	assert.False(t, roundTripped.Spec.HasMultipleSources(), "Single-source app should not have multiple sources")
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
					Prune:    true,
					SelfHeal: true,
				},
				SyncOptions: &SyncOptions{
					CreateNamespace: boolPtr(true),
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

func int64Ptr(i int64) *int64 {
	return &i
}
