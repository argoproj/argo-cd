package helm

import "github.com/Masterminds/semver/v3"

func IsVersion(text string) bool {
	_, err := semver.NewVersion(text)
	return err == nil
}
