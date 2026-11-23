package hydrator

import (
	"testing"

	"github.com/stretchr/testify/assert"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestIsOCITag(t *testing.T) {
	t.Parallel()
	for revision, expected := range map[string]bool{
		"dev":            true,
		"latest":         true,
		"1.18.9":         true,
		"v1.18.9":        true,
		"1.2":            true,
		"env_prod-eu.1":  true,
		"":               false,
		"1.x":            false,
		"1.*":            false,
		"^1.2.0":         false,
		">=1.0.0":        false,
		"-leading-dash":  false,
		"has/slash":      false,
		"sha256:" + "ab": false,
	} {
		assert.Equalf(t, expected, IsOCITag(revision), "revision %q", revision)
	}
}

func TestValidateSourceHydrator(t *testing.T) {
	t.Parallel()
	dry := appv1.DrySource{RepoURL: "https://example.com/dry", TargetRevision: "main", Path: "app"}

	tests := []struct {
		name     string
		hydrator appv1.SourceHydrator
		expected []string
	}{
		{
			name:     "valid git",
			hydrator: appv1.SourceHydrator{DrySource: dry, SyncSource: appv1.SyncSource{TargetBranch: "env/dev", Path: "app"}},
		},
		{
			name: "valid git with hydrateTo",
			hydrator: appv1.SourceHydrator{
				DrySource:  dry,
				SyncSource: appv1.SyncSource{TargetBranch: "env/dev", Path: "app"},
				HydrateTo:  &appv1.HydrateTo{TargetBranch: "env/dev-next"},
			},
		},
		{
			name:     "valid oci at root",
			hydrator: appv1.SourceHydrator{DrySource: dry, SyncSource: appv1.SyncSource{RepoURL: "oci://example.com/hydrated", TargetRevision: "dev", Path: "."}},
		},
		{
			name:     "missing dry repo",
			hydrator: appv1.SourceHydrator{SyncSource: appv1.SyncSource{RepoURL: "https://example.com/hydrated", TargetBranch: "env/dev", Path: "app"}},
			expected: []string{"spec.sourceHydrator.drySource.repoURL is required"},
		},
		{
			name:     "git without branch",
			hydrator: appv1.SourceHydrator{DrySource: dry, SyncSource: appv1.SyncSource{Path: "app"}},
			expected: []string{"spec.sourceHydrator.syncSource.targetBranch is required"},
		},
		{
			name:     "git with targetRevision instead of branch",
			hydrator: appv1.SourceHydrator{DrySource: dry, SyncSource: appv1.SyncSource{TargetRevision: "dev", Path: "app"}},
			expected: []string{
				"spec.sourceHydrator.syncSource.targetBranch is required",
				"spec.sourceHydrator.syncSource.targetRevision must only be set when the sync source is an OCI repository; use spec.sourceHydrator.syncSource.targetBranch",
			},
		},
		{
			name: "git hydrateTo without branch",
			hydrator: appv1.SourceHydrator{
				DrySource:  dry,
				SyncSource: appv1.SyncSource{TargetBranch: "env/dev", Path: "app"},
				HydrateTo:  &appv1.HydrateTo{},
			},
			expected: []string{"when spec.sourceHydrator.hydrateTo is set, spec.sourceHydrator.hydrateTo.targetBranch is required"},
		},
		{
			name:     "oci without tag",
			hydrator: appv1.SourceHydrator{DrySource: dry, SyncSource: appv1.SyncSource{RepoURL: "oci://example.com/hydrated", Path: "."}},
			expected: []string{"spec.sourceHydrator.syncSource.targetRevision is required when the sync source is an OCI repository"},
		},
		{
			name:     "oci with branch instead of tag",
			hydrator: appv1.SourceHydrator{DrySource: dry, SyncSource: appv1.SyncSource{RepoURL: "oci://example.com/hydrated", TargetBranch: "dev", Path: "."}},
			expected: []string{
				"spec.sourceHydrator.syncSource.targetRevision is required when the sync source is an OCI repository",
				"spec.sourceHydrator.syncSource.targetBranch must not be set when the sync source is an OCI repository; use spec.sourceHydrator.syncSource.targetRevision",
			},
		},
		{
			name:     "oci with semver constraint",
			hydrator: appv1.SourceHydrator{DrySource: dry, SyncSource: appv1.SyncSource{RepoURL: "oci://example.com/hydrated", TargetRevision: "1.x", Path: "."}},
			expected: []string{"spec.sourceHydrator.syncSource.targetRevision must be an OCI tag when the sync source is an OCI repository; digests and semver constraints are not supported"},
		},
		{
			name: "oci with hydrateTo",
			hydrator: appv1.SourceHydrator{
				DrySource:  dry,
				SyncSource: appv1.SyncSource{RepoURL: "oci://example.com/hydrated", TargetRevision: "dev", Path: "."},
				HydrateTo:  &appv1.HydrateTo{TargetBranch: "dev-next"},
			},
			expected: []string{"spec.sourceHydrator.hydrateTo is not supported when the sync source is an OCI repository"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ValidateSourceHydrator(&tt.hydrator))
		})
	}
}
