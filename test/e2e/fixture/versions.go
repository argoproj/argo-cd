package fixture

import (
	"encoding/json"
	"fmt"

	"github.com/argoproj/argo-cd/errors"
)

type Versions struct {
	ServerVersion Version
}

type Version struct {
	Major, Minor string
}

func (v Version) String() string {
	return v.Format("%s.%s")
}

func (v Version) Format(format string) string {
	return fmt.Sprintf(format, v.Major, v.Minor)
}

func GetVersions() *Versions {
	output := errors.FailOnErr(Run(".", "kubectl", "version", "-o", "json")).(string)
	version := &Versions{}
	errors.CheckError(json.Unmarshal([]byte(output), version))
	return version
}
