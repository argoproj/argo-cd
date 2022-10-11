package application

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	apimachineryvalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	sessionmgr "github.com/argoproj/argo-cd/v2/util/session"
)

type terminalHandler struct {
	appLister         applisters.ApplicationLister
	db                db.ArgoDB
	enf               *rbac.Enforcer
	cache             *servercache.Cache
	appResourceTreeFn func(ctx context.Context, app *appv1.Application) (*appv1.ApplicationTree, error)
	allowedShells     []string
	namespace         string
}

// NewHandler returns a new terminal handler.
func NewHandler(appLister applisters.ApplicationLister, namespace string, db db.ArgoDB, enf *rbac.Enforcer, cache *servercache.Cache,
	appResourceTree AppResourceTreeFn, allowedShells []string) *terminalHandler {
	return &terminalHandler{
		appLister:         appLister,
		db:                db,
		enf:               enf,
		cache:             cache,
		appResourceTreeFn: appResourceTree,
		allowedShells:     allowedShells,
		namespace:         namespace,
	}
}

func (s *terminalHandler) getApplicationClusterRawConfig(ctx context.Context, a *appv1.Application) (*rest.Config, error) {
	if err := argo.ValidateDestination(ctx, &a.Spec.Destination, s.db); err != nil {
		return nil, err
	}
	clst, err := s.db.GetClusterByUrl(ctx, a.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	return clst.RawRestConfig(), nil
}

// isValidPodName checks that a podName is valid
func isValidPodName(name string) bool {
	// https://github.com/kubernetes/kubernetes/blob/976a940f4a4e84fe814583848f97b9aafcdb083f/pkg/apis/core/validation/validation.go#L241
	validationErrors := apimachineryvalidation.NameIsDNSSubdomain(name, false)
	return len(validationErrors) == 0
}

func isValidAppName(name string) bool {
	// app names have the same rules as pods.
	return isValidPodName(name)
}

func isValidProjectName(name string) bool {
	// project names have the same rules as pods.
	return isValidPodName(name)
}

// isValidNamespaceName checks that a namespace name is valid
func isValidNamespaceName(name string) bool {
	// https://github.com/kubernetes/kubernetes/blob/976a940f4a4e84fe814583848f97b9aafcdb083f/pkg/apis/core/validation/validation.go#L262
	validationErrors := apimachineryvalidation.ValidateNamespaceName(name, false)
	return len(validationErrors) == 0
}

// isValidContainerName checks that a containerName is valid
func isValidContainerName(name string) bool {
	// https://github.com/kubernetes/kubernetes/blob/53a9d106c4aabcd550cc32ae4e8004f32fb0ae7b/pkg/api/validation/validation.go#L280
	validationErrors := apimachineryvalidation.NameIsDNSLabel(name, false)
	return len(validationErrors) == 0
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

	if !isValidPodName(podName) {
		http.Error(w, "Pod name is not valid", http.StatusBadRequest)
		return
	}
	if !isValidContainerName(container) {
		http.Error(w, "Container name is not valid", http.StatusBadRequest)
		return
	}
	if !isValidAppName(app) {
		http.Error(w, "App name is not valid", http.StatusBadRequest)
		return
	}
	if !isValidProjectName(project) {
		http.Error(w, "Project name is not valid", http.StatusBadRequest)
		return
	}
	if !isValidNamespaceName(namespace) {
		http.Error(w, "Namespace name is not valid", http.StatusBadRequest)
		return
	}
	shell := q.Get("shell") // No need to validate. Will only be used if it's in the allow-list.

	ctx := r.Context()

	appRBACName := fmt.Sprintf("%s/%s", project, app)
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceExec, rbacpolicy.ActionCreate, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	fieldLog := log.WithFields(log.Fields{"application": app, "userName": sessionmgr.Username(ctx), "container": container,
		"podName": podName, "namespace": namespace, "cluster": project})

	a, err := s.appLister.Applications(s.namespace).Get(app)
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

	session, err := newTerminalSession(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to start terminal session", http.StatusBadRequest)
		return
	}
	defer session.Done()

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

	return exec.Stream(remotecommand.StreamOptions{
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
