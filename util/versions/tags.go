package versions

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/Masterminds/semver/v3"
)

// filterTagsByPrefix returns only tags that have the specified prefix.
// If prefix is empty, returns all tags unchanged.
func filterTagsByPrefix(tags []string, prefix string) []string {
	if prefix == "" {
		return tags
	}

	var filtered []string
	for _, tag := range tags {
		if strings.HasPrefix(tag, prefix) {
			filtered = append(filtered, tag)
		}
	}
	return filtered
}

// stripPrefixFromTags removes the prefix from each tag.
// If prefix is empty, returns tags unchanged.
func stripPrefixFromTags(tags []string, prefix string) []string {
	if prefix == "" {
		return tags
	}

	stripped := make([]string, len(tags))
	for i, tag := range tags {
		stripped[i] = strings.TrimPrefix(tag, prefix)
	}
	return stripped
}

// MaxVersion returns the highest version tag satisfying revision, optionally scoped to a tag prefix.
// If tagPrefix is non-empty, only tags with that prefix are considered; the prefix is stripped before
// semver comparison and re-added to the returned value.
// Exact versions and non-constraint strings are returned as-is (with prefix prepended) without consulting the tag list.
// Returns an error if revision is a constraint and no matching tag is found.
func MaxVersion(revision string, tags []string, tagPrefix string) (string, error) {
	filteredTags := filterTagsByPrefix(tags, tagPrefix)
	strippedTags := stripPrefixFromTags(filteredTags, tagPrefix)

	if v, err := semver.NewVersion(revision); err == nil {
		// If the revision is a valid version, then we know it isn't a constraint; it's just a pin.
		// In which case, we should use standard tag resolution mechanisms and return the original value.
		// For example, the following are considered valid versions, and therefore should match an exact tag:
		// - "v1.0.0"/"1.0.0"
		// - "v1.0"/"1.0"
		return tagPrefix + v.Original(), nil
	}

	constraints, err := semver.NewConstraint(revision)

	if err != nil {
		log.Debugf("Revision '%s' is not a valid semver constraint, resolving via basic string equality.", revision)
		// If this is also an invalid constraint, we just iterate over available tags to determine if it is valid/invalid.
		if slices.Contains(strippedTags, revision) {
			return tagPrefix + revision, nil
		}
		return "", fmt.Errorf("failed to determine semver constraint: %w", err)
	}

	var maxVersion *semver.Version
	for _, tag := range strippedTags {
		v, err := semver.NewVersion(tag)

		// Invalid semantic version ignored
		if errors.Is(err, semver.ErrInvalidSemVer) {
			log.Debugf("Invalid semantic version: %s", tag)
			continue
		}
		if err != nil {
			return "", fmt.Errorf("invalid semver version in tags: %w", err)
		}
		if constraints.Check(v) {
			if maxVersion == nil || v.GreaterThan(maxVersion) {
				maxVersion = v
			}
		}
	}
	if maxVersion == nil {
		return "", fmt.Errorf("version matching constraint not found in %d tags", len(filteredTags))
	}

	log.Debugf("Semver constraint '%s' resolved to version '%s'", constraints.String(), tagPrefix+maxVersion.Original())
	return tagPrefix + maxVersion.Original(), nil
}

// Returns true if the given revision is not an exact semver and can be parsed as a semver constraint
func IsConstraint(revision string) bool {
	if _, err := semver.NewVersion(revision); err == nil {
		return false
	}
	_, err := semver.NewConstraint(revision)
	return err == nil
}
