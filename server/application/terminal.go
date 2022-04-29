package application

import (
	"context"
	"io"
	"net/http"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	apputil "github.com/argoproj/argo-cd/v2/util/app"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/rbac"
)

type terminalHandler struct {
	appLister         applisters.ApplicationNamespaceLister
	db                db.ArgoDB
	enf               *rbac.Enforcer
	cache             *servercache.Cache
	appResourceTreeFn func(ctx context.Context, app *appv1.Application) (*appv1.ApplicationTree, error)
}

// NewHandler returns a new terminal handler.
func NewHandler(appLister applisters.ApplicationNamespaceLister, db db.ArgoDB, enf *rbac.Enforcer, cache *servercache.Cache,
	appResourceTree AppResourceTreeFn) *terminalHandler {
	return &terminalHandler{
		appLister:         appLister,
		db:                db,
		enf:               enf,
		cache:             cache,
		appResourceTreeFn: appResourceTree,
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

func (s *terminalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	podName := q.Get("pod")
	container := q.Get("container")
	app := q.Get("appName")
	namespace := q.Get("namespace")
	shell := q.Get("shell")

	if podName == "" || container == "" || app == "" || namespace == "" {
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	a, err := s.appLister.Get(app)
	if err != nil {
		http.Error(w, "Cannot get app", http.StatusBadRequest)
		return
	}

	appRBACName := apputil.AppRBACName(*a)
	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if err := s.enf.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceExec, rbacpolicy.ActionGet, appRBACName); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
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
		http.Error(w, "Cannot find container", http.StatusBadRequest)
		return
	}

	session, err := newTerminalSession(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to start terminal session", http.StatusBadRequest)
		return
	}
	defer session.Done()

	validShells := []string{"bash", "sh", "powershell", "cmd"}
	if isValidShell(validShells, shell) {
		cmd := []string{shell}
		err = startProcess(kubeClientset, config, namespace, podName, container, cmd, session)
	} else {
		// No shell given or it was not valid: try some shells until one succeeds or all fail
		// FIXME: if the first shell fails then the first keyboard event is lost
		for _, testShell := range validShells {
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
