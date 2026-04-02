package application

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"slices"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

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
	terminalOptions   *TerminalOptions
	getSettings       GetSettingsFunc
}

// NewDebugHandler returns a new debug handler for ephemeral container sessions.
func NewDebugHandler(appLister applisters.ApplicationLister, namespace string, enabledNamespaces []string, db db.ArgoDB, appResourceTree AppResourceTreeFn, sessionManager *util_session.SessionManager, terminalOptions *TerminalOptions, getSettings GetSettingsFunc) *debugHandler {
	return &debugHandler{
		appLister:         appLister,
		db:                db,
		appResourceTreeFn: appResourceTree,
		namespace:         namespace,
		enabledNamespaces: enabledNamespaces,
		sessionManager:    sessionManager,
		terminalOptions:   terminalOptions,
		getSettings:       getSettings,
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
// feature is enabled before invoking the main handler.
func (s *debugHandler) WithFeatureFlagMiddleware() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		argocdSettings, err := s.getSettings()
		if err != nil {
			log.Errorf("error executing debug WithFeatureFlagMiddleware: error getting settings: %s", err)
			http.Error(w, "Failed to get settings", http.StatusBadRequest)
			return
		}
		if !argocdSettings.DebugEnabled {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		s.ServeHTTP(w, r)
	})
}

