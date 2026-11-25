package application

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/assets"
	"github.com/argoproj/argo-cd/v3/util/rbac"
)

const (
	// listSSOBenchmarkApps and listSSOBenchmarkProjects match an instance where the Application List call was
	// profiled: 3,453 Applications in one namespace, spread over 365 AppProjects.
	listSSOBenchmarkApps     = 3453
	listSSOBenchmarkProjects = 365
	// listSSOBenchmarkUserProjects is the number of projects whose roles bind one of the user's groups.
	listSSOBenchmarkUserProjects = 10
)

func listSSOBenchmarkProjectName(i int) string { return fmt.Sprintf("proj-%04d", i) }

func listSSOBenchmarkGroupName(i int) string {
	return fmt.Sprintf("example-org/platform/team-%04d", i)
}

// listSSOBenchmarkFixture returns listSSOBenchmarkProjects projects, each with an admin, a developer and a read-only
// role bound to SSO groups, and listSSOBenchmarkApps applications spread evenly over them. Every project's roles hold
// 15 policy lines, close to the instance where the call was profiled (6,157 lines over 365 projects).
func listSSOBenchmarkFixture() []runtime.Object {
	objects := make([]runtime.Object, 0, listSSOBenchmarkProjects+listSSOBenchmarkApps)
	for i := range listSSOBenchmarkProjects {
		name := listSSOBenchmarkProjectName(i)
		group := listSSOBenchmarkGroupName(i)
		proj := &v1alpha1.AppProject{
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos:  []string{"*"},
				Destinations: []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				Roles: []v1alpha1.ProjectRole{
					{
						Name: "admin",
						Policies: []string{
							fmt.Sprintf("p, proj:%s:admin, applications, *, %s/*, allow", name, name),
							fmt.Sprintf("p, proj:%s:admin, logs, get, %s/*, allow", name, name),
							fmt.Sprintf("p, proj:%s:admin, exec, create, %s/*, allow", name, name),
						},
						Groups: []string{group + "/maintainers"},
					},
					{
						Name: "dev",
						Policies: []string{
							fmt.Sprintf("p, proj:%s:dev, applications, get, %s/*, allow", name, name),
							fmt.Sprintf("p, proj:%s:dev, applications, sync, %s/*, allow", name, name),
							fmt.Sprintf("p, proj:%s:dev, applications, action/*, %s/*, allow", name, name),
						},
						Groups: []string{group + "/developers"},
					},
					{
						Name: "readonly",
						Policies: []string{
							fmt.Sprintf("p, proj:%s:readonly, applications, get, %s/*, allow", name, name),
							fmt.Sprintf("p, proj:%s:readonly, logs, get, %s/*, allow", name, name),
						},
						Groups: []string{group},
					},
				},
			},
		}
		// Set after the literal: a promoted field in a struct literal needs Go 1.27, which release branches don't use.
		proj.Name = name
		proj.Namespace = testNamespace
		objects = append(objects, proj)
	}
	for i := range listSSOBenchmarkApps {
		objects = append(objects, newTestApp(func(app *v1alpha1.Application) {
			app.Name = fmt.Sprintf("app-%05d", i)
			app.Spec.Project = listSSOBenchmarkProjectName(i % listSSOBenchmarkProjects)
		}))
	}
	return objects
}

// listSSOBenchmarkClaims returns the claims of a Dex ID token for a user with userGroups SSO groups, decoded the
// way the API server decodes them: as a jwt.MapClaims value from JSON. The user is a developer in the first
// listSSOBenchmarkUserProjects projects; the other groups are not bound in any project, as happens when the
// connector passes every group of the user (for example GitLab without a groups filter).
func listSSOBenchmarkClaims(b *testing.B, userGroups int) jwt.MapClaims {
	b.Helper()
	groups := make([]string, 0, userGroups)
	for i := range listSSOBenchmarkUserProjects {
		groups = append(groups, listSSOBenchmarkGroupName(i)+"/developers")
	}
	for i := len(groups); i < userGroups; i++ {
		groups = append(groups, fmt.Sprintf("example-org/unrelated/group-%04d", i))
	}
	token := map[string]any{
		"iss":                "https://argocd.example.com/api/dex",
		"sub":                "CgY0MjQyNDISBmdpdGxhYg",
		"aud":                "argo-cd",
		"exp":                1893456000,
		"iat":                1893369600,
		"at_hash":            "dGhpcyBpcyBub3QgYSByZWFsIGhhc2g",
		"c_hash":             "YW5vdGhlciBmYWtlIGhhc2g",
		"email":              "alice@example.com",
		"email_verified":     true,
		"groups":             groups,
		"name":               "Alice Example",
		"preferred_username": "alice",
		"federated_claims":   map[string]any{"connector_id": "gitlab", "user_id": "424242"},
	}
	data, err := json.Marshal(token)
	require.NoError(b, err)
	var claims jwt.MapClaims
	require.NoError(b, json.Unmarshal(data, &claims))
	return claims
}

// benchmarkListAppsSSOUser measures an Application List call by an SSO user whose claims are in the request context
// as a jwt.MapClaims value, the type that the API server's authentication puts there.
//
// A cold call is the first one after the RBAC policy was loaded (API server start, policy change, or the hourly
// expiry of the enforcer cache). A warm call is answered from the enforcers' decision caches.
func benchmarkListAppsSSOUser(b *testing.B, userGroups int, cold bool) {
	b.Helper()
	// The informers log at info level while the server starts, which splits the benchmark's result lines.
	level := log.GetLevel()
	log.SetLevel(log.WarnLevel)
	b.Cleanup(func() { log.SetLevel(level) })
	appServer := newTestAppServerWithEnforcerConfigureWithBenchmark(b, func(enf *rbac.Enforcer) {
		require.NoError(b, enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV))
	}, listSSOBenchmarkFixture()...)

	//nolint:staticcheck
	ctx := context.WithValue(b.Context(), "claims", listSSOBenchmarkClaims(b, userGroups))
	visibleApps := 0
	for i := range listSSOBenchmarkApps {
		if i%listSSOBenchmarkProjects < listSSOBenchmarkUserProjects {
			visibleApps++
		}
	}

	list := func() {
		res, err := appServer.List(ctx, &application.ApplicationQuery{})
		require.NoError(b, err)
		require.Len(b, res.Items, visibleApps)
	}
	if !cold {
		list()
	}
	b.ReportAllocs()
	for b.Loop() {
		if cold {
			b.StopTimer()
			require.NoError(b, appServer.enf.SetUserPolicy(""))
			b.StartTimer()
		}
		list()
	}
}

func BenchmarkListAppsSSOUser(b *testing.B) {
	for _, userGroups := range []int{20, 200} {
		for _, state := range []string{"cold", "warm"} {
			b.Run(fmt.Sprintf("groups=%d/%s", userGroups, state), func(b *testing.B) {
				benchmarkListAppsSSOUser(b, userGroups, state == "cold")
			})
		}
	}
}
