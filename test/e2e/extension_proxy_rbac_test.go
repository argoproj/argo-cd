package e2e

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/server/extension"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/account"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// Security property (GHSA-g3ff-q88g-chrj): proxy extension authorize must evaluate
// Application get RBAC with the control-plane namespace as RBACName defaultNS. When an
// Application lives outside that namespace, subjects who only have the 2-segment
// project/name permission must be denied; granting the 3-segment
// project/namespace/name permission must allow the call.
func TestExtensionProxyRequiresNamespacedApplicationRBAC(t *testing.T) {
	var backendHits atomic.Int32
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		backendHits.Add(1)
		_, _ = io.WriteString(w, "ok")
	}))
	t.Cleanup(backend.Close)

	extName := "e2e-proxy-rbac"
	appCtx := Given(t)
	require.NoError(t, fixture.SetParamInSettingConfigMap("extension.config", fmt.Sprintf(`
extensions:
- name: %s
  backend:
    services:
    - url: %s
`, extName, backend.URL)))

	appCtx.
		Path(guestbookPath).
		SetAppNamespace(fixture.AppNamespace()).
		When().
		CreateApp().
		Then().
		And(func(app *Application) {
			assert.Equal(t, fixture.AppNamespace(), app.Namespace)
		})

	// Use exact object scopes. Glob default/* also matches the 3-segment
	// project/namespace/name form, which would hide the RBACName collapse bug.
	twoSegmentScope := "default/" + appCtx.AppName()
	threeSegmentScope := "default/" + fixture.AppNamespace() + "/" + appCtx.AppName()

	accountCtx := accountFixture.GivenWithSameState(appCtx)
	accountCtx.Name("ext-rbac-user").
		When().
		Create().
		SetPermissions([]fixture.ACL{
			{Resource: "applications", Action: "get", Scope: twoSegmentScope},
			{Resource: "extensions", Action: "invoke", Scope: extName},
		}, "ext-two-segment").
		Login()

	userToken := fixture.GetToken()
	headers := map[string]string{
		extension.HeaderArgoCDApplicationName: fixture.AppNamespace() + ":" + appCtx.AppName(),
		extension.HeaderArgoCDProjectName:     "default",
	}
	path := fmt.Sprintf("/extensions/%s/", extName)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := fixture.DoHttpRequestWithToken(userToken, http.MethodGet, path, "", headers)
		require.NoError(c, err)
		require.NotNil(c, resp)
		defer resp.Body.Close()
		// Fixed behavior: 2-segment Application get must not authorize apps-in-any-namespace.
		assert.Equal(c, http.StatusUnauthorized, resp.StatusCode)
	}, 30*time.Second, time.Second)

	assert.Equal(t, int32(0), backendHits.Load(), "backend must not be reached when Application get is denied")

	require.NoError(t, fixture.SetPermissions([]fixture.ACL{
		{Resource: "applications", Action: "get", Scope: threeSegmentScope},
		{Resource: "extensions", Action: "invoke", Scope: extName},
	}, accountCtx.GetName(), "ext-three-segment"))
	require.NoError(t, fixture.LoginAs(accountCtx.GetName()))
	userToken = fixture.GetToken()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		resp, err := fixture.DoHttpRequestWithToken(userToken, http.MethodGet, path, "", headers)
		require.NoError(c, err)
		require.NotNil(c, resp)
		defer resp.Body.Close()
		assert.Equal(c, http.StatusOK, resp.StatusCode)
	}, 30*time.Second, time.Second)

	assert.Positive(t, backendHits.Load(), "backend should be reached once namespaced Application get is granted")
}
