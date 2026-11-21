package application

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/assets"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/security"
)

func TestPodExists(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		podName        string
		namespace      string
		treeNodes      []appv1.ResourceNode
		expectedResult bool
	}{
		{
			name:           "empty tree nodes",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{},
			expectedResult: false,
		},
		{
			name:           "matched Pod but empty UID",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{Name: "test-pod", Namespace: "test", UID: "", Kind: kube.PodKind}},
			expectedResult: false,
		},
		{
			name:           "matched Pod",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{Name: "test-pod", Namespace: "test", UID: "testUID", Kind: kube.PodKind}},
			expectedResult: true,
		},
		{
			name:           "unmatched Pod Namespace",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{Name: "test-pod", Namespace: "test-A", UID: "testUID", Kind: kube.PodKind}},
			expectedResult: false,
		},
		{
			name:           "unmatched Kind",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{Name: "test-pod", Namespace: "test-A", UID: "testUID", Kind: kube.DeploymentKind}},
			expectedResult: false,
		},
		{
			name:           "unmatched Group",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{Name: "test-pod", Namespace: "test", UID: "testUID", Group: "A", Kind: kube.PodKind}},
			expectedResult: false,
		},
		{
			name:           "unmatched Pod Name",
			podName:        "test-pod",
			namespace:      "test",
			treeNodes:      []appv1.ResourceNode{{Name: "test", Namespace: "test", UID: "testUID", Kind: kube.PodKind}},
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := podExists(tcase.treeNodes, tcase.podName, tcase.namespace)
			assert.Equalf(t, tcase.expectedResult, result, "Expected result %v, but got %v", tcase.expectedResult, result)
		})
	}
}

func TestContainerRunning(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		pod            *corev1.Pod
		containerName  string
		expectedResult bool
	}{
		{
			name:           "empty container",
			pod:            &corev1.Pod{},
			containerName:  "",
			expectedResult: false,
		},
		{
			name:           "container not found",
			pod:            &corev1.Pod{},
			containerName:  "not-found",
			expectedResult: false,
		},
		{
			name: "container running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.NewTime(time.Now()),
								},
							},
						},
					},
				},
			},
			containerName:  "test",
			expectedResult: true,
		},
		{
			name: "init container running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.NewTime(time.Now()),
								},
							},
						},
					},
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test-init",
							State: corev1.ContainerState{
								Running: &corev1.ContainerStateRunning{
									StartedAt: metav1.NewTime(time.Now()),
								},
							},
						},
					},
				},
			},
			containerName:  "test-init",
			expectedResult: true,
		},
		{
			name: "container not running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					ContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test",
							State: corev1.ContainerState{
								Running: nil,
							},
						},
					},
				},
			},
			containerName:  "test",
			expectedResult: false,
		},
		{
			name: "init container not running",
			pod: &corev1.Pod{
				Status: corev1.PodStatus{
					InitContainerStatuses: []corev1.ContainerStatus{
						{
							Name: "test-init",
							State: corev1.ContainerState{
								Running: nil,
							},
						},
					},
				},
			},
			containerName:  "test-init",
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := containerRunning(tcase.pod, tcase.containerName)
			assert.Equalf(t, tcase.expectedResult, result, "Expected result %v, but got %v", tcase.expectedResult, result)
		})
	}
}

func TestIsValidPodName(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		resourceName   string
		expectedResult bool
	}{
		{
			name:           "valid pod name",
			resourceName:   "argocd-server-794644486d-r8v9d",
			expectedResult: true,
		},
		{
			name:           "not valid contains spaces",
			resourceName:   "kubectl delete pods",
			expectedResult: false,
		},
		{
			name:           "not valid",
			resourceName:   "kubectl -n kube-system delete pods --all",
			expectedResult: false,
		},
		{
			name:           "not valid contains special characters",
			resourceName:   "delete+*+from+etcd%3b",
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := argo.IsValidPodName(tcase.resourceName)
			assert.Equalf(t, tcase.expectedResult, result, "Expected result %v, but got %v", tcase.expectedResult, result)
		})
	}
}

