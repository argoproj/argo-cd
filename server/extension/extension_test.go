package extension_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/extension"
	"github.com/argoproj/argo-cd/v2/server/extension/mocks"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestValidateHeaders(t *testing.T) {
	t.Run("will build RequestResources successfully", func(t *testing.T) {
		// given
		r, err := http.NewRequest("Get", "http://null", nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDApplicationName, "namespace:app-name")
		r.Header.Add(extension.HeaderArgoCDProjectName, "project-name")

		// when
		rr, err := extension.ValidateHeaders(r)

		// then
		require.NoError(t, err)
		assert.NotNil(t, rr)
		assert.Equal(t, "namespace", rr.ApplicationNamespace)
		assert.Equal(t, "app-name", rr.ApplicationName)
		assert.Equal(t, "project-name", rr.ProjectName)
	})
	t.Run("will return error if application is malformatted", func(t *testing.T) {
		// given
		r, err := http.NewRequest("Get", "http://null", nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDApplicationName, "no-namespace")

		// when
		rr, err := extension.ValidateHeaders(r)

		// then
		assert.Error(t, err)
		assert.Nil(t, rr)
	})
	t.Run("will return error if application header is missing", func(t *testing.T) {
		// given
		r, err := http.NewRequest("Get", "http://null", nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDProjectName, "project-name")

		// when
		rr, err := extension.ValidateHeaders(r)

		// then
		assert.Error(t, err)
		assert.Nil(t, rr)
	})
	t.Run("will return error if project header is missing", func(t *testing.T) {
		// given
		r, err := http.NewRequest("Get", "http://null", nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDApplicationName, "namespace:app-name")

		// when
		rr, err := extension.ValidateHeaders(r)

		// then
		assert.Error(t, err)
		assert.Nil(t, rr)
	})
	t.Run("will return error if invalid namespace", func(t *testing.T) {
		// given
		r, err := http.NewRequest("Get", "http://null", nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDApplicationName, "bad%namespace:app-name")
		r.Header.Add(extension.HeaderArgoCDProjectName, "project-name")

		// when
		rr, err := extension.ValidateHeaders(r)

		// then
		assert.Error(t, err)
		assert.Nil(t, rr)
	})
	t.Run("will return error if invalid app name", func(t *testing.T) {
		// given
		r, err := http.NewRequest("Get", "http://null", nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDApplicationName, "namespace:bad@app")
		r.Header.Add(extension.HeaderArgoCDProjectName, "project-name")

		// when
		rr, err := extension.ValidateHeaders(r)

		// then
		assert.Error(t, err)
		assert.Nil(t, rr)
	})
	t.Run("will return error if invalid project name", func(t *testing.T) {
		// given
		r, err := http.NewRequest("Get", "http://null", nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDApplicationName, "namespace:app")
		r.Header.Add(extension.HeaderArgoCDProjectName, "bad^project")

		// when
		rr, err := extension.ValidateHeaders(r)

		// then
		assert.Error(t, err)
		assert.Nil(t, rr)
	})
}

