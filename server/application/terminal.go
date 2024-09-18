package application

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	util_session "github.com/argoproj/argo-cd/v2/util/session"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	"github.com/argoproj/argo-cd/v2/util/security"
	sessionmgr "github.com/argoproj/argo-cd/v2/util/session"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type terminalHandler struct {
	appLister         applisters.ApplicationLister
	db                db.ArgoDB
	cache             *servercache.Cache
	appResourceTreeFn func(ctx context.Context, app *appv1.Application) (*appv1.ApplicationTree, error)
	allowedShells     []string
	namespace         string
	enabledNamespaces []string
	sessionManager    *util_session.SessionManager
	terminalOptions   *TerminalOptions
}

type TerminalOptions struct {
	DisableAuth bool
	Enf         *rbac.Enforcer
}

// NewHandler returns a new terminal handler.
func NewHandler(appLister applisters.ApplicationLister, namespace string, enabledNamespaces []string, db db.ArgoDB, cache *servercache.Cache, appResourceTree AppResourceTreeFn, allowedShells []string, sessionManager *sessionmgr.SessionManager, terminalOptions *TerminalOptions) *terminalHandler {
	return &terminalHandler{
		appLister:         appLister,
		db:                db,
		cache:             cache,
		appResourceTreeFn: appResourceTree,
		allowedShells:     allowedShells,
		namespace:         namespace,
		enabledNamespaces: enabledNamespaces,
		sessionManager:    sessionManager,
		terminalOptions:   terminalOptions,
	}
}

func (s *terminalHandler) getApplicationClusterRawConfig(ctx context.Context, a *appv1.Application) (*rest.Config, error) {
	if err := argo.ValidateDestination(ctx, &a.Spec.Destination, s.db); err != nil {
		return nil, err
	}
	clst, err := s.db.GetCluster(ctx, a.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	return clst.RawRestConfig(), nil
}

type GetSettingsFunc func() (*settings.ArgoCDSettings, error)

// WithFeatureFlagMiddleware is an HTTP middleware to verify if the terminal
// feature is enabled before invoking the main handler
func (s *terminalHandler) WithFeatureFlagMiddleware(getSettings GetSettingsFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		argocdSettings, err := getSettings()
		if err != nil {
			log.Errorf("error executing WithFeatureFlagMiddleware: error getting settings: %s", err)
			http.Error(w, "Failed to get settings", http.StatusBadRequest)
			return
		}
		if !argocdSettings.ExecEnabled {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		s.ServeHTTP(w, r)
	})
}

func (s *terminalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	podName := q.Get("pod")
	container := q.Get("container")
	app := q.Get("appName")
	project := q.Get("projectName")
	namespace := q.Get("namespace")

	if podName == "" || container == "" || app == "" || project == "" || namespace == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	appNamespace := q.Get("appNamespace")

	if !argo.IsValidPodName(podName) {
		http.Error(w, "Pod name is not valid", http.StatusBadRequest)
		return
	}
	if !argo.IsValidContainerName(container) {
		http.Error(w, "Container name is not valid", http.StatusBadRequest)
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

	ns := appNamespace
	if ns == "" {
		ns = s.namespace
	}

	if !security.IsNamespaceEnabled(ns, s.namespace, s.enabledNamespaces) {
		http.Error(w, security.NamespaceNotPermittedError(ns).Error(), http.StatusForbidden)
		return
	}

	shell := q.Get("shell") // No need to validate. Will only be used if it's in the allow-list.

	ctx := r.Context()

	appRBACName := security.RBACName(s.namespace, project, appNamespace, app)
	if err := s.terminalOptions.Enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := s.terminalOptions.Enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceExec, rbacpolicy.ActionCreate, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	fieldLog := log.WithFields(log.Fields{
		"application": app, "userName": sessionmgr.Username(ctx), "container": container,
		"podName": podName, "namespace": namespace, "project": project, "appNamespace": appNamespace,
	})

	a, err := s.appLister.Applications(ns).Get(app)
	if err != nil {
		if apierr.IsNotFound(err) {
			http.Error(w, "App not found", http.StatusNotFound)
			return
		}
		fieldLog.Errorf("Error when getting app %q when launching a terminal: %s", app, err)
		http.Error(w, "Cannot get app", http.StatusInternalServerError)
		return
	}

	if a.Spec.Project != project {
		fieldLog.Warnf("The wrong project (%q) was specified for the app %q when launching a terminal", project, app)
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

	// From the tree find pods which match the given pod.
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

	if pod.Status.Phase != v1.PodRunning {
		http.Error(w, "Pod not running", http.StatusBadRequest)
		return
	}

	var findContainer bool
	for _, c := range pod.Spec.Containers {
		if container == c.Name {
			findContainer = true
			break
		}
	}
	if !findContainer {
		fieldLog.Warn("terminal container not found")
		http.Error(w, "Cannot find container", http.StatusBadRequest)
		return
	}

	fieldLog.Info("terminal session starting")

	session, err := newTerminalSession(ctx, w, r, nil, s.sessionManager, appRBACName, s.terminalOptions)
	if err != nil {
		http.Error(w, "Failed to start terminal session", http.StatusBadRequest)
		return
	}
	defer session.Done()

	// send pings across the WebSocket channel at regular intervals to keep it alive through
	// load balancers which may close an idle connection after some period of time
	go session.StartKeepalives(time.Second * 5)

	if isValidShell(s.allowedShells, shell) {
		cmd := []string{shell}
		err = startProcess(kubeClientset, config, namespace, podName, container, cmd, session)
	} else {
		// No shell given or the given shell was not allowed: try the configured shells until one succeeds or all fail.
		for _, testShell := range s.allowedShells {
			cmd := []string{testShell}
			if err = startProcess(kubeClientset, config, namespace, podName, container, cmd, session); err == nil {
				break
			}
		}
	}

	if err != nil {
		http.Error(w, "Failed to exec container", http.StatusBadRequest)
		session.Close()
		return
	}

	session.Close()
}

func podExists(treeNodes []appv1.ResourceNode, podName, namespace string) bool {
	for _, treeNode := range treeNodes {
		if treeNode.Kind == kube.PodKind && treeNode.Group == "" && treeNode.UID != "" &&
			treeNode.Name == podName && treeNode.Namespace == namespace {
			return true
		}
	}
	return false
}

const EndOfTransmission = "\u0004"

// PtyHandler is what remotecommand expects from a pty
type PtyHandler interface {
	io.Reader
	io.Writer
	remotecommand.TerminalSizeQueue
}

// TerminalMessage is the struct for websocket message.
type TerminalMessage struct {
	Operation string `json:"operation"`
	Data      string `json:"data"`
	Rows      uint16 `json:"rows"`
	Cols      uint16 `json:"cols"`
}

// TerminalCommand is the struct for websocket commands,For example you need ask client to reconnect
type TerminalCommand struct {
	Code int
}

// startProcess executes specified commands in the container and connects it up with the ptyHandler (a session)
func startProcess(k8sClient kubernetes.Interface, cfg *rest.Config, namespace, podName, containerName string, cmd []string, ptyHandler PtyHandler) error {
	req := k8sClient.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec")

	req.VersionedParams(&v1.PodExecOptions{
		Container: containerName,
		Command:   cmd,
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

// isValidShell checks if the shell is an allowed one
func isValidShell(validShells []string, shell string) bool {
	for _, validShell := range validShells {
		if validShell == shell {
			return true
		}
	}
	return false
}
