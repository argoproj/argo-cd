package helm

// Version-miss recovery helpers (moved out of the original index_snapshot.go
// so the disk-only slice carries them without the PR3 projection cache).
// After a dependency-build failure that looks like a version lookup miss,
// missingDependencyVersion decides which cached index can plausibly satisfy
// the requested constraint — the caller then revalidates just those indexes
// against the origin (ignoreFreshness) and retries the build.

import (
	"os"
	"strings"

	"github.com/Masterminds/semver/v3"
	"sigs.k8s.io/yaml"
)

// isIndexVersionLookupError reports whether a helm dependency build failure
// looks like a version-lookup miss against the cached indexes (retryable via
// origin revalidation) rather than an origin validation, authentication or
// network failure (which must keep its normal failure path). oci:// and
// file:// sources never hit the index store.
func isIndexVersionLookupError(err error) bool {
	message := err.Error()
	if strings.Contains(message, "oci://") || strings.Contains(message, "file://") {
		return false
	}
	return strings.Contains(message, "can't get a valid version for") || strings.Contains(message, "chart not found in repo") || strings.HasSuffix(strings.TrimSpace(message), "Error: could not find : entry not found") || strings.HasSuffix(strings.TrimSpace(message), "Error: could not find : no matching version")
}

func helmRepositoryAliasName(repository string) string {
	if name, ok := strings.CutPrefix(repository, "@"); ok {
		return name
	}
	return strings.TrimPrefix(repository, "alias:")
}

// missingDependencyVersion reports whether the metadata files request a
// version of a dependency of `repository` that the given (cached) index body
// cannot satisfy. Only the failure path inspects requested versions.
func missingDependencyVersion(dir, repository string, aliases map[string]string, body []byte) bool {
	var metadata struct {
		Dependencies []struct {
			Name, Version, Repository string
		}
	}
	for _, name := range []string{"Chart.yaml", "requirements.yaml", "Chart.lock", "requirements.lock"} {
		data, err := ReadChartMetadata(dir, name)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil || yaml.Unmarshal(data, &metadata) != nil {
			return false
		}
	}
	type entry struct {
		Version string
		URLs    []string
	}
	var full struct {
		Entries map[string][]entry
	}
	if yaml.Unmarshal(body, &full) != nil {
		return false
	}
	for _, dependency := range metadata.Dependencies {
		target := dependency.Repository
		if strings.HasPrefix(target, "@") || strings.HasPrefix(target, "alias:") {
			target = aliases[helmRepositoryAliasName(target)]
		}
		if target == "" || helmRepositoryURLKey(target) != helmRepositoryURLKey(repository) {
			continue
		}
		constraint, err := semver.NewConstraint(dependency.Version)
		if err != nil {
			continue
		}
		found := false
		for _, entry := range full.Entries[dependency.Name] {
			version, err := semver.NewVersion(entry.Version)
			if err == nil && constraint.Check(version) && len(entry.URLs) > 0 {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}