func TestRegisterHandlers(t *testing.T) {
	type fixture struct {
		settingsGetterMock *mocks.SettingsGetter
		manager            *extension.Manager
	}

	setup := func() *fixture {
		settMock := &mocks.SettingsGetter{}

		logger, _ := test.NewNullLogger()
		logEntry := logger.WithContext(context.Background())
		m := extension.NewManager(logEntry, settMock, nil, nil, nil)

		return &fixture{
			settingsGetterMock: settMock,
			manager:            m,
		}
	}
	t.Run("will register handlers successfully", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		router := mux.NewRouter()
		settings := &settings.ArgoCDSettings{
			ExtensionConfig: getExtensionConfigString(),
		}
		f.settingsGetterMock.On("Get", mock.Anything).Return(settings, nil)
		expectedRegexRoutes := []string{
			"^/extensions/",
			"^/extensions/external-backend/",
			"^/extensions/some-backend/",
			"^/extensions/$"}

		// when
		err := f.manager.RegisterHandlers(router)

		// then
		require.NoError(t, err)
		walkFn := func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			pathRegex, err := route.GetPathRegexp()
			require.NoError(t, err)
			assert.Contains(t, expectedRegexRoutes, pathRegex)
			return nil
		}
		err = router.Walk(walkFn)
		assert.NoError(t, err)
	})
	t.Run("will return error if extension config is invalid", func(t *testing.T) {
		// given
		t.Parallel()
		type testCase struct {
			name       string
			configYaml string
		}
		cases := []testCase{
			{
				name:       "no config",
				configYaml: "",
			},
			{
				name:       "no name",
				configYaml: getExtensionConfigNoName(),
			},
			{
				name:       "no URL",
				configYaml: getExtensionConfigNoURL(),
			},
			{
				name:       "invalid name",
				configYaml: getExtensionConfigInvalidName(),
			},
		}

		// when
		for _, tc := range cases {
			tc := tc
			t.Run(tc.name, func(t *testing.T) {
				// given
				t.Parallel()
				f := setup()
				router := mux.NewRouter()
				settings := &settings.ArgoCDSettings{
					ExtensionConfig: tc.configYaml,
				}
				f.settingsGetterMock.On("Get", mock.Anything).Return(settings, nil)

				// when
				err := f.manager.RegisterHandlers(router)

				// then
				assert.Error(t, err)
			})
		}
	})
}

