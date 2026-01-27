package versions

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/Masterminds/semver/v3"
)

var (
	tagRegexString = `[a-zA-Z0-9-_./]+`
	tagPrefixRegex = regexp.MustCompile(`([a-zA-Z0-9-_./]+/).+`)
	tagRegex       = regexp.MustCompile(tagRegexString)
)

// extractPrefixes extracts a common path prefix from a semver revision/constraint string.
// The prefix must end with "/" and be consistent across all version segments in the constraint.
//
// Examples:
//   - "prod/v1.0.*" → ("prod/", "v1.0.*")
//   - "> prod/v1.0.0 < prod/v2.0.0" → ("prod/", "> v1.0.0 < v2.0.0")
//   - "> prod/v1.0.0 < dev/v2.0.0" → ("", "> prod/v1.0.0 < dev/v2.0.0") - mixed prefixes, no extraction
//   - "v1.0.*" → ("", "v1.0.*") - no prefix
func extractPrefixes(revision string) (prefix string, stripped string) {
	allMatches := tagRegex.FindAllString(revision, -1)
	submatches := tagPrefixRegex.FindStringSubmatch(revision)

	// we should find exactly 1 prefix and each tag should contain that prefix
	if len(submatches) == 2 && strings.Count(revision, submatches[1]) == len(allMatches) {
		return submatches[1], strings.ReplaceAll(revision, submatches[1], "")
	}
	return "", revision
}

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

// MaxVersion takes a revision and a list of tags.
// If the revision is a version, it returns that version, even if it is not in the list of tags.
// If the revision is not a version, but is also not a constraint, it returns that revision, even if it is not in the list of tags.
// If the revision is a constraint, it iterates over the list of tags to find the "maximum" tag which satisfies that
// constraint.
// If the revision is a constraint, but no tag satisfies that constraint, then it returns an error.
//
// Supports hierarchical tag prefixes (e.g., "prod/v1.0.*" will match tags like "prod/v1.0.0", "prod/v1.0.1").
// The prefix must be consistent across all version segments in the constraint.
func MaxVersion(revision string, tags []string) (string, error) {
	// Extract prefix from revision (e.g., "prod/v1.0.*" -> prefix: "prod/", stripped: "v1.0.*")
	constraintPrefix, constraintStripped := extractPrefixes(revision)

	if v, err := semver.NewVersion(constraintStripped); err == nil {
		// If the revision is a valid version, then we know it isn't a constraint; it's just a pin.
		// In which case, we should use standard tag resolution mechanisms and return the original value.
		// For example, the following are considered valid versions, and therefore should match an exact tag:
		// - "v1.0.0"/"1.0.0"
		// - "v1.0"/"1.0"
		return constraintPrefix + v.Original(), nil
	}

	constraints, err := semver.NewConstraint(constraintStripped)

	// Filter tags to only those matching the prefix, then strip prefix for version comparison
	filteredTags := filterTagsByPrefix(tags, constraintPrefix)
	strippedTags := stripPrefixFromTags(filteredTags, constraintPrefix)

	if err != nil {
		log.Debugf("Revision '%s' is not a valid semver constraint, resolving via basic string equality.", revision)
		// If this is also an invalid constraint, we just iterate over available tags to determine if it is valid/invalid.
		for _, tag := range strippedTags {
			if tag == constraintStripped {
				return constraintPrefix + constraintStripped, nil
			}
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

	log.Debugf("Semver constraint '%s' resolved to version '%s'", constraints.String(), constraintPrefix+maxVersion.Original())
	return constraintPrefix + maxVersion.Original(), nil
}

// Returns true if the given revision is not an exact semver and can be parsed as a semver constraint
func IsConstraint(revision string) bool {
	if _, err := semver.NewVersion(revision); err == nil {
		return false
	}
	_, err := semver.NewConstraint(revision)
	return err == nil
}
