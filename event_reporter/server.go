package event_reporter

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/argoproj/argo-cd/v2/common"
	event_reporter "github.com/argoproj/argo-cd/v2/event_reporter/controller"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/server/repository"
	"github.com/argoproj/argo-cd/v2/ui"
	"github.com/argoproj/argo-cd/v2/util/assets"
	"github.com/argoproj/argo-cd/v2/util/db"
	errorsutil "github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/healthz"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/rbac"
	util_session "github.com/argoproj/argo-cd/v2/util/session"
	settings_util "github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"io/fs"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"net/http"
	"os"
	"strings"
	gosync "sync"
)

const (
	// catches corrupted informer state; see https://github.com/argoproj/argo-cd/issues/4960 for more information
	notObjectErrMsg = "object does not implement the Object interfaces"
)

type EventReporterServer struct {
	EventReporterServerOpts

	settings       *settings_util.ArgoCDSettings
	log            *log.Entry
	sessionMgr     *util_session.SessionManager
	settingsMgr    *settings_util.SettingsManager
	enf            *rbac.Enforcer
	projInformer   cache.SharedIndexInformer
	projLister     applisters.AppProjectNamespaceLister
	policyEnforcer *rbacpolicy.RBACPolicyEnforcer
	appInformer    cache.SharedIndexInformer
	appLister      applisters.ApplicationLister
	db             db.ArgoDB

	// stopCh is the channel which when closed, will shutdown the Argo CD server
	stopCh           chan struct{}
	userStateStorage util_session.UserStateStorage
	indexDataInit    gosync.Once
	indexData        []byte
	indexDataErr     error
	staticAssets     http.FileSystem
	serviceSet       *EventReporterServerSet
}

type EventReporterServerSet struct {
	RepoService *repository.Server
}

type EventReporterServerOpts struct {
	ListenPort               int
	ListenHost               string
	Namespace                string
	KubeClientset            kubernetes.Interface
	AppClientset             appclientset.Interface
	RepoClientset            repoapiclient.Clientset
	ApplicationServiceClient applicationpkg.ApplicationServiceClient
	Cache                    *servercache.Cache
	RedisClient              *redis.Client
	ApplicationNamespaces    []string
	BaseHRef                 string
	RootPath                 string
}

type handlerSwitcher struct {
	handler              http.Handler
	urlToHandler         map[string]http.Handler
	contentTypeToHandler map[string]http.Handler
}

func (s *handlerSwitcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if urlHandler, ok := s.urlToHandler[r.URL.Path]; ok {
		urlHandler.ServeHTTP(w, r)
	} else if contentHandler, ok := s.contentTypeToHandler[r.Header.Get("content-type")]; ok {
		contentHandler.ServeHTTP(w, r)
	} else {
		s.handler.ServeHTTP(w, r)
	}
}

func (a *EventReporterServer) healthCheck(r *http.Request) error {
	if val, ok := r.URL.Query()["full"]; ok && len(val) > 0 && val[0] == "true" {
		argoDB := db.NewDB(a.Namespace, a.settingsMgr, a.KubeClientset)
		_, err := argoDB.ListClusters(r.Context())
		if err != nil && strings.Contains(err.Error(), notObjectErrMsg) {
			return err
		}
	}
	return nil
}

// Init starts informers used by the API server
func (a *EventReporterServer) Init(ctx context.Context) {
	go a.appInformer.Run(ctx.Done())
	controller := event_reporter.NewEventReporterController(a.appInformer, a.Cache, a.settingsMgr, a.ApplicationServiceClient)
	go controller.Run(ctx)
}

// newHTTPServer returns the HTTP server to serve HTTP/HTTPS requests. This is implemented
// using grpc-gateway as a proxy to the gRPC server.
func (a *EventReporterServer) newHTTPServer(ctx context.Context, port int) *http.Server {
	endpoint := fmt.Sprintf("localhost:%d", port)
	mux := http.NewServeMux()
	httpS := http.Server{
		Addr: endpoint,
		Handler: &handlerSwitcher{
			handler: mux,
		},
	}

	healthz.ServeHealthCheck(mux, a.healthCheck)
	return &httpS
}

