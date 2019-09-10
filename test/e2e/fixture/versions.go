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

func (s Version) String() string {
	return fmt.Sprintf("v%s.%s.0", s.Major, s.Minor)
}

func GetVersions() *Versions {
	output := errors.FailOnErr(Run(".", "kubectl", "version", "-o", "json")).(string)
	version := &Versions{}
	errors.CheckError(json.Unmarshal([]byte(output), version))
	return version
}
