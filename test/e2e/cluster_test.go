package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/account"
	clusterFixture "github.com/argoproj/argo-cd/v2/test/e2e/fixture/cluster"
	. "github.com/argoproj/argo-cd/v2/util/errors"
)

func TestClusterList(t *testing.T) {
	SkipIfAlreadyRun(t)
	defer RecordTestRun(t)

	clusterFixture.
		Given(t).
		Project(ProjectName).
		When().
		List().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, fmt.Sprintf(`SERVER                          NAME        VERSION  STATUS      MESSAGE  PROJECT
https://kubernetes.default.svc  in-cluster  %v     Successful           `, GetVersions().ServerVersion), output)
		})
}

func TestClusterAdd(t *testing.T) {
	clusterFixture.
		Given(t).
		Project(ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		List().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, fmt.Sprintf(`SERVER                          NAME              VERSION  STATUS      MESSAGE  PROJECT
https://kubernetes.default.svc  test-cluster-add  %v     Successful           %s`, GetVersions().ServerVersion, ProjectName), output)
		})
}

func TestClusterAddPermissionDenied(t *testing.T) {
	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{}, "org-admin")

	clusterFixture.
		GivenWithSameState(t).
		Project(ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		IgnoreErrors().
		Create().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.True(t, strings.Contains(err.Error(), "PermissionDenied desc = permission denied: clusters, create"))
		})
}

func TestClusterAddAllowed(t *testing.T) {
	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "clusters",
				Action:   "create",
				Scope:    ProjectName + "/*",
			},
			{
				Resource: "clusters",
				Action:   "get",
				Scope:    ProjectName + "/*",
			},
		}, "org-admin")

	clusterFixture.
		GivenWithSameState(t).
		Project(ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		List().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, fmt.Sprintf(`SERVER                          NAME                      VERSION  STATUS      MESSAGE  PROJECT
https://kubernetes.default.svc  test-cluster-add-allowed  %v     Successful           argo-project`, GetVersions().ServerVersion), output)
		})
}

func TestClusterListDenied(t *testing.T) {
	accountFixture.Given(t).
		Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "clusters",
				Action:   "create",
				Scope:    ProjectName + "/*",
			},
		}, "org-admin")

	clusterFixture.
		GivenWithSameState(t).
		Project(ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		List().
		Then().
		AndCLIOutput(func(output string, err error) {
			assert.Equal(t, output, "SERVER  NAME  VERSION  STATUS  MESSAGE  PROJECT")
		})
}

func TestClusterGet(t *testing.T) {
	SkipIfAlreadyRun(t)
	EnsureCleanState(t)
	defer RecordTestRun(t)
	output := FailOnErr(RunCli("cluster", "get", "https://kubernetes.default.svc")).(string)

	assert.Contains(t, output, "name: in-cluster")
	assert.Contains(t, output, "server: https://kubernetes.default.svc")
	assert.Contains(t, output, fmt.Sprintf(`serverVersion: "%v"`, GetVersions().ServerVersion))
	assert.Contains(t, output, `config:
  tlsClientConfig:
    insecure: false`)

	assert.Contains(t, output, `status: Successful`)
}
