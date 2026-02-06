package fixture

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/errors"
	kubeutil "github.com/argoproj/argo-cd/v3/util/kube"
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

func GetVersions(t *testing.T) *Versions {
	t.Helper()
	output := errors.NewHandler(t).FailOnErr(Run(".", "kubectl", "version", "-o", "json")).(string)
	version := &Versions{}
	require.NoError(t, json.Unmarshal([]byte(output), version))
	return version
}

func GetApiResources(t *testing.T) string { //nolint:revive //FIXME(var-naming)
	t.Helper()
	kubectl := kubeutil.NewKubectl()
	resources := errors.NewHandler(t).FailOnErr(kubectl.GetAPIResources(KubeConfig, false, cache.NewNoopSettings())).([]kube.APIResourceInfo)
	return strings.Join(argo.APIResourcesToStrings(resources, true), ",")
}
