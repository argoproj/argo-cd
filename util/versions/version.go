package versions

import "github.com/Masterminds/semver/v3"

func IsVersion(text string) bool {
	_, err := semver.NewVersion(text)
	return err == nil
}

func IsConstraint(text string) bool {
	_, err := semver.NewConstraint(text)
	return !IsVersion(text) && err == nil
}