func TestIsValidNamespaceName(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		resourceName   string
		expectedResult bool
	}{
		{
			name:           "valid pod namespace name",
			resourceName:   "argocd",
			expectedResult: true,
		},
		{
			name:           "not valid contains spaces",
			resourceName:   "kubectl delete ns argocd",
			expectedResult: false,
		},
		{
			name:           "not valid contains special characters",
			resourceName:   "delete+*+from+etcd%3b",
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := argo.IsValidNamespaceName(tcase.resourceName)
			assert.Equalf(t, tcase.expectedResult, result, "Expected result %v, but got %v", tcase.expectedResult, result)
		})
	}
}

func TestIsValidContainerNameName(t *testing.T) {
	for _, tcase := range []struct {
		name           string
		resourceName   string
		expectedResult bool
	}{
		{
			name:           "valid container name",
			resourceName:   "argocd-server",
			expectedResult: true,
		},
		{
			name:           "not valid contains spaces",
			resourceName:   "kubectl delete pods",
			expectedResult: false,
		},
		{
			name:           "not valid contains special characters",
			resourceName:   "delete+*+from+etcd%3b",
			expectedResult: false,
		},
	} {
		t.Run(tcase.name, func(t *testing.T) {
			result := argo.IsValidContainerName(tcase.resourceName)
			assert.Equalf(t, tcase.expectedResult, result, "Expected result %v, but got %v", tcase.expectedResult, result)
		})
	}
}

func TestTerminalHandler_ServeHTTP_empty_params(t *testing.T) {
	t.Parallel()

	testKeys := []string{
		"pod",
		"container",
		"app",
		"project",
		"namespace",
	}

	// test both empty and invalid
	testValues := []string{"", "invalid%20name"}

	for _, testKey := range testKeys {
		testKeyCopy := testKey

		for _, testValue := range testValues {
			testValueCopy := testValue

			t.Run(testKeyCopy+" "+testValueCopy, func(t *testing.T) {
				t.Parallel()

				handler := terminalHandler{}
				params := map[string]string{
					"pod":       "valid",
					"container": "valid",
					"app":       "valid",
					"project":   "valid",
					"namespace": "valid",
				}
				params[testKeyCopy] = testValueCopy
				var paramsArray []string
				for key, value := range params {
					paramsArray = append(paramsArray, key+"="+value)
				}
				paramsString := strings.Join(paramsArray, "&")
				request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/api/v1/terminal?"+paramsString, http.NoBody)
				recorder := httptest.NewRecorder()
				handler.ServeHTTP(recorder, request)
				response := recorder.Result()
				assert.Equal(t, http.StatusBadRequest, response.StatusCode)
			})
		}
	}
}

func TestTerminalHandler_ServeHTTP_disallowed_namespace(t *testing.T) {
	handler := terminalHandler{namespace: "argocd", enabledNamespaces: []string{"allowed"}}
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/api/v1/terminal?pod=valid&container=valid&appName=valid&projectName=valid&namespace=test&appNamespace=disallowed", http.NoBody)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	response := recorder.Result()
	assert.Equal(t, http.StatusForbidden, response.StatusCode)
	assert.Equal(t, security.NamespaceNotPermittedError("disallowed").Error()+"\n", recorder.Body.String())
}

func TestTerminalHandler_sourceIPFields(t *testing.T) {
	request := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "https://argocd.example.com/terminal", http.NoBody)
	request.RemoteAddr = "10.1.2.3:4711"
	request.Header.Add("X-Forwarded-For", "203.0.113.7")

	t.Run("disabled", func(t *testing.T) {
		handler := terminalHandler{terminalOptions: &TerminalOptions{}}
		assert.Empty(t, handler.sourceIPFields(request))
	})

	t.Run("enabled", func(t *testing.T) {
		handler := terminalHandler{terminalOptions: &TerminalOptions{
			EnableSourceIPLogging: true,
			TrustedProxies:        []netip.Prefix{netip.MustParsePrefix("10.0.0.0/8")},
		}}
		assert.Equal(t, log.Fields{"source.ip": "203.0.113.7", "forwarded.for": "203.0.113.7"}, handler.sourceIPFields(request))
	})
}

