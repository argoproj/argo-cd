package extension_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/extension"
	"github.com/argoproj/argo-cd/v2/server/extension/mocks"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRegisterHandlers(t *testing.T) {
	type fixture struct {
		settingsGetterMock *mocks.SettingsGetter
		manager            *extension.Manager
	}

	setup := func() *fixture {
		settMock := &mocks.SettingsGetter{}

		logger, _ := test.NewNullLogger()
		logEntry := logger.WithContext(context.Background())
		m := extension.NewManager(settMock, nil, logEntry)

		return &fixture{
			settingsGetterMock: settMock,
			manager:            m,
		}
	}
	t.Run("will register handlers successfully", func(t *testing.T) {
		// given
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

func TestExtensionsHandlers(t *testing.T) {
	type fixture struct {
		router             *mux.Router
		appGetterMock      *mocks.ApplicationGetter
		settingsGetterMock *mocks.SettingsGetter
		manager            *extension.Manager
	}

	setup := func() *fixture {
		appMock := &mocks.ApplicationGetter{}
		settMock := &mocks.SettingsGetter{}

		logger, _ := test.NewNullLogger()
		logEntry := logger.WithContext(context.Background())
		m := extension.NewManager(settMock, appMock, logEntry)

		router := mux.NewRouter()

		return &fixture{
			router:             router,
			appGetterMock:      appMock,
			settingsGetterMock: settMock,
			manager:            m,
		}
	}

	withExtensionConfig := func(configYaml string, f *fixture) {
		settings := &settings.ArgoCDSettings{
			ExtensionConfig: configYaml,
		}
		f.settingsGetterMock.On("Get", mock.Anything).Return(settings, nil)
	}

	startTestServer := func(t *testing.T, f *fixture) *httptest.Server {
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
	t.Run("proxy will return 404 if no extension endpoint is registered", func(t *testing.T) {
		// given
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
		f := setup()
		backendResponse := "some data"
		backendEndpoint := "some-backend"
		backendSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, backendResponse)
		}))
		defer backendSrv.Close()
		withExtensionConfig(getExtensionConfig(backendEndpoint, backendSrv.URL), f)
		ts := startTestServer(t, f)
		defer ts.Close()

		// when
		resp, err := http.Get(fmt.Sprintf("%s/extensions/%s/", ts.URL, backendEndpoint))

		// then
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, err := ioutil.ReadAll(resp.Body)
		require.NoError(t, err)
		actual := strings.TrimSuffix(string(body), "\n")
		assert.Equal(t, backendResponse, actual)
	})
	t.Run("will route requests with 2 backends for the same extension successfully", func(t *testing.T) {
		// given
		f := setup()
		extName := "some-extension"

		response1 := "response backend 1"
		cluster1 := "cluster1"
		beSrv1 := startBackendTestSrv(response1)
		defer beSrv1.Close()

		response2 := "response backend 2"
		cluster2 := "cluster2"
		beSrv2 := startBackendTestSrv(response2)
		defer beSrv2.Close()

		withExtensionConfig(getExtensionConfigWith2Backends(extName, beSrv1.URL, cluster1, beSrv2.URL, cluster2), f)
		ts := startTestServer(t, f)
		defer ts.Close()

		app1 := &v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				Destination: v1alpha1.ApplicationDestination{
					Server: beSrv1.URL,
					Name:   cluster1,
				},
			},
		}
		f.appGetterMock.On("Get", "ns1", "app1").Return(app1, nil)

		app2 := &v1alpha1.Application{
			Spec: v1alpha1.ApplicationSpec{
				Destination: v1alpha1.ApplicationDestination{
					Server: beSrv2.URL,
					Name:   cluster2,
				},
			},
		}
		f.appGetterMock.On("Get", "ns2", "app2").Return(app2, nil)

		url := fmt.Sprintf("%s/extensions/%s/", ts.URL, extName)
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			t.Fatalf("error creating request: %s", err)
		}
		req1 := req.Clone(context.Background())
		req1.Header.Add(extension.HeaderArgoCDApplicationName, "ns1/app1")
		req2 := req.Clone(context.Background())
		req2.Header.Add(extension.HeaderArgoCDApplicationName, "ns2/app2")

		// when
		resp1, err := http.DefaultClient.Do(req1)
		require.NoError(t, err)
		resp2, err := http.DefaultClient.Do(req2)
		require.NoError(t, err)

		// then
		require.NotNil(t, resp1)
		require.Equal(t, http.StatusOK, resp1.StatusCode)
		body, err := ioutil.ReadAll(resp1.Body)
		require.NoError(t, err)
		actual := strings.TrimSuffix(string(body), "\n")
		assert.Equal(t, response1, actual)

		require.NotNil(t, resp2)
		require.Equal(t, http.StatusOK, resp2.StatusCode)
		body, err = ioutil.ReadAll(resp2.Body)
		require.NoError(t, err)
		actual = strings.TrimSuffix(string(body), "\n")
		assert.Equal(t, response2, actual)
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

func getExtensionConfigWith2Backends(name, url1, clus1, url2, clus2 string) string {
	cfg := `
extensions:
- name: %s
  backend:
    services:
    - url: %s
      cluster: %s
    - url: %s
      cluster: %s
`
	return fmt.Sprintf(cfg, name, url1, clus1, url2, clus2)
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