func (s *debugHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	podName := q.Get("pod")
	app := q.Get("appName")
	project := q.Get("projectName")
	namespace := q.Get("namespace")
	image := q.Get("image")

	if podName == "" || app == "" || project == "" || namespace == "" || image == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	appNamespace := q.Get("appNamespace")
	targetContainer := q.Get("targetContainer") // optional

	if !argo.IsValidPodName(podName) {
		http.Error(w, "Pod name is not valid", http.StatusBadRequest)
		return
	}
	if !argo.IsValidAppName(app) {
		http.Error(w, "App name is not valid", http.StatusBadRequest)
		return
	}
	if !argo.IsValidProjectName(project) {
		http.Error(w, "Project name is not valid", http.StatusBadRequest)
		return
	}
	if !argo.IsValidNamespaceName(namespace) {
		http.Error(w, "Namespace name is not valid", http.StatusBadRequest)
		return
	}
	if !argo.IsValidNamespaceName(appNamespace) {
		http.Error(w, "App namespace name is not valid", http.StatusBadRequest)
		return
	}
	if targetContainer != "" && !argo.IsValidContainerName(targetContainer) {
		http.Error(w, "Target container name is not valid", http.StatusBadRequest)
		return
	}

	ns := appNamespace
	if ns == "" {
		ns = s.namespace
	}

	if !security.IsNamespaceEnabled(ns, s.namespace, s.enabledNamespaces) {
		http.Error(w, security.NamespaceNotPermittedError(ns).Error(), http.StatusForbidden)
		return
	}

	ctx := r.Context()

	appRBACName := security.RBACName(s.namespace, project, appNamespace, app)
	if err := s.terminalOptions.Enf.EnforceErr(ctx.Value("claims"), rbac.ResourceApplications, rbac.ActionGet, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := s.terminalOptions.Enf.EnforceErr(ctx.Value("claims"), rbac.ResourceDebug, rbac.ActionCreate, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Validate image is in the allowlist
	argocdSettings, err := s.getSettings()
	if err != nil {
		http.Error(w, "Failed to get settings", http.StatusInternalServerError)
		return
	}
	if !slices.Contains(argocdSettings.DebugImages, image) {
		http.Error(w, "Image is not in the allowed debug images list", http.StatusBadRequest)
		return
	}

	fieldLog := log.WithFields(log.Fields{
		"application": app, "userName": util_session.Username(ctx),
		"podName": podName, "namespace": namespace, "project": project,
		"appNamespace": appNamespace, "image": image,
	})

	a, err := s.appLister.Applications(ns).Get(app)
	if err != nil {
		if apierrors.IsNotFound(err) {
			http.Error(w, "App not found", http.StatusNotFound)
			return
		}
		fieldLog.Errorf("Error when getting app %q when launching debug session: %s", app, err)
		http.Error(w, "Cannot get app", http.StatusInternalServerError)
		return
	}

	if a.Spec.Project != project {
		fieldLog.Warnf("The wrong project (%q) was specified for the app %q when launching debug session", project, app)
		http.Error(w, "The wrong project was specified for the app", http.StatusBadRequest)
		return
	}

	config, err := s.getApplicationClusterRawConfig(ctx, a)
	if err != nil {
		http.Error(w, "Cannot get raw cluster config", http.StatusBadRequest)
		return
	}

	kubeClientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		http.Error(w, "Cannot initialize kubeclient", http.StatusBadRequest)
		return
	}

	resourceTree, err := s.appResourceTreeFn(ctx, a)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if !podExists(resourceTree.Nodes, podName, namespace) {
		http.Error(w, "Pod doesn't belong to specified app", http.StatusBadRequest)
		return
	}

	pod, err := kubeClientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		fieldLog.Errorf("error retrieving pod: %s", err)
		http.Error(w, "Cannot find pod", http.StatusBadRequest)
		return
	}

	if pod.Status.Phase != corev1.PodRunning {
		http.Error(w, "Pod not running", http.StatusBadRequest)
		return
	}

	debugContainerName, err := startDebugProcess(ctx, kubeClientset, namespace, podName, image, targetContainer)
	if err != nil {
		fieldLog.Errorf("error starting ephemeral container: %s", err)
		http.Error(w, "Failed to start debug container: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Wait for ephemeral container to be running
	if err := waitForEphemeralContainer(ctx, kubeClientset, namespace, podName, debugContainerName); err != nil {
		fieldLog.Errorf("error waiting for ephemeral container: %s", err)
		http.Error(w, "Debug container failed to start: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fieldLog.Infof("debug session starting with ephemeral container %q", debugContainerName)

	session, err := newTerminalSession(ctx, w, r, nil, s.sessionManager, appRBACName, s.terminalOptions)
	if err != nil {
		http.Error(w, "Failed to start debug session", http.StatusBadRequest)
		return
	}
	defer session.Done()

	go session.StartKeepalives(time.Second * 5)

	if err = attachToContainer(kubeClientset, config, namespace, podName, debugContainerName, session); err != nil {
		http.Error(w, "Failed to attach to debug container", http.StatusBadRequest)
		session.Close()
		return
	}

	session.Close()
}

// startDebugProcess creates an ephemeral container on the target pod and returns the container name.
func startDebugProcess(ctx context.Context, k8sClient kubernetes.Interface, namespace, podName, image, targetContainer string) (string, error) {
	debugContainerName := fmt.Sprintf("debug-%s", randomSuffix(6))

	ephemeralContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:                     debugContainerName,
			Image:                    image,
			Stdin:                    true,
			TTY:                      true,
			TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
		},
	}
	if targetContainer != "" {
		ephemeralContainer.TargetContainerName = targetContainer
	}

	pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot get pod: %w", err)
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, ephemeralContainer)

	_, err = k8sClient.CoreV1().Pods(namespace).UpdateEphemeralContainers(ctx, podName, pod, metav1.UpdateOptions{})
	if err != nil {
		return "", fmt.Errorf("cannot update ephemeral containers: %w", err)
	}

	return debugContainerName, nil
}

// waitForEphemeralContainer polls until the named ephemeral container is running or the context is done.
func waitForEphemeralContainer(ctx context.Context, k8sClient kubernetes.Interface, namespace, podName, containerName string) error {
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timed out waiting for ephemeral container %q to start", containerName)
		case <-ticker.C:
			pod, err := k8sClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("error polling pod: %w", err)
			}
			for _, cs := range pod.Status.EphemeralContainerStatuses {
				if cs.Name == containerName {
					if cs.State.Running != nil {
						return nil
					}
					if cs.State.Terminated != nil {
						return fmt.Errorf("ephemeral container terminated: %s", cs.State.Terminated.Reason)
					}
				}
			}
		}
	}
}

// attachToContainer attaches stdin/stdout/stderr to an already-running container via the attach API.
func attachToContainer(k8sClient kubernetes.Interface, cfg *rest.Config, namespace, podName, containerName string, ptyHandler PtyHandler) error {
	req := k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("attach")

	req.VersionedParams(&corev1.PodAttachOptions{
		Container: containerName,
		Stdin:     true,
		Stdout:    true,
		Stderr:    true,
		TTY:       true,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return err
	}

	return exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin:             ptyHandler,
		Stdout:            ptyHandler,
		Stderr:            ptyHandler,
		TerminalSizeQueue: ptyHandler,
		Tty:               true,
	})
}

func randomSuffix(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
