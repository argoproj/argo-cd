package application

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/security"
)

func TestStartProcessWithShells(t *testing.T) {
	t.Setenv("KUBECTL_REMOTE_COMMAND_WEBSOCKETS", "true")

	t.Run("does not start a canceled session", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		err := startProcessWithShells(ctx, nil, nil, "default", "pod", "container", []string{"bash", "sh"}, nil)
		require.ErrorIs(t, err, context.Canceled)
	})

	for _, testCase := range []struct {
		name            string
		cancelOnRequest bool
	}{
		{name: "does not fall back after transport failure"},
		{name: "stops after session cancellation", cancelOnRequest: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			var bashRequests atomic.Int32
			var shRequests atomic.Int32
			streamStarted := make(chan struct{}, 1)
			wsUpgrader := websocket.Upgrader{
				Subprotocols: []string{"v5.channel.k8s.io"},
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				commands := r.URL.Query()["command"]
				if len(commands) > 0 {
					switch commands[0] {
					case "bash":
						bashRequests.Add(1)
					case "sh":
						shRequests.Add(1)
					}
				}
				if testCase.cancelOnRequest {
					conn, err := wsUpgrader.Upgrade(w, r, nil)
					if err != nil {
						return
					}
					defer conn.Close()
					for {
						if _, _, err := conn.ReadMessage(); err != nil {
							return
						}
					}
				}
				http.Error(w, "exec failed", http.StatusInternalServerError)
			}))
			defer server.Close()
			defer server.CloseClientConnections()

			cfg := &rest.Config{Host: server.URL}
			client, err := kubernetes.NewForConfig(cfg)
			require.NoError(t, err)

			result := make(chan error, 1)
			go func() {
				result <- startProcessWithShells(ctx, client, cfg, "default", "pod", "container", []string{"bash", "sh"}, &blockingPtyHandler{ctx: ctx, started: streamStarted})
			}()
			if testCase.cancelOnRequest {
				select {
				case <-streamStarted:
				case <-time.After(5 * time.Second):
					t.Fatal("timed out waiting for the terminal stream to start")
				}
				cancel()
			}
			select {
			case err = <-result:
			case <-time.After(5 * time.Second):
				t.Fatal("terminal request did not stop after cancellation")
			}
			if testCase.cancelOnRequest {
				require.ErrorIs(t, err, context.Canceled)
			} else {
				require.Error(t, err)
			}
			assert.Positive(t, bashRequests.Load())
			assert.Zero(t, shRequests.Load())
		})
	}
}

func TestStartProcessWithShellsFallsBackToSH(t *testing.T) {
	t.Setenv("KUBECTL_REMOTE_COMMAND_WEBSOCKETS", "true")

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var bashRequests atomic.Int32
	var shRequests atomic.Int32
	wsUpgrader := websocket.Upgrader{
		Subprotocols: []string{"v5.channel.k8s.io", "v4.channel.k8s.io"},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commands := r.URL.Query()["command"]
		if len(commands) == 0 {
			http.Error(w, "missing command", http.StatusBadRequest)
			return
		}
		switch commands[0] {
		case "bash":
			bashRequests.Add(1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(&metav1.Status{
				APIVersion: "v1",
				Kind:       "Status",
				Status:     metav1.StatusFailure,
				Reason:     metav1.StatusReasonInternalError,
				Code:       http.StatusInternalServerError,
				Message:    `Internal error occurred: error executing command in container: failed to exec in container: OCI runtime exec failed: exec failed: unable to start container process: exec: "bash": executable file not found in $PATH: unknown`,
			})
		case "sh":
			shRequests.Add(1)
			conn, err := wsUpgrader.Upgrade(w, r, nil)
			if err != nil {
				return
			}
			defer conn.Close()
			statusBytes, err := json.Marshal(&metav1.Status{
				APIVersion: "v1",
				Kind:       "Status",
				Status:     metav1.StatusSuccess,
			})
			if err != nil {
				return
			}
			_ = conn.WriteMessage(websocket.BinaryMessage, append([]byte{3}, statusBytes...))
			_ = conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		}
	}))
	defer server.Close()

	cfg := &rest.Config{Host: server.URL}
	client, err := kubernetes.NewForConfig(cfg)
	require.NoError(t, err)

	pty := &stubPtyHandler{}
	err = startProcessWithShells(ctx, client, cfg, "default", "pod", "container", []string{"bash", "sh"}, pty)
	require.NoError(t, err)
	assert.Positive(t, bashRequests.Load())
	assert.Equal(t, int32(1), shRequests.Load())
}

type blockingPtyHandler struct {
	ctx     context.Context
	started chan<- struct{}
}

func (h *blockingPtyHandler) Read([]byte) (int, error) {
	select {
	case h.started <- struct{}{}:
	default:
	}
	<-h.ctx.Done()
	return 0, h.ctx.Err()
}

func (h *blockingPtyHandler) Write(p []byte) (int, error) {
	return len(p), nil
}
func (h *blockingPtyHandler) Next() *remotecommand.TerminalSize { return nil }
func (h *blockingPtyHandler) Done()                             {}

type stubPtyHandler struct{}

func (s *stubPtyHandler) Read([]byte) (int, error) { return 0, io.EOF }
func (s *stubPtyHandler) Write(p []byte) (int, error) {
	return len(p), nil
}
func (s *stubPtyHandler) Next() *remotecommand.TerminalSize { return nil }
func (s *stubPtyHandler) Done()                             {}

func TestIsShellInitializationError(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name     string
		err      error
		expected bool
	}{
		{name: "command not found", err: utilexec.CodeExitError{Err: errors.New("not found"), Code: 127}, expected: true},
		{name: "command not executable", err: utilexec.CodeExitError{Err: errors.New("not executable"), Code: 126}, expected: true},
		{name: "runtime executable not found", err: errors.New(`failed to exec in container: OCI runtime exec failed: exec: "bash": executable file not found in $PATH: unknown`), expected: true},
		{name: "runtime stat no such file", err: errors.New(`exec failed: unable to start container process: exec: "/bin/bash": stat /bin/bash: no such file or directory`), expected: true},
		{name: "command exited", err: utilexec.CodeExitError{Err: errors.New("exited"), Code: 1}},
		{name: "unrelated no such file", err: errors.New("open config: no such file or directory")},
		{name: "transport failure", err: errors.New("connection reset")},
		{name: "no error"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.expected, isShellInitializationError(testCase.err))
		})
	}
}

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
