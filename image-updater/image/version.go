package image

import (
	"path/filepath"

	"github.com/argoproj/argo-cd/v2/image-updater/log"
	"github.com/argoproj/argo-cd/v2/image-updater/options"
	"github.com/argoproj/argo-cd/v2/image-updater/tag"

	"github.com/Masterminds/semver"
)

// VersionSortMode defines the method to sort a list of tags
type UpdateStrategy int

const (
	// VersionSortSemVer sorts tags using semver sorting (the default)
	StrategySemVer UpdateStrategy = 0
	// VersionSortLatest sorts tags after their creation date
	StrategyNewestBuild UpdateStrategy = 1
	// VersionSortName sorts tags alphabetically by name
	StrategyAlphabetical UpdateStrategy = 2
	// VersionSortDigest uses latest digest of an image
	StrategyDigest UpdateStrategy = 3
)

func (us UpdateStrategy) String() string {
	switch us {
	case StrategySemVer:
		return "semver"
	case StrategyNewestBuild:
		return "newest-build"
	case StrategyAlphabetical:
		return "alphabetical"
	case StrategyDigest:
		return "digest"
	}

	return "unknown"
}

// ConstraintMatchMode defines how the constraint should be matched
type ConstraintMatchMode int

const (
	// ConstraintMatchSemVer uses semver to match a constraint
	ConstraintMatchSemver ConstraintMatchMode = 0
	// ConstraintMatchRegExp uses regexp to match a constraint
	ConstraintMatchRegExp ConstraintMatchMode = 1
	// ConstraintMatchNone does not enforce a constraint
	ConstraintMatchNone ConstraintMatchMode = 2
)

// VersionConstraint defines a constraint for comparing versions
type VersionConstraint struct {
	Constraint string
	MatchFunc  MatchFuncFn
	MatchArgs  interface{}
	IgnoreList []string
	Strategy   UpdateStrategy
	Options    *options.ManifestOptions
}

type MatchFuncFn func(tagName string, pattern interface{}) bool

// String returns the string representation of VersionConstraint
func (vc *VersionConstraint) String() string {
	return vc.Constraint
}

func NewVersionConstraint() *VersionConstraint {
	return &VersionConstraint{
		MatchFunc: MatchFuncNone,
		Strategy:  StrategySemVer,
		Options:   options.NewManifestOptions(),
	}
}

// GetNewestVersionFromTags returns the latest available version from a list of
// tags while optionally taking a semver constraint into account. Returns the
// original version if no new version could be found from the list of tags.
func (img *ContainerImage) GetNewestVersionFromTags(vc *VersionConstraint, tagList *tag.ImageTagList) (*tag.ImageTag, error) {
	logCtx := log.NewContext()
	logCtx.AddField("image", img.String())

	var availableTags tag.SortableImageTagList
	switch vc.Strategy {
	case StrategySemVer:
		availableTags = tagList.SortBySemVer()
	case StrategyAlphabetical:
		availableTags = tagList.SortAlphabetically()
	case StrategyNewestBuild:
		availableTags = tagList.SortByDate()
	case StrategyDigest:
		availableTags = tagList.SortAlphabetically()
	}

	considerTags := tag.SortableImageTagList{}

	// It makes no sense to proceed if we have no available tags
	if len(availableTags) == 0 {
		return img.ImageTag, nil
	}

	// The given constraint MUST match a semver constraint
	var semverConstraint *semver.Constraints
	var err error
	if vc.Strategy == StrategySemVer {
		// TODO: Shall we really ensure a valid semver on the current tag?
		// This prevents updating from a non-semver tag currently.
		if img.ImageTag != nil && img.ImageTag.TagName != "" {
			_, err := semver.NewVersion(img.ImageTag.TagName)
			if err != nil {
				return nil, err
			}
		}

		if vc.Constraint != "" {
			if vc.Strategy == StrategySemVer {
				semverConstraint, err = semver.NewConstraint(vc.Constraint)
				if err != nil {
					logCtx.Errorf("invalid constraint '%s' given: '%v'", vc, err)
					return nil, err
				}
			}
		}
	}

	// Loop through all tags to check whether it's an update candidate.
	for _, tag := range availableTags {
		logCtx.Tracef("Finding out whether to consider %s for being updateable", tag.TagName)

		if vc.Strategy == StrategySemVer {
			// Non-parseable tag does not mean error - just skip it
			ver, err := semver.NewVersion(tag.TagName)
			if err != nil {
				logCtx.Tracef("Not a valid version: %s", tag.TagName)
				continue
			}

			// If we have a version constraint, check image tag against it. If the
			// constraint is not satisfied, skip tag.
			if semverConstraint != nil {
				if !semverConstraint.Check(ver) {
					logCtx.Tracef("%s did not match constraint %s", ver.Original(), vc.Constraint)
					continue
				}
			}
		} else if vc.Strategy == StrategyDigest {
			if tag.TagName != vc.Constraint {
				logCtx.Tracef("%s did not match contraint %s", tag.TagName, vc.Constraint)
				continue
			}
		}

		// Append tag as update candidate
		considerTags = append(considerTags, tag)
	}

	logCtx.Debugf("found %d from %d tags eligible for consideration", len(considerTags), len(availableTags))

	// If we found tags to consider, return the most recent tag found according
	// to the update strategy.
	if len(considerTags) > 0 {
		return considerTags[len(considerTags)-1], nil
	}

	return nil, nil
}

// IsTagIgnored matches tag against the patterns in IgnoreList and returns true if one of them matches
func (vc *VersionConstraint) IsTagIgnored(tag string) bool {
	for _, t := range vc.IgnoreList {
		if match, err := filepath.Match(t, tag); err == nil && match {
			log.Tracef("tag %s is ignored by pattern %s", tag, t)
			return true
		}
	}
	return false
}

// IsCacheable returns true if we can safely cache tags for strategy s
func (s UpdateStrategy) IsCacheable() bool {
	switch s {
	case StrategyDigest:
		return false
	default:
		return true
	}
}

// NeedsMetadata returns true if strategy s requires image metadata to work correctly
func (s UpdateStrategy) NeedsMetadata() bool {
	switch s {
	case StrategyNewestBuild:
		return true
	default:
		return false
	}
}

// NeedsVersionConstraint returns true if strategy s requires a version constraint to be defined
func (s UpdateStrategy) NeedsVersionConstraint() bool {
	switch s {
	case StrategyDigest:
		return true
	default:
		return false
	}
}

// WantsOnlyConstraintTag returns true if strategy s only wants to inspect the tag specified by the constraint
func (s UpdateStrategy) WantsOnlyConstraintTag() bool {
	switch s {
	case StrategyDigest:
		return true
	default:
		return false
	}
}