func (a *EventReporterServer) checkServeErr(name string, err error) {
	if err != nil {
		if a.stopCh == nil {
			// a nil stopCh indicates a graceful shutdown
			log.Infof("graceful shutdown %s: %v", name, err)
		} else {
			log.Fatalf("%s: %v", name, err)
		}
	} else {
		log.Infof("graceful shutdown %s", name)
	}
}

// Run runs the API Server
// We use k8s.io/code-generator/cmd/go-to-protobuf to generate the .proto files from the API types.
// k8s.io/ go-to-protobuf uses protoc-gen-gogo, which comes from gogo/protobuf (a fork of
// golang/protobuf).
func (a *EventReporterServer) Run(ctx context.Context) {
	a.userStateStorage.Init(ctx)
	svcSet := newEventReporterServiceSet(a)
	a.serviceSet = svcSet
	var httpS = a.newHTTPServer(ctx, a.ListenPort)
	tlsConfig := tls.Config{}
	tlsConfig.GetCertificate = func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return a.settings.Certificate, nil
	}
	go func() { a.checkServeErr("httpS", httpS.ListenAndServe()) }()
	if !cache.WaitForCacheSync(ctx.Done(), a.projInformer.HasSynced, a.appInformer.HasSynced) {
		log.Fatal("Timed out waiting for project cache to sync")
	}

	a.stopCh = make(chan struct{})
	<-a.stopCh
}

// NewServer returns a new instance of the Argo CD API server
func NewEventReporterServer(ctx context.Context, opts EventReporterServerOpts) *EventReporterServer {
	settingsMgr := settings_util.NewSettingsManager(ctx, opts.KubeClientset, opts.Namespace)
	settings, err := settingsMgr.InitializeSettings(true)
	errorsutil.CheckError(err)

	appInformerNs := opts.Namespace
	if len(opts.ApplicationNamespaces) > 0 {
		appInformerNs = ""
	}
	projFactory := appinformer.NewSharedInformerFactoryWithOptions(opts.AppClientset, 0, appinformer.WithNamespace(opts.Namespace), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))
	appFactory := appinformer.NewSharedInformerFactoryWithOptions(opts.AppClientset, 0, appinformer.WithNamespace(appInformerNs), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))

	projInformer := projFactory.Argoproj().V1alpha1().AppProjects().Informer()
	projLister := projFactory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(opts.Namespace)

	appInformer := appFactory.Argoproj().V1alpha1().Applications().Informer()
	appLister := appFactory.Argoproj().V1alpha1().Applications().Lister()

	userStateStorage := util_session.NewUserStateStorage(opts.RedisClient)
	sessionMgr := util_session.NewSessionManager(settingsMgr, projLister, "", nil, userStateStorage)
	enf := rbac.NewEnforcer(opts.KubeClientset, opts.Namespace, common.ArgoCDRBACConfigMapName, nil)
	enf.EnableEnforce(false)
	err = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	errorsutil.CheckError(err)
	enf.EnableLog(os.Getenv(common.EnvVarRBACDebug) == "1")

	policyEnf := rbacpolicy.NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(policyEnf.EnforceClaims)

	var staticFS fs.FS = io.NewSubDirFS("dist/app", ui.Embedded)

	dbInstance := db.NewDB(opts.Namespace, settingsMgr, opts.KubeClientset)

	a := &EventReporterServer{
		EventReporterServerOpts: opts,
		log:                     log.NewEntry(log.StandardLogger()),
		settings:                settings,
		sessionMgr:              sessionMgr,
		settingsMgr:             settingsMgr,
		enf:                     enf,
		projInformer:            projInformer,
		projLister:              projLister,
		appInformer:             appInformer,
		appLister:               appLister,
		policyEnforcer:          policyEnf,
		userStateStorage:        userStateStorage,
		staticAssets:            http.FS(staticFS),
		db:                      dbInstance,
	}

	if err != nil {
		// Just log. It's not critical.
		log.Warnf("Failed to log in-cluster warnings: %v", err)
	}

	return a
}

func newEventReporterServiceSet(a *EventReporterServer) *EventReporterServerSet {
	repoService := repository.NewServer(a.RepoClientset, a.db, a.enf, a.Cache, a.appLister, a.projInformer, a.Namespace, a.settingsMgr)

	return &EventReporterServerSet{
		RepoService: repoService,
	}
}