func TestExtensionsHandler(t *testing.T) {
	type fixture struct {
		router             *mux.Router
		appGetterMock      *mocks.ApplicationGetter
		settingsGetterMock *mocks.SettingsGetter
		rbacMock           *mocks.RbacEnforcer
		projMock           *mocks.ProjectGetter
		manager            *extension.Manager
	}
	defaultProjectName := "project-name"

	setup := func() *fixture {
		appMock := &mocks.ApplicationGetter{}
		settMock := &mocks.SettingsGetter{}
		rbacMock := &mocks.RbacEnforcer{}
		projMock := &mocks.ProjectGetter{}

		logger, _ := test.NewNullLogger()
		logEntry := logger.WithContext(context.Background())
		m := extension.NewManager(logEntry, settMock, appMock, projMock, rbacMock)

		router := mux.NewRouter()

		return &fixture{
			router:             router,
			appGetterMock:      appMock,
			settingsGetterMock: settMock,
			rbacMock:           rbacMock,
			projMock:           projMock,
			manager:            m,
		}
	}

	getApp := func(destName, destServer, projName string) *v1alpha1.Application {
		return &v1alpha1.Application{
			TypeMeta:   v1.TypeMeta{},
			ObjectMeta: v1.ObjectMeta{},
			Spec: v1alpha1.ApplicationSpec{
				Destination: v1alpha1.ApplicationDestination{
					Name:   destName,
					Server: destServer,
				},
				Project: projName,
			},
			Status: v1alpha1.ApplicationStatus{
				Resources: []v1alpha1.ResourceStatus{
					{
						Group:     "apps",
						Version:   "v1",
						Kind:      "Pod",
						Namespace: "default",
						Name:      "some-pod",
					},
				},
			},
		}
	}

	getProjectWithDestinations := func(prjName string, destNames []string, destURLs []string) *v1alpha1.AppProject {
		destinations := []v1alpha1.ApplicationDestination{}
		for _, destName := range destNames {
			destination := v1alpha1.ApplicationDestination{
				Name: destName,
			}
			destinations = append(destinations, destination)
		}
		for _, destURL := range destURLs {
			destination := v1alpha1.ApplicationDestination{
				Server: destURL,
			}
			destinations = append(destinations, destination)
		}
		return &v1alpha1.AppProject{
			ObjectMeta: v1.ObjectMeta{
				Name: prjName,
			},
			Spec: v1alpha1.AppProjectSpec{
				Destinations: destinations,
			},
		}
	}

	withProject := func(prj *v1alpha1.AppProject, f *fixture) {
		f.projMock.On("Get", prj.GetName()).Return(prj, nil)
	}

	withRbac := func(f *fixture, allowApp, allowExt bool) {
		var appAccessError error
		var extAccessError error
		if !allowApp {
			appAccessError = errors.New("no app permission")
		}
		if !allowExt {
			extAccessError = errors.New("no extension permission")
		}
		f.rbacMock.On("EnforceErr", mock.Anything, rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, mock.Anything).Return(appAccessError)
		f.rbacMock.On("EnforceErr", mock.Anything, rbacpolicy.ResourceExtensions, rbacpolicy.ActionInvoke, mock.Anything).Return(extAccessError)
	}

	withExtensionConfig := func(configYaml string, f *fixture) {
		settings := &settings.ArgoCDSettings{
			ExtensionConfig: configYaml,
		}
		f.settingsGetterMock.On("Get", mock.Anything).Return(settings, nil)
	}

	startTestServer := func(t *testing.T, f *fixture) *httptest.Server {
		t.Helper()
		err := f.manager.RegisterHandlers(f.router)
		if err != nil {
			t.Fatalf("error starting test server: %s", err)
		}
		return httptest.NewServer(f.router)
	}

	startBackendTestSrv := func(response string) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, response)
		}))

	}
	newExtensionRequest := func(t *testing.T, method, url string) *http.Request {
		t.Helper()
		r, err := http.NewRequest(method, url, nil)
		if err != nil {
			t.Fatalf("error initializing request: %s", err)
		}
		r.Header.Add(extension.HeaderArgoCDApplicationName, "namespace:app-name")
		r.Header.Add(extension.HeaderArgoCDProjectName, defaultProjectName)
		return r
	}

	t.Run("proxy will return 404 if no extension endpoint is registered", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		withExtensionConfig(getExtensionConfigString(), f)
		ts := startTestServer(t, f)
		defer ts.Close()
		nonRegisteredEndpoint := "non-registered"

		// when
		resp, err := http.Get(fmt.Sprintf("%s/extensions/%s/", ts.URL, nonRegisteredEndpoint))

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
	t.Run("will call extension backend successfully", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		backendResponse := "some data"
		backendEndpoint := "some-backend"
		clusterName := "clusterName"
		clusterURL := "clusterURL"
		backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, backendResponse)
		}))
		defer backendSrv.Close()
		withRbac(f, true, true)
		withExtensionConfig(getExtensionConfig(backendEndpoint, backendSrv.URL), f)
		ts := startTestServer(t, f)
		defer ts.Close()
		r := newExtensionRequest(t, "Get", fmt.Sprintf("%s/extensions/%s/", ts.URL, backendEndpoint))
		app := getApp(clusterName, clusterURL, defaultProjectName)
		proj := getProjectWithDestinations("project-name", nil, []string{clusterURL})
		f.appGetterMock.On("Get", mock.Anything, mock.Anything).Return(app, nil)
		withProject(proj, f)

		// when
		resp, err := http.DefaultClient.Do(r)

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		actual := strings.TrimSuffix(string(body), "\n")
		assert.Equal(t, backendResponse, actual)
	})
	t.Run("will route requests with 2 backends for the same extension successfully", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		extName := "some-extension"

		response1 := "response backend 1"
		cluster1Name := "cluster1"
		beSrv1 := startBackendTestSrv(response1)
		defer beSrv1.Close()

		response2 := "response backend 2"
		cluster2URL := "cluster2"
		beSrv2 := startBackendTestSrv(response2)
		defer beSrv2.Close()

		f.appGetterMock.On("Get", "ns1", "app1").Return(getApp(cluster1Name, "", defaultProjectName), nil)
		f.appGetterMock.On("Get", "ns2", "app2").Return(getApp("", cluster2URL, defaultProjectName), nil)

		withRbac(f, true, true)
		withExtensionConfig(getExtensionConfigWith2Backends(extName, beSrv1.URL, cluster1Name, beSrv2.URL, cluster2URL), f)
		withProject(getProjectWithDestinations("project-name", []string{cluster1Name}, []string{cluster2URL}), f)

		ts := startTestServer(t, f)
		defer ts.Close()

		url := fmt.Sprintf("%s/extensions/%s/", ts.URL, extName)
		req := newExtensionRequest(t, http.MethodGet, url)
		req.Header.Del(extension.HeaderArgoCDApplicationName)

		req1 := req.Clone(context.Background())
		req1.Header.Add(extension.HeaderArgoCDApplicationName, "ns1:app1")
		req2 := req.Clone(context.Background())
		req2.Header.Add(extension.HeaderArgoCDApplicationName, "ns2:app2")

		// when
		resp1, err := http.DefaultClient.Do(req1)
		require.NoError(t, err)
		resp2, err := http.DefaultClient.Do(req2)
		require.NoError(t, err)

		// then
		require.NotNil(t, resp1)
		assert.Equal(t, http.StatusOK, resp1.StatusCode)
		body, err := io.ReadAll(resp1.Body)
		require.NoError(t, err)
		actual := strings.TrimSuffix(string(body), "\n")
		assert.Equal(t, response1, actual)

		require.NotNil(t, resp2)
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
		body, err = io.ReadAll(resp2.Body)
		require.NoError(t, err)
		actual = strings.TrimSuffix(string(body), "\n")
		assert.Equal(t, response2, actual)
	})
	t.Run("will return 401 if sub has no access to get application", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		allowApp := false
		allowExtension := true
		extName := "some-extension"
		withRbac(f, allowApp, allowExtension)
		withExtensionConfig(getExtensionConfig(extName, "http://fake"), f)
		ts := startTestServer(t, f)
		defer ts.Close()
		r := newExtensionRequest(t, "Get", fmt.Sprintf("%s/extensions/%s/", ts.URL, extName))
		f.appGetterMock.On("Get", mock.Anything, mock.Anything).Return(getApp("", "", defaultProjectName), nil)

		// when
		resp, err := http.DefaultClient.Do(r)

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
	t.Run("will return 401 if sub has no access to invoke extension", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		allowApp := true
		allowExtension := false
		extName := "some-extension"
		withRbac(f, allowApp, allowExtension)
		withExtensionConfig(getExtensionConfig(extName, "http://fake"), f)
		ts := startTestServer(t, f)
		defer ts.Close()
		r := newExtensionRequest(t, "Get", fmt.Sprintf("%s/extensions/%s/", ts.URL, extName))
		f.appGetterMock.On("Get", mock.Anything, mock.Anything).Return(getApp("", "", defaultProjectName), nil)

		// when
		resp, err := http.DefaultClient.Do(r)

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
	t.Run("will return 401 if project has no access to target cluster", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		allowApp := true
		allowExtension := true
		extName := "some-extension"
		noCluster := []string{}
		withRbac(f, allowApp, allowExtension)
		withExtensionConfig(getExtensionConfig(extName, "http://fake"), f)
		ts := startTestServer(t, f)
		defer ts.Close()
		r := newExtensionRequest(t, "Get", fmt.Sprintf("%s/extensions/%s/", ts.URL, extName))
		f.appGetterMock.On("Get", mock.Anything, mock.Anything).Return(getApp("", "", defaultProjectName), nil)
		proj := getProjectWithDestinations("project-name", nil, noCluster)
		withProject(proj, f)

		// when
		resp, err := http.DefaultClient.Do(r)

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
	t.Run("will return 401 if project in application does not exist", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		allowApp := true
		allowExtension := true
		extName := "some-extension"
		withRbac(f, allowApp, allowExtension)
		withExtensionConfig(getExtensionConfig(extName, "http://fake"), f)
		ts := startTestServer(t, f)
		defer ts.Close()
		r := newExtensionRequest(t, "Get", fmt.Sprintf("%s/extensions/%s/", ts.URL, extName))
		f.appGetterMock.On("Get", mock.Anything, mock.Anything).Return(getApp("", "", defaultProjectName), nil)
		f.projMock.On("Get", defaultProjectName).Return(nil, nil)

		// when
		resp, err := http.DefaultClient.Do(r)

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
	t.Run("will return 401 if project in application does not match with header", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		allowApp := true
		allowExtension := true
		extName := "some-extension"
		differentProject := "differentProject"
		withRbac(f, allowApp, allowExtension)
		withExtensionConfig(getExtensionConfig(extName, "http://fake"), f)
		ts := startTestServer(t, f)
		defer ts.Close()
		r := newExtensionRequest(t, "Get", fmt.Sprintf("%s/extensions/%s/", ts.URL, extName))
		f.appGetterMock.On("Get", mock.Anything, mock.Anything).Return(getApp("", "", differentProject), nil)

		// when
		resp, err := http.DefaultClient.Do(r)

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
	t.Run("will return 400 if application defines name and server destination", func(t *testing.T) {
		// This test is to validate a security risk with malicious application
		// trying to gain access to execute extensions in clusters it doesn't
		// have access.

		// given
		t.Parallel()
		f := setup()
		extName := "some-extension"
		maliciousName := "srv1"
		destinationServer := "some-valid-server"

		f.appGetterMock.On("Get", "ns1", "app1").Return(getApp(maliciousName, destinationServer, defaultProjectName), nil)

		withRbac(f, true, true)
		withExtensionConfig(getExtensionConfigWith2Backends(extName, "url1", "clusterName", "url2", "clusterURL"), f)
		withProject(getProjectWithDestinations("project-name", nil, []string{"srv1", destinationServer}), f)

		ts := startTestServer(t, f)
		defer ts.Close()

		url := fmt.Sprintf("%s/extensions/%s/", ts.URL, extName)
		req := newExtensionRequest(t, http.MethodGet, url)
		req.Header.Del(extension.HeaderArgoCDApplicationName)
		req1 := req.Clone(context.Background())
		req1.Header.Add(extension.HeaderArgoCDApplicationName, "ns1:app1")

		// when
		resp1, err := http.DefaultClient.Do(req1)
		require.NoError(t, err)

		// then
		require.NotNil(t, resp1)
		assert.Equal(t, http.StatusBadRequest, resp1.StatusCode)
		body, err := io.ReadAll(resp1.Body)
		require.NoError(t, err)
		actual := strings.TrimSuffix(string(body), "\n")
		assert.Equal(t, "invalid extension", actual)
	})
}

func getExtensionConfig(name, url string) string {
	cfg := `
extensions:
- name: %s
  backend:
    services:
    - url: %s
`
	return fmt.Sprintf(cfg, name, url)
}

func getExtensionConfigWith2Backends(name, url1, clusName, url2, clusURL string) string {
	cfg := `
extensions:
- name: %s
  backend:
    services:
    - url: %s
      cluster:
        name: %s
    - url: %s
      cluster:
        server: %s
`
	// second extension is configured with the cluster url rather
	// than the cluster name so we can validate that both use-cases
	// are working
	return fmt.Sprintf(cfg, name, url1, clusName, url2, clusURL)
}

func getExtensionConfigString() string {
	return `
extensions:
- name: external-backend
  backend:
    services:
    - url: https://httpbin.org
- name: some-backend
  backend:
    services:
    - url: http://localhost:7777
`
}

func getExtensionConfigNoName() string {
	return `
extensions:
- backend:
    services:
    - url: https://httpbin.org
`
}
func getExtensionConfigInvalidName() string {
	return `
extensions:
- name: invalid/name
  backend:
    services:
    - url: https://httpbin.org
`
}

func getExtensionConfigNoURL() string {
	return `
extensions:
- name: some-backend
  backend:
    services:
    - cluster: some-cluster
`
}