func TestGetApplicationClusterRawConfig(t *testing.T) {
	configureEnforcer := func(enf *rbac.Enforcer) {
		_ = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
		enf.SetDefaultRole("role:admin")
	}
	newHandler := func(t *testing.T, objects ...runtime.Object) *terminalHandler {
		t.Helper()
		appServer := newTestAppServerWithEnforcerConfigure(t, configureEnforcer,
			map[string]string{"application.sync.impersonation.enabled": "true"}, objects...)
		return &terminalHandler{
			projLister:  applisters.NewAppProjectLister(appServer.projInformer.GetIndexer()),
			db:          appServer.db,
			settingsMgr: appServer.settingsMgr,
			namespace:   appServer.ns,
		}
	}

	t.Run("impersonates matching service account", func(t *testing.T) {
		project := &appv1.AppProject{
			Name: "terminal-project", Namespace: testNamespace,
			Spec: appv1.AppProjectSpec{
				SourceRepos:  []string{"*"},
				Destinations: []appv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
				DestinationServiceAccounts: []appv1.ApplicationDestinationServiceAccount{{
					Server:                "https://cluster-api.example.com",
					Namespace:             test.FakeDestNamespace,
					DefaultServiceAccount: "terminal-sa",
				}},
			},
		}
		app := newTestApp(func(app *appv1.Application) {
			app.Spec.Project = project.Name
		})
		handler := newHandler(t, app, project)

		config, err := handler.getApplicationClusterRawConfig(t.Context(), app)
		require.NoError(t, err)
		assert.Equal(t, "system:serviceaccount:"+test.FakeDestNamespace+":terminal-sa", config.Impersonate.UserName)
	})

	t.Run("fails closed without matching service account", func(t *testing.T) {
		app := newTestApp()
		handler := newHandler(t, app)

		config, err := handler.getApplicationClusterRawConfig(t.Context(), app)
		assert.Nil(t, config)
		assert.ErrorContains(t, err, "no matching service account found")
	})

	t.Run("does not require a project when impersonation is disabled", func(t *testing.T) {
		app := newTestApp()
		appServer := newTestAppServerWithEnforcerConfigure(t, configureEnforcer, nil, app)
		handler := &terminalHandler{
			db:          appServer.db,
			settingsMgr: appServer.settingsMgr,
			namespace:   appServer.ns,
		}

		config, err := handler.getApplicationClusterRawConfig(t.Context(), app)
		require.NoError(t, err)
		assert.Empty(t, config.Impersonate.UserName)
	})
}

type terminalTestPty struct {
	bytes.Buffer
}

func (*terminalTestPty) Next() *remotecommand.TerminalSize {
	return nil
}

func TestTerminalKubernetesRequestsUseImpersonation(t *testing.T) {
	const userName = "system:serviceaccount:workloads:terminal-sa"
	t.Setenv(string(cmdutil.RemoteCommandWebsockets), "true")

	var lock sync.Mutex
	headers := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "pod-get"
		if strings.HasSuffix(r.URL.Path, "/exec") {
			key = r.Method
		}
		lock.Lock()
		headers[key] = r.Header.Get("Impersonate-User")
		lock.Unlock()
		http.Error(w, "upgrade rejected", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	config := &rest.Config{
		Host:        server.URL,
		Impersonate: rest.ImpersonationConfig{UserName: userName},
	}
	client, err := kubernetes.NewForConfig(config)
	require.NoError(t, err)
	_, err = client.CoreV1().Pods("workloads").Get(t.Context(), "pod", metav1.GetOptions{})
	require.Error(t, err)

	err = startProcess(t.Context(), client, config, "workloads", "pod", "container", []string{"sh"}, &terminalTestPty{})
	require.Error(t, err)

	lock.Lock()
	defer lock.Unlock()
	assert.Equal(t, userName, headers["pod-get"])
	assert.Equal(t, userName, headers[http.MethodGet])
	assert.Equal(t, userName, headers[http.MethodPost])
}
