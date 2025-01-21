package helm

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/Masterminds/semver/v3"
)

type Entry struct {
	Version string
	Created time.Time
}

type Index struct {
	Entries map[string]Entries
}

func (i *Index) GetEntries(chart string) (Entries, error) {
	entries, ok := i.Entries[chart]
	if !ok {
		return nil, fmt.Errorf("chart '%s' not found in index", chart)
	}
	return entries, nil
}

type Entries []Entry

func (e Entries) MaxVersion(constraints *semver.Constraints) (*semver.Version, error) {
	versions := semver.Collection{}
	for _, entry := range e {
		v, err := semver.NewVersion(entry.Version)

		//Invalid semantic version ignored
		if err == semver.ErrInvalidSemVer {
			log.Debugf("Invalid sementic version: %s", entry.Version)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("invalid constraint in index: %v", err)
		}
		if constraints.Check(v) {
			versions = append(versions, v)
		}
	}
	if len(versions) == 0 {
		return nil, fmt.Errorf("constraint not found in index")
	}
	maxVersion := versions[0]
	for _, v := range versions {
		if v.GreaterThan(maxVersion) {
			maxVersion = v
		}
	}
	return maxVersion, nil
}
