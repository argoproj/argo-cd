package helm

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/Masterminds/semver/v3"
)

type TagsList struct {
	Tags []string
}

func (t TagsList) MaxVersion(constraints *semver.Constraints) (*semver.Version, error) {
	versions := semver.Collection{}
	for _, tag := range t.Tags {
		v, err := semver.NewVersion(tag)

		// Invalid semantic version ignored
		if errors.Is(err, semver.ErrInvalidSemVer) {
			log.Debugf("Invalid semantic version: %s", tag)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("invalid constraint in tags: %w", err)
		}
		if constraints.Check(v) {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("constraint not found in %v tags", len(t.Tags))
	}
	maxVersion := versions[0]
	for _, v := range versions {
		if v.GreaterThan(maxVersion) {
			maxVersion = v
		}
	}
	return maxVersion, nil
}
