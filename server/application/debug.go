package application

import (
	"context"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/httpstream"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	"github.com/argoproj/argo-cd/v3/util/security"
	util_session "github.com/argoproj/argo-cd/v3/util/session"
)

type debugHandler struct {
	appLister         applisters.ApplicationLister
	db                db.ArgoDB
	appResourceTreeFn func(ctx context.Context, app *appv1.Application) (*appv1.ApplicationTree, error)
	namespace         string
	enabledNamespaces []string
	sessionManager    *util_session.SessionManager
	debugOptions      *DebugOptions
}

type DebugOptions struct {
	DisableAuth bool
	Enf         *rbac.Enforcer
}

// NewDebugHandler returns a new debug handler.
func NewDebugHandler(appLister applisters.ApplicationLister, namespace string, enabledNamespaces []string, db db.ArgoDB, appResourceTree AppResourceTreeFn, sessionManager *util_session.SessionManager, debugOptions *DebugOptions) *debugHandler {
	return &debugHandler{
		appLister:         appLister,
		db:                db,
		appResourceTreeFn: appResourceTree,
		namespace:         namespace,
		enabledNamespaces: enabledNamespaces,
		sessionManager:    sessionManager,
		debugOptions:      debugOptions,
	}
}

func (s *debugHandler) getApplicationClusterRawConfig(ctx context.Context, a *appv1.Application) (*rest.Config, error) {
	destCluster, err := argo.GetDestinationCluster(ctx, a.Spec.Destination, s.db)
	if err != nil {
		return nil, err
	}
	rawConfig, err := destCluster.RawRestConfig()
	if err != nil {
		return nil, err
	}
	return rawConfig, nil
}

// WithFeatureFlagMiddleware is an HTTP middleware to verify if the debug
// feature is enabled before invoking the main handler
func (s *debugHandler) WithFeatureFlagMiddleware(getSettings GetSettingsFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := getSettings()
		if err != nil {
			log.Errorf("error executing WithFeatureFlagMiddleware: error getting settings: %s", err)
			http.Error(w, "Failed to get settings", http.StatusBadRequest)
			return
		}
		// Debug feature is enabled by default (no feature flag needed like exec)
		// This allows kubectl debug without additional configuration
		s.ServeHTTP(w, r)
	})
}

