package hydrator

import (
	"regexp"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/versions"
)

// ociTagRegexp matches a tag as defined by the OCI distribution spec.
var ociTagRegexp = regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9._-]{0,127}$`)

// IsOCITag returns true if the revision is a tag that hydrated manifests can be pushed to. Digests cannot be pushed
// to, and a semver constraint would be resolved to some other tag when the application is synced.
func IsOCITag(revision string) bool {
	return ociTagRegexp.MatchString(revision) && !versions.IsConstraint(revision)
}

// ValidateSourceHydrator returns a message for every problem found in the source hydrator configuration. It only
// looks at the configuration itself; it does not check project permissions or how other applications are configured.
func ValidateSourceHydrator(h *appv1.SourceHydrator) []string {
	var problems []string
	if h.DrySource.RepoURL == "" {
		problems = append(problems, "spec.sourceHydrator.drySource.repoURL is required")
	}
	if h.IsSyncSourceOCI() {
		switch {
		case h.SyncSource.TargetRevision == "":
			problems = append(problems, "spec.sourceHydrator.syncSource.targetRevision is required when the sync source is an OCI repository")
		case !IsOCITag(h.SyncSource.TargetRevision):
			problems = append(problems, "spec.sourceHydrator.syncSource.targetRevision must be an OCI tag when the sync source is an OCI repository; digests and semver constraints are not supported")
		}
		if h.SyncSource.TargetBranch != "" {
			problems = append(problems, "spec.sourceHydrator.syncSource.targetBranch must not be set when the sync source is an OCI repository; use spec.sourceHydrator.syncSource.targetRevision")
		}
		if h.HydrateTo != nil {
			problems = append(problems, "spec.sourceHydrator.hydrateTo is not supported when the sync source is an OCI repository")
		}
		return problems
	}
	if h.SyncSource.TargetBranch == "" {
		problems = append(problems, "spec.sourceHydrator.syncSource.targetBranch is required")
	}
	if h.SyncSource.TargetRevision != "" {
		problems = append(problems, "spec.sourceHydrator.syncSource.targetRevision must only be set when the sync source is an OCI repository; use spec.sourceHydrator.syncSource.targetBranch")
	}
	if h.HydrateTo != nil && h.HydrateTo.TargetBranch == "" {
		problems = append(problems, "when spec.sourceHydrator.hydrateTo is set, spec.sourceHydrator.hydrateTo.targetBranch is required")
	}
	return problems
}
