package fixture

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"

	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/errors"
	kubeutil "github.com/argoproj/argo-cd/v2/util/kube"
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

func GetApiResources() string {
	kubectl := kubeutil.NewKubectl()
	resources := errors.FailOnErr(kubectl.GetAPIResources(KubeConfig, false, cache.NewNoopSettings())).([]kube.APIResourceInfo)
	return strings.Join(argo.APIResourcesToStrings(resources, true), ",")
}
