package fixture

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/version"

	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/errors"
	kubeutil "github.com/argoproj/argo-cd/v3/util/kube"
)

type Versions struct {
	ServerVersion Version
}

type Version struct {
	Major, Minor, GitVersion string
}

func (v Version) String() string {
	return "v" + version.MustParseGeneric(v.GitVersion).String()
}

func GetVersions(t *testing.T) *Versions {
	t.Helper()
	output := errors.NewHandler(t).FailOnErr(Run(".", "kubectl", "version", "-o", "json")).(string)
	versions := &Versions{}
	require.NoError(t, json.Unmarshal([]byte(output), versions))
	return versions
}

func GetApiResources(t *testing.T) string { //nolint:revive //FIXME(var-naming)
	t.Helper()
	kubectl := kubeutil.NewKubectl()
	resources := errors.NewHandler(t).FailOnErr(kubectl.GetAPIResources(KubeConfig, false, cache.NewNoopSettings())).([]kube.APIResourceInfo)
	return strings.Join(argo.APIResourcesToStrings(resources, true), ",")
}