func (s *debugHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle health check requests
	if r.Method == "GET" && r.URL.Path == "/debug/health" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Debug endpoint is healthy"))
		return
	}

	// Only allow WebSocket upgrade requests for debug sessions
	if r.Header.Get("Upgrade") != "websocket" {
		http.Error(w, "WebSocket upgrade required", http.StatusBadRequest)
		return
	}

	q := r.URL.Query()

	podName := q.Get("pod")
	debugImage := q.Get("image")
	debugCommand := q.Get("command")
	shareProcesses := q.Get("shareProcesses") == "true"
	app := q.Get("appName")
	project := q.Get("projectName")
	namespace := q.Get("namespace")

	log.WithFields(log.Fields{
		"podName":        podName,
		"debugImage":     debugImage,
		"debugCommand":   debugCommand,
		"shareProcesses": shareProcesses,
		"app":            app,
		"project":        project,
		"namespace":      namespace,
		"url":            r.URL.String(),
	}).Info("Debug session request received")

	if podName == "" {
		log.Error("Pod name is required")
		http.Error(w, "Pod name is required", http.StatusBadRequest)
		return
	}

	if debugImage == "" {
		log.Error("Debug image is required")
		http.Error(w, "Debug image is required", http.StatusBadRequest)
		return
	}

	if debugCommand == "" {
		debugCommand = "sh" // Default command
	}

	if app == "" {
		log.Error("App name is required")
		http.Error(w, "App name is required", http.StatusBadRequest)
		return
	}

	if namespace == "" {
		log.Error("Namespace is required")
		http.Error(w, "Namespace is required", http.StatusBadRequest)
		return
	}

	if project == "" {
		log.Error("Project name is required")
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	fieldLog := log.WithFields(log.Fields{
		"application": app,
		"pod":         podName,
		"namespace":   namespace,
		"debugImage":  debugImage,
		"command":     debugCommand,
	})

	ctx := r.Context()

	// Get the application
	appRBACName := security.RBACName(s.namespace, project, app, "")
	a, err := s.appLister.Applications(s.namespace).Get(app)
	if err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, "Application not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get application resource tree
	tree, err := s.appResourceTreeFn(ctx, a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if pod exists in the tree
	if !podExists(tree.Nodes, podName, namespace) {
		http.Error(w, "Pod not found in application", http.StatusNotFound)
		return
	}

	// Get cluster config
	config, err := s.getApplicationClusterRawConfig(ctx, a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get the target pod to verify it exists and is running
	pod, err := kubeClientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		http.Error(w, "Cannot find pod", http.StatusBadRequest)
		return
	}

	if pod.Status.Phase != corev1.PodRunning {
		http.Error(w, "Pod not running", http.StatusBadRequest)
		return
	}

	fieldLog.Info("debug session starting")

	session, err := newDebugSession(ctx, w, r, s.sessionManager, appRBACName, s.debugOptions)
	if err != nil {
		http.Error(w, "Failed to start debug session", http.StatusBadRequest)
		return
	}
	defer session.Done()

	// send pings across the WebSocket channel at regular intervals to keep it alive through
	// load balancers which may close an idle connection after some period of time
	go session.StartKeepalives(time.Second * 5)

	// Only support ephemeral containers (no pod fallback)
	err = startDebugProcess(kubeClientset, config, namespace, podName, debugImage, debugCommand, shareProcesses, session)
	if err != nil {
		fieldLog.WithError(err).Error("Failed to start ephemeral debug container")
		http.Error(w, fmt.Sprintf("Failed to start debug container: %v", err), http.StatusBadRequest)
		session.Close()
		return
	}

	session.Close()
}

// DebugMessage is the struct for websocket message (same as TerminalMessage).
type DebugMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

// DebugCommand is the struct for websocket commands.
type DebugCommand struct {
	Code int
}

// startDebugProcess creates a debug container and connects it up with the ptyHandler (a session)
func startDebugProcess(k8sClient kubernetes.Interface, cfg *rest.Config, namespace, podName, debugImage, command string, shareProcesses bool, ptyHandler PtyHandler) error {
	log.WithFields(log.Fields{
		"namespace":      namespace,
		"podName":        podName,
		"debugImage":     debugImage,
		"command":        command,
		"shareProcesses": shareProcesses,
	}).Info("Starting debug process")

	// Create the debug container specification
	debugContainerName := fmt.Sprintf("debugger-%d", time.Now().Unix())
	debugContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    debugContainerName,
			Image:   debugImage,
			Command: []string{command},
			Stdin:   true,
			TTY:     true,
		},
	}

	// Set process namespace sharing if requested
	if shareProcesses {
		debugContainer.EphemeralContainerCommon.SecurityContext = &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{"SYS_PTRACE"},
			},
		}
		log.Info("Process namespace sharing enabled")
	}

	// Get the current pod
	pod, err := k8sClient.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		log.WithError(err).Error("Failed to get pod")
		return fmt.Errorf("failed to get pod: %v", err)
	}

	log.WithField("podPhase", pod.Status.Phase).Info("Retrieved pod")

	// Add the ephemeral container
	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, debugContainer)

	log.WithField("containerName", debugContainerName).Info("Adding ephemeral container")

	// Update the pod with the ephemeral container
	_, err = k8sClient.CoreV1().Pods(namespace).UpdateEphemeralContainers(context.Background(), podName, pod, metav1.UpdateOptions{})
	if err != nil {
		log.WithError(err).Error("Failed to create ephemeral container")
		return fmt.Errorf("failed to create ephemeral container: %v", err)
	}

	log.Info("Ephemeral container created successfully")

	// Wait for the ephemeral container to be ready
	log.Info("Waiting for ephemeral container to be ready...")
	for i := 0; i < 30; i++ { // Wait up to 30 seconds
		time.Sleep(1 * time.Second)
		updatedPod, err := k8sClient.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
		if err != nil {
			log.WithError(err).Warnf("Failed to get pod status (attempt %d/30)", i+1)
			continue
		}

		// Check if our ephemeral container is running
		for _, containerStatus := range updatedPod.Status.EphemeralContainerStatuses {
			if containerStatus.Name == debugContainerName {
				log.WithFields(log.Fields{
					"containerName": containerStatus.Name,
					"state":         containerStatus.State,
				}).Info("Ephemeral container status")

				if containerStatus.State.Running != nil {
					// Container is ready, now exec into it
					log.Info("Ephemeral container is running, starting exec session")
					return execIntoDebugContainer(k8sClient, cfg, namespace, podName, debugContainerName, ptyHandler)
				}

				if containerStatus.State.Waiting != nil {
					log.WithField("reason", containerStatus.State.Waiting.Reason).Info("Container is waiting")
				}

				if containerStatus.State.Terminated != nil {
					log.WithField("reason", containerStatus.State.Terminated.Reason).Error("Container terminated")
					return fmt.Errorf("debug container terminated: %s", containerStatus.State.Terminated.Reason)
				}
			}
		}

		log.WithField("attempt", i+1).Info("Waiting for ephemeral container to be ready...")
	}

	return fmt.Errorf("debug container failed to start within timeout")
}

// execIntoDebugContainer executes a shell in the debug container
func execIntoDebugContainer(k8sClient kubernetes.Interface, cfg *rest.Config, namespace, podName, containerName string, ptyHandler PtyHandler) error {
	log.WithFields(log.Fields{
		"namespace":     namespace,
		"podName":       podName,
		"containerName": containerName,
	}).Info("Starting exec into debug container")

	req := k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: containerName,
		Command:   []string{"sh"}, // Start with shell in debug container
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	log.WithField("execURL", req.URL().String()).Info("Creating executor")

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		log.WithError(err).Error("Failed to create SPDY executor")
		return err
	}

	// Fallback executor is default, unless feature flag is explicitly disabled.
	// Reuse environment variable for kubectl to disable the feature flag, default is enabled.
	if !cmdutil.RemoteCommandWebsockets.IsDisabled() {
		// WebSocketExecutor must be "GET" method as described in RFC 6455 Sec. 4.1 (page 17).
		websocketExec, err := remotecommand.NewWebSocketExecutor(cfg, "GET", req.URL().String())
		if err != nil {
			log.WithError(err).Warn("Failed to create WebSocket executor, falling back to SPDY")
		} else {
			exec, err = remotecommand.NewFallbackExecutor(websocketExec, exec, httpstream.IsUpgradeFailure)
			if err != nil {
				log.WithError(err).Warn("Failed to create fallback executor")
				return err
			}
		}
	}

	log.Info("Starting exec stream")
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin:             ptyHandler,
		Stdout:            ptyHandler,
		Stderr:            ptyHandler,
		TerminalSizeQueue: ptyHandler,
		Tty:               true,
	})

	if err != nil {
		log.WithError(err).Error("Exec stream failed")
	} else {
		log.Info("Exec stream completed")
	}

	return err
}
