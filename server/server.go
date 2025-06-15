package server

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	goio "io"
	"io/fs"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	go_runtime "runtime"
	"runtime/debug"
	"strings"
	gosync "sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/pkg/v2/sync"
	"github.com/golang-jwt/jwt/v5"
	golang_proto "github.com/golang/protobuf/proto" //nolint:staticcheck
	"github.com/gorilla/handlers"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-middleware/providers/prometheus"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/auth"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient"
	accountpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	applicationsetpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/applicationset"
	certificatepkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/certificate"
	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	gpgkeypkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/gpgkey"
	notificationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/notification"
	projectpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/project"
	repocredspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/repocreds"
	repositorypkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/repository"
	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	settingspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	versionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	appinformer "github.com/argoproj/argo-cd/v3/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v3/pkg/client/listers/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	repocache "github.com/argoproj/argo-cd/v3/reposerver/cache"
	"github.com/argoproj/argo-cd/v3/server/account"
	"github.com/argoproj/argo-cd/v3/server/application"
	"github.com/argoproj/argo-cd/v3/server/applicationset"
	"github.com/argoproj/argo-cd/v3/server/badge"
	servercache "github.com/argoproj/argo-cd/v3/server/cache"
	"github.com/argoproj/argo-cd/v3/server/certificate"
	"github.com/argoproj/argo-cd/v3/server/cluster"
	"github.com/argoproj/argo-cd/v3/server/extension"
	"github.com/argoproj/argo-cd/v3/server/gpgkey"
	"github.com/argoproj/argo-cd/v3/server/logout"
	"github.com/argoproj/argo-cd/v3/server/metrics"
	"github.com/argoproj/argo-cd/v3/server/notification"
	"github.com/argoproj/argo-cd/v3/server/project"
	"github.com/argoproj/argo-cd/v3/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v3/server/repocreds"
	"github.com/argoproj/argo-cd/v3/server/repository"
	"github.com/argoproj/argo-cd/v3/server/session"
	"github.com/argoproj/argo-cd/v3/server/settings"
	"github.com/argoproj/argo-cd/v3/server/version"
	"github.com/argoproj/argo-cd/v3/ui"
	"github.com/argoproj/argo-cd/v3/util/assets"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/db"
	dexutil "github.com/argoproj/argo-cd/v3/util/dex"
	"github.com/argoproj/argo-cd/v3/util/env"
	errorsutil "github.com/argoproj/argo-cd/v3/util/errors"
	grpc_util "github.com/argoproj/argo-cd/v3/util/grpc"
	"github.com/argoproj/argo-cd/v3/util/healthz"
	httputil "github.com/argoproj/argo-cd/v3/util/http"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	"github.com/argoproj/argo-cd/v3/util/io/files"
	jwtutil "github.com/argoproj/argo-cd/v3/util/jwt"
	kubeutil "github.com/argoproj/argo-cd/v3/util/kube"
	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"
	"github.com/argoproj/argo-cd/v3/util/notification/k8s"
	settings_notif "github.com/argoproj/argo-cd/v3/util/notification/settings"
	"github.com/argoproj/argo-cd/v3/util/oidc"
	"github.com/argoproj/argo-cd/v3/util/rbac"
	util_session "github.com/argoproj/argo-cd/v3/util/session"
	settings_util "github.com/argoproj/argo-cd/v3/util/settings"
	"github.com/argoproj/argo-cd/v3/util/swagger"
	tlsutil "github.com/argoproj/argo-cd/v3/util/tls"
	"github.com/argoproj/argo-cd/v3/util/webhook"
)

const (
	maxConcurrentLoginRequestsCountEnv = "ARGOCD_MAX_CONCURRENT_LOGIN_REQUESTS_COUNT"
	replicasCountEnv                   = "ARGOCD_API_SERVER_REPLICAS"
	renewTokenKey                      = "renew-token"
)

// ErrNoSession indicates no auth token was supplied as part of a request
var ErrNoSession = status.Errorf(codes.Unauthenticated, "no session information")

var noCacheHeaders = map[string]string{
	"Expires":         time.Unix(0, 0).Format(time.RFC1123),
	"Cache-Control":   "no-cache, private, max-age=0",
	"Pragma":          "no-cache",
	"X-Accel-Expires": "0",
}

var backoff = wait.Backoff{
	Steps:    5,
	Duration: 500 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

var (
	clientConstraint = ">= " + common.MinClientVersion
	baseHRefRegex    = regexp.MustCompile(`<base href="(.*?)">`)
	// limits number of concurrent login requests to prevent password brute forcing. If set to 0 then no limit is enforced.
	maxConcurrentLoginRequestsCount = 50
	replicasCount                   = 1
	enableGRPCTimeHistogram         = true
)

func init() {
	maxConcurrentLoginRequestsCount = env.ParseNumFromEnv(maxConcurrentLoginRequestsCountEnv, maxConcurrentLoginRequestsCount, 0, math.MaxInt32)
	replicasCount = env.ParseNumFromEnv(replicasCountEnv, replicasCount, 0, math.MaxInt32)
	if replicasCount > 0 {
		maxConcurrentLoginRequestsCount = maxConcurrentLoginRequestsCount / replicasCount
	}
	enableGRPCTimeHistogram = env.ParseBoolFromEnv(common.EnvEnableGRPCTimeHistogramEnv, false)
}

// ArgoCDServer is the API server for Argo CD
type ArgoCDServer struct {
	ArgoCDServerOpts
	ApplicationSetOpts

	ssoClientApp   *oidc.ClientApp
	settings       *settings_util.ArgoCDSettings
	log            *log.Entry
	sessionMgr     *util_session.SessionManager
	settingsMgr    *settings_util.SettingsManager
	enf            *rbac.Enforcer
	projInformer   cache.SharedIndexInformer
	policyEnforcer *rbacpolicy.RBACPolicyEnforcer
	appInformer    cache.SharedIndexInformer
	appLister      applisters.ApplicationLister
	appsetInformer cache.SharedIndexInformer
	appsetLister   applisters.ApplicationSetLister
	db             db.ArgoDB

	// stopCh is the channel which when closed, will shutdown the Argo CD server
	stopCh             chan os.Signal
	userStateStorage   util_session.UserStateStorage
	indexDataInit      gosync.Once
	indexData          []byte
	indexDataErr       error
	staticAssets       http.FileSystem
	apiFactory         api.Factory
	secretInformer     cache.SharedIndexInformer
	configMapInformer  cache.SharedIndexInformer
	serviceSet         *ArgoCDServiceSet
	extensionManager   *extension.Manager
	Shutdown           func()
	terminateRequested atomic.Bool
	available          atomic.Bool
}

type ArgoCDServerOpts struct {
	DisableAuth             bool
	ContentTypes            []string
	EnableGZip              bool
	Insecure                bool
	StaticAssetsDir         string
	ListenPort              int
	ListenHost              string
	MetricsPort             int
	MetricsHost             string
	Namespace               string
	DexServerAddr           string
	DexTLSConfig            *dexutil.DexTLSConfig
	BaseHRef                string
	RootPath                string
	DynamicClientset        dynamic.Interface
	KubeControllerClientset client.Client
	KubeClientset           kubernetes.Interface
	AppClientset            appclientset.Interface
	RepoClientset           repoapiclient.Clientset
	Cache                   *servercache.Cache
	RepoServerCache         *repocache.Cache
	RedisClient             *redis.Client
	TLSConfigCustomizer     tlsutil.ConfigCustomizer
	XFrameOptions           string
	ContentSecurityPolicy   string
	ApplicationNamespaces   []string
	EnableProxyExtension    bool
	WebhookParallelism      int
	EnableK8sEvent          []string
	HydratorEnabled         bool
	SyncWithReplaceAllowed  bool
}

type ApplicationSetOpts struct {
	GitSubmoduleEnabled      bool
	EnableNewGitFileGlobbing bool
	ScmRootCAPath            string
	AllowedScmProviders      []string
	EnableScmProviders       bool
}

// GracefulRestartSignal implements a signal to be used for a graceful restart trigger.
type GracefulRestartSignal struct{}

// HTTPMetricsRegistry exposes operations to update http metrics in the Argo CD
// API server.
type HTTPMetricsRegistry interface {
	// IncExtensionRequestCounter will increase the request counter for the given
	// extension with the given status.
	IncExtensionRequestCounter(extension string, status int)
	// ObserveExtensionRequestDuration will register the request roundtrip duration
	// between Argo CD API Server and the extension backend service for the given
	// extension.
	ObserveExtensionRequestDuration(extension string, duration time.Duration)
}

// String is a part of os.Signal interface to represent a signal as a string.
func (g GracefulRestartSignal) String() string {
	return "GracefulRestartSignal"
}

// Signal is a part of os.Signal interface doing nothing.
func (g GracefulRestartSignal) Signal() {}

// initializeDefaultProject creates the default project if it does not already exist
func initializeDefaultProject(opts ArgoCDServerOpts) error {
	defaultProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.DefaultAppProjectName, Namespace: opts.Namespace},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:              []string{"*"},
			Destinations:             []v1alpha1.ApplicationDestination{{Server: "*", Namespace: "*"}},
			ClusterResourceWhitelist: []metav1.GroupKind{{Group: "*", Kind: "*"}},
		},
	}

	_, err := opts.AppClientset.ArgoprojV1alpha1().AppProjects(opts.Namespace).Get(context.Background(), defaultProj.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = opts.AppClientset.ArgoprojV1alpha1().AppProjects(opts.Namespace).Create(context.Background(), defaultProj, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return nil
		}
	}
	return err
}

// NewServer returns a new instance of the Argo CD API server
func NewServer(ctx context.Context, opts ArgoCDServerOpts, appsetOpts ApplicationSetOpts) *ArgoCDServer {
	settingsMgr := settings_util.NewSettingsManager(ctx, opts.KubeClientset, opts.Namespace)
	settings, err := settingsMgr.InitializeSettings(opts.Insecure)
	errorsutil.CheckError(err)
	err = initializeDefaultProject(opts)
	errorsutil.CheckError(err)

	appInformerNs := opts.Namespace
	if len(opts.ApplicationNamespaces) > 0 {
		appInformerNs = ""
	}
	projFactory := appinformer.NewSharedInformerFactoryWithOptions(opts.AppClientset, 0, appinformer.WithNamespace(opts.Namespace), appinformer.WithTweakListOptions(func(_ *metav1.ListOptions) {}))
	appFactory := appinformer.NewSharedInformerFactoryWithOptions(opts.AppClientset, 0, appinformer.WithNamespace(appInformerNs), appinformer.WithTweakListOptions(func(_ *metav1.ListOptions) {}))

	projInformer := projFactory.Argoproj().V1alpha1().AppProjects().Informer()
	projLister := projFactory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(opts.Namespace)

	appInformer := appFactory.Argoproj().V1alpha1().Applications().Informer()
	appLister := appFactory.Argoproj().V1alpha1().Applications().Lister()

	appsetInformer := appFactory.Argoproj().V1alpha1().ApplicationSets().Informer()
	appsetLister := appFactory.Argoproj().V1alpha1().ApplicationSets().Lister()

	userStateStorage := util_session.NewUserStateStorage(opts.RedisClient)
	sessionMgr := util_session.NewSessionManager(settingsMgr, projLister, opts.DexServerAddr, opts.DexTLSConfig, userStateStorage)
	enf := rbac.NewEnforcer(opts.KubeClientset, opts.Namespace, common.ArgoCDRBACConfigMapName, nil)
	enf.EnableEnforce(!opts.DisableAuth)
	err = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	errorsutil.CheckError(err)
	enf.EnableLog(os.Getenv(common.EnvVarRBACDebug) == "1")

	policyEnf := rbacpolicy.NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(policyEnf.EnforceClaims)

	staticFS, err := fs.Sub(ui.Embedded, "dist/app")
	errorsutil.CheckError(err)

	root, err := os.OpenRoot(opts.StaticAssetsDir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warnf("Static assets directory %q does not exist, using only embedded assets", opts.StaticAssetsDir)
		} else {
			errorsutil.CheckError(err)
		}
	} else {
		staticFS = utilio.NewComposableFS(staticFS, root.FS())
	}

	argocdService, err := service.NewArgoCDService(opts.KubeClientset, opts.Namespace, opts.RepoClientset)
	errorsutil.CheckError(err)

	secretInformer := k8s.NewSecretInformer(opts.KubeClientset, opts.Namespace, "argocd-notifications-secret")
	configMapInformer := k8s.NewConfigMapInformer(opts.KubeClientset, opts.Namespace, "argocd-notifications-cm")

	apiFactory := api.NewFactory(settings_notif.GetFactorySettings(argocdService, "argocd-notifications-secret", "argocd-notifications-cm", false), opts.Namespace, secretInformer, configMapInformer)

	dbInstance := db.NewDB(opts.Namespace, settingsMgr, opts.KubeClientset)
	logger := log.NewEntry(log.StandardLogger())

	sg := extension.NewDefaultSettingsGetter(settingsMgr)
	ag := extension.NewDefaultApplicationGetter(appLister)
	pg := extension.NewDefaultProjectGetter(projLister, dbInstance)
	ug := extension.NewDefaultUserGetter(policyEnf)
	em := extension.NewManager(logger, opts.Namespace, sg, ag, pg, dbInstance, enf, ug)
	noopShutdown := func() {
		log.Error("API Server Shutdown function called but server is not started yet.")
	}

	a := &ArgoCDServer{
		ArgoCDServerOpts:   opts,
		ApplicationSetOpts: appsetOpts,
		log:                logger,
		settings:           settings,
		sessionMgr:         sessionMgr,
		settingsMgr:        settingsMgr,
		enf:                enf,
		projInformer:       projInformer,
		appInformer:        appInformer,
		appLister:          appLister,
		appsetInformer:     appsetInformer,
		appsetLister:       appsetLister,
		policyEnforcer:     policyEnf,
		userStateStorage:   userStateStorage,
		staticAssets:       http.FS(staticFS),
		db:                 dbInstance,
		apiFactory:         apiFactory,
		secretInformer:     secretInformer,
		configMapInformer:  configMapInformer,
		extensionManager:   em,
		Shutdown:           noopShutdown,
		stopCh:             make(chan os.Signal, 1),
	}

	err = a.logInClusterWarnings()
	if err != nil {
		// Just log. It's not critical.
		log.Warnf("Failed to log in-cluster warnings: %v", err)
	}

	return a
}

const (
	// catches corrupted informer state; see https://github.com/argoproj/argo-cd/issues/4960 for more information
	notObjectErrMsg = "object does not implement the Object interfaces"
)

func (server *ArgoCDServer) healthCheck(r *http.Request) error {
	if server.terminateRequested.Load() {
		return errors.New("API Server is terminating and unable to serve requests")
	}
	if !server.available.Load() {
		return errors.New("API Server is not available: it either hasn't started or is restarting")
	}
	if val, ok := r.URL.Query()["full"]; ok && len(val) > 0 && val[0] == "true" {
		argoDB := db.NewDB(server.Namespace, server.settingsMgr, server.KubeClientset)
		_, err := argoDB.ListClusters(r.Context())
		if err != nil && strings.Contains(err.Error(), notObjectErrMsg) {
			return err
		}
	}
	return nil
}

type Listeners struct {
	Main        net.Listener
	Metrics     net.Listener
	GatewayConn *grpc.ClientConn
}

func (l *Listeners) Close() error {
	if l.Main != nil {
		if err := l.Main.Close(); err != nil {
			return err
		}
		l.Main = nil
	}
	if l.Metrics != nil {
		if err := l.Metrics.Close(); err != nil {
			return err
		}
		l.Metrics = nil
	}
	if l.GatewayConn != nil {
		if err := l.GatewayConn.Close(); err != nil {
			return err
		}
		l.GatewayConn = nil
	}
	return nil
}

// logInClusterWarnings checks the in-cluster configuration and prints out any warnings.
func (server *ArgoCDServer) logInClusterWarnings() error {
	labelSelector := labels.NewSelector()
	req, err := labels.NewRequirement(common.LabelKeySecretType, selection.Equals, []string{common.LabelValueSecretTypeCluster})
	if err != nil {
		return fmt.Errorf("failed to construct cluster-type label selector: %w", err)
	}
	labelSelector = labelSelector.Add(*req)
	secretsLister, err := server.settingsMgr.GetSecretsLister()
	if err != nil {
		return fmt.Errorf("failed to get secrets lister: %w", err)
	}
	clusterSecrets, err := secretsLister.Secrets(server.ArgoCDServerOpts.Namespace).List(labelSelector)
	if err != nil {
		return fmt.Errorf("failed to list cluster secrets: %w", err)
	}
	var inClusterSecrets []string
	for _, clusterSecret := range clusterSecrets {
		cluster, err := db.SecretToCluster(clusterSecret)
		if err != nil {
			return fmt.Errorf("could not unmarshal cluster secret %q: %w", clusterSecret.Name, err)
		}
		if cluster.Server == v1alpha1.KubernetesInternalAPIServerAddr {
			inClusterSecrets = append(inClusterSecrets, clusterSecret.Name)
		}
	}
	if len(inClusterSecrets) > 0 {
		// Don't make this call unless we actually have in-cluster secrets, to save time.
		dbSettings, err := server.settingsMgr.GetSettings()
		if err != nil {
			return fmt.Errorf("could not get DB settings: %w", err)
		}
		if !dbSettings.InClusterEnabled {
			for _, clusterName := range inClusterSecrets {
				log.Warnf("cluster %q uses in-cluster server address but it's disabled in Argo CD settings", clusterName)
			}
		}
	}
	return nil
}

func startListener(host string, port int) (net.Listener, error) {
	var conn net.Listener
	var realErr error
	_ = wait.ExponentialBackoff(backoff, func() (bool, error) {
		conn, realErr = net.Listen("tcp", fmt.Sprintf("%s:%d", host, port))
		if realErr != nil {
			return false, nil
		}
		return true, nil
	})
	return conn, realErr
}

func (server *ArgoCDServer) Listen() (*Listeners, error) {
	mainLn, err := startListener(server.ListenHost, server.ListenPort)
	if err != nil {
		return nil, err
	}
	metricsLn, err := startListener(server.ListenHost, server.MetricsPort)
	if err != nil {
		utilio.Close(mainLn)
		return nil, err
	}
	var dOpts []grpc.DialOption
	dOpts = append(dOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(apiclient.MaxGRPCMessageSize)))
	dOpts = append(dOpts, grpc.WithUserAgent(fmt.Sprintf("%s/%s", common.ArgoCDUserAgentName, common.GetVersion().Version)))
	dOpts = append(dOpts, grpc.WithUnaryInterceptor(grpc_util.OTELUnaryClientInterceptor()))
	dOpts = append(dOpts, grpc.WithStreamInterceptor(grpc_util.OTELStreamClientInterceptor()))
	if server.useTLS() {
		// The following sets up the dial Options for grpc-gateway to talk to gRPC server over TLS.
		// grpc-gateway is just translating HTTP/HTTPS requests as gRPC requests over localhost,
		// so we need to supply the same certificates to establish the connections that a normal,
		// external gRPC client would need.
		tlsConfig := server.settings.TLSConfig()
		if server.TLSConfigCustomizer != nil {
			server.TLSConfigCustomizer(tlsConfig)
		}
		tlsConfig.InsecureSkipVerify = true
		dCreds := credentials.NewTLS(tlsConfig)
		dOpts = append(dOpts, grpc.WithTransportCredentials(dCreds))
	} else {
		dOpts = append(dOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	//nolint:staticcheck
	conn, err := grpc.Dial(fmt.Sprintf("localhost:%d", server.ListenPort), dOpts...)
	if err != nil {
		utilio.Close(mainLn)
		utilio.Close(metricsLn)
		return nil, err
	}
	return &Listeners{Main: mainLn, Metrics: metricsLn, GatewayConn: conn}, nil
}

// Init starts informers used by the API server
func (server *ArgoCDServer) Init(ctx context.Context) {
	go server.projInformer.Run(ctx.Done())
	go server.appInformer.Run(ctx.Done())
	go server.appsetInformer.Run(ctx.Done())
	go server.configMapInformer.Run(ctx.Done())
	go server.secretInformer.Run(ctx.Done())
}

// Run runs the API Server
// We use k8s.io/code-generator/cmd/go-to-protobuf to generate the .proto files from the API types.
// k8s.io/ go-to-protobuf uses protoc-gen-gogo, which comes from gogo/protobuf (a fork of
// golang/protobuf).
func (server *ArgoCDServer) Run(ctx context.Context, listeners *Listeners) {
	defer func() {
		if r := recover(); r != nil {
			log.WithField("trace", string(debug.Stack())).Error("Recovered from panic: ", r)
			server.terminateRequested.Store(true)
			server.Shutdown()
		}
	}()

	metricsServ := metrics.NewMetricsServer(server.MetricsHost, server.MetricsPort)
	if server.RedisClient != nil {
		cacheutil.CollectMetrics(server.RedisClient, metricsServ, server.userStateStorage.GetLockObject())
	}

	// Don't init storage until after CollectMetrics. CollectMetrics adds hooks to the Redis client, and Init
	// reads those hooks. If this is called first, there may be a data race.
	server.userStateStorage.Init(ctx)

	svcSet := newArgoCDServiceSet(server)
	server.serviceSet = svcSet
	grpcS, appResourceTreeFn := server.newGRPCServer()
	grpcWebS := grpcweb.WrapServer(grpcS)
	var httpS *http.Server
	var httpsS *http.Server
	if server.useTLS() {
		httpS = newRedirectServer(server.ListenPort, server.RootPath)
		httpsS = server.newHTTPServer(ctx, server.ListenPort, grpcWebS, appResourceTreeFn, listeners.GatewayConn, metricsServ)
	} else {
		httpS = server.newHTTPServer(ctx, server.ListenPort, grpcWebS, appResourceTreeFn, listeners.GatewayConn, metricsServ)
	}
	if server.RootPath != "" {
		httpS.Handler = withRootPath(httpS.Handler, server)

		if httpsS != nil {
			httpsS.Handler = withRootPath(httpsS.Handler, server)
		}
	}
	httpS.Handler = &bug21955Workaround{handler: httpS.Handler}
	if httpsS != nil {
		httpsS.Handler = &bug21955Workaround{handler: httpsS.Handler}
	}

	// CMux is used to support servicing gRPC and HTTP1.1+JSON on the same port
	tcpm := cmux.New(listeners.Main)
	var tlsm cmux.CMux
	var grpcL net.Listener
	var httpL net.Listener
	var httpsL net.Listener
	if !server.useTLS() {
		httpL = tcpm.Match(cmux.HTTP1Fast("PATCH"))
		grpcL = tcpm.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	} else {
		// We first match on HTTP 1.1 methods.
		httpL = tcpm.Match(cmux.HTTP1Fast("PATCH"))

		// If not matched, we assume that its TLS.
		tlsl := tcpm.Match(cmux.Any())
		tlsConfig := tls.Config{
			// Advertise that we support both http/1.1 and http2 for application level communication.
			// By putting http/1.1 first, we ensure that HTTPS clients will use http/1.1, which is the only
			// protocol our server supports for HTTPS clients. By including h2 in the list, we ensure that
			// gRPC clients know we support http2 for their communication.
			NextProtos: []string{"http/1.1", "h2"},
		}
		tlsConfig.GetCertificate = func(_ *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return server.settings.Certificate, nil
		}
		if server.TLSConfigCustomizer != nil {
			server.TLSConfigCustomizer(&tlsConfig)
		}
		tlsl = tls.NewListener(tlsl, &tlsConfig)

		// Now, we build another mux recursively to match HTTPS and gRPC.
		tlsm = cmux.New(tlsl)
		httpsL = tlsm.Match(cmux.HTTP1Fast("PATCH"))
		grpcL = tlsm.MatchWithWriters(cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"))
	}

	// Start the muxed listeners for our servers
	log.Infof("argocd %s serving on port %d (url: %s, tls: %v, namespace: %s, sso: %v)",
		common.GetVersion(), server.ListenPort, server.settings.URL, server.useTLS(), server.Namespace, server.settings.IsSSOConfigured())
	log.Infof("Enabled application namespace patterns: %s", server.allowedApplicationNamespacesAsString())

	go func() { server.checkServeErr("grpcS", grpcS.Serve(grpcL)) }()
	go func() { server.checkServeErr("httpS", httpS.Serve(httpL)) }()
	if server.useTLS() {
		go func() { server.checkServeErr("httpsS", httpsS.Serve(httpsL)) }()
		go func() { server.checkServeErr("tlsm", tlsm.Serve()) }()
	}
	go server.watchSettings()
	go server.rbacPolicyLoader(ctx)
	go func() { server.checkServeErr("tcpm", tcpm.Serve()) }()
	go func() { server.checkServeErr("metrics", metricsServ.Serve(listeners.Metrics)) }()
	if !cache.WaitForCacheSync(ctx.Done(), server.projInformer.HasSynced, server.appInformer.HasSynced) {
		log.Fatal("Timed out waiting for project cache to sync")
	}

	shutdownFunc := func() {
		log.Info("API Server shutdown initiated. Shutting down servers...")
		server.available.Store(false)
		shutdownCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		var wg gosync.WaitGroup

		// Shutdown http server
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := httpS.Shutdown(shutdownCtx)
			if err != nil {
				log.Errorf("Error shutting down http server: %s", err)
			}
		}()

		if server.useTLS() {
			// Shutdown https server
			wg.Add(1)
			go func() {
				defer wg.Done()
				err := httpsS.Shutdown(shutdownCtx)
				if err != nil {
					log.Errorf("Error shutting down https server: %s", err)
				}
			}()
		}

		// Shutdown gRPC server
		wg.Add(1)
		go func() {
			defer wg.Done()
			grpcS.GracefulStop()
		}()

		// Shutdown metrics server
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := metricsServ.Shutdown(shutdownCtx)
			if err != nil {
				log.Errorf("Error shutting down metrics server: %s", err)
			}
		}()

		if server.useTLS() {
			// Shutdown tls server
			wg.Add(1)
			go func() {
				defer wg.Done()
				tlsm.Close()
			}()
		}

		// Shutdown tcp server
		wg.Add(1)
		go func() {
			defer wg.Done()
			tcpm.Close()
		}()

		c := make(chan struct{})
		// This goroutine will wait for all servers to conclude the shutdown
		// process
		go func() {
			defer close(c)
			wg.Wait()
		}()

		select {
		case <-c:
			log.Info("All servers were gracefully shutdown. Exiting...")
		case <-shutdownCtx.Done():
			log.Warn("Graceful shutdown timeout. Exiting...")
		}
	}
	server.Shutdown = shutdownFunc
	signal.Notify(server.stopCh, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	server.available.Store(true)

	select {
	case signal := <-server.stopCh:
		log.Infof("API Server received signal: %s", signal.String())
		gracefulRestartSignal := GracefulRestartSignal{}
		if signal != gracefulRestartSignal {
			server.terminateRequested.Store(true)
		}
		server.Shutdown()
	case <-ctx.Done():
		log.Infof("API Server: %s", ctx.Err())
		server.terminateRequested.Store(true)
		server.Shutdown()
	}
}

func (server *ArgoCDServer) Initialized() bool {
	return server.projInformer.HasSynced() && server.appInformer.HasSynced()
}

// TerminateRequested returns whether a shutdown was initiated by a signal or context cancel
// as opposed to a watch.
func (server *ArgoCDServer) TerminateRequested() bool {
	return server.terminateRequested.Load()
}

// checkServeErr checks the error from a .Serve() call to decide if it was a graceful shutdown
func (server *ArgoCDServer) checkServeErr(name string, err error) {
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Errorf("Error received from server %s: %v", name, err)
	} else {
		log.Infof("Graceful shutdown of %s initiated", name)
	}
}

func checkOIDCConfigChange(currentOIDCConfig *settings_util.OIDCConfig, newArgoCDSettings *settings_util.ArgoCDSettings) bool {
	newOIDCConfig := newArgoCDSettings.OIDCConfig()

	if (currentOIDCConfig != nil && newOIDCConfig == nil) || (currentOIDCConfig == nil && newOIDCConfig != nil) {
		return true
	}

	if currentOIDCConfig != nil && newOIDCConfig != nil {
		if !reflect.DeepEqual(*currentOIDCConfig, *newOIDCConfig) {
			return true
		}
	}

	return false
}

// watchSettings watches the configmap and secret for any setting updates that would warrant a
// restart of the API server.
func (server *ArgoCDServer) watchSettings() {
	updateCh := make(chan *settings_util.ArgoCDSettings, 1)
	server.settingsMgr.Subscribe(updateCh)

	prevURL := server.settings.URL
	prevAdditionalURLs := server.settings.AdditionalURLs
	prevOIDCConfig := server.settings.OIDCConfig()
	prevDexCfgBytes, err := dexutil.GenerateDexConfigYAML(server.settings, server.DexTLSConfig == nil || server.DexTLSConfig.DisableTLS)
	errorsutil.CheckError(err)
	prevGitHubSecret := server.settings.WebhookGitHubSecret
	prevGitLabSecret := server.settings.WebhookGitLabSecret
	prevBitbucketUUID := server.settings.WebhookBitbucketUUID
	prevBitbucketServerSecret := server.settings.WebhookBitbucketServerSecret
	prevGogsSecret := server.settings.WebhookGogsSecret
	prevExtConfig := server.settings.ExtensionConfig
	var prevCert, prevCertKey string
	if server.settings.Certificate != nil && !server.Insecure {
		prevCert, prevCertKey = tlsutil.EncodeX509KeyPairString(*server.settings.Certificate)
	}

	for {
		newSettings := <-updateCh
		server.settings = newSettings
		newDexCfgBytes, err := dexutil.GenerateDexConfigYAML(server.settings, server.DexTLSConfig == nil || server.DexTLSConfig.DisableTLS)
		errorsutil.CheckError(err)
		if !bytes.Equal(newDexCfgBytes, prevDexCfgBytes) {
			log.Infof("dex config modified. restarting")
			break
		}
		if checkOIDCConfigChange(prevOIDCConfig, server.settings) {
			log.Infof("oidc config modified. restarting")
			break
		}
		if prevURL != server.settings.URL {
			log.Infof("url modified. restarting")
			break
		}
		if !reflect.DeepEqual(prevAdditionalURLs, server.settings.AdditionalURLs) {
			log.Infof("additionalURLs modified. restarting")
			break
		}
		if prevGitHubSecret != server.settings.WebhookGitHubSecret {
			log.Infof("github secret modified. restarting")
			break
		}
		if prevGitLabSecret != server.settings.WebhookGitLabSecret {
			log.Infof("gitlab secret modified. restarting")
			break
		}
		if prevBitbucketUUID != server.settings.WebhookBitbucketUUID {
			log.Infof("bitbucket uuid modified. restarting")
			break
		}
		if prevBitbucketServerSecret != server.settings.WebhookBitbucketServerSecret {
			log.Infof("bitbucket server secret modified. restarting")
			break
		}
		if prevGogsSecret != server.settings.WebhookGogsSecret {
			log.Infof("gogs secret modified. restarting")
			break
		}
		if !reflect.DeepEqual(prevExtConfig, server.settings.ExtensionConfig) {
			prevExtConfig = server.settings.ExtensionConfig
			log.Infof("extensions configs modified. Updating proxy registry...")
			err := server.extensionManager.UpdateExtensionRegistry(server.settings)
			if err != nil {
				log.Errorf("error updating extensions configs: %s", err)
			} else {
				log.Info("extensions configs updated successfully")
			}
		}
		if !server.Insecure {
			var newCert, newCertKey string
			if server.settings.Certificate != nil {
				newCert, newCertKey = tlsutil.EncodeX509KeyPairString(*server.settings.Certificate)
			}
			if newCert != prevCert || newCertKey != prevCertKey {
				log.Infof("tls certificate modified. reloading certificate")
				// No need to break out of this loop since TlsConfig.GetCertificate will automagically reload the cert.
			}
		}
	}
	log.Info("shutting down settings watch")
	server.settingsMgr.Unsubscribe(updateCh)
	close(updateCh)
	// Triggers server restart
	server.stopCh <- GracefulRestartSignal{}
}

func (server *ArgoCDServer) rbacPolicyLoader(ctx context.Context) {
	err := server.enf.RunPolicyLoader(ctx, func(cm *corev1.ConfigMap) error {
		var scopes []string
		if scopesStr, ok := cm.Data[rbac.ConfigMapScopesKey]; scopesStr != "" && ok {
			scopes = make([]string, 0)
			err := yaml.Unmarshal([]byte(scopesStr), &scopes)
			if err != nil {
				return fmt.Errorf("error unmarshalling scopes: %w", err)
			}
		}

		server.policyEnforcer.SetScopes(scopes)
		return nil
	})
	errorsutil.CheckError(err)
}

func (server *ArgoCDServer) useTLS() bool {
	if server.Insecure || server.settings.Certificate == nil {
		return false
	}
	return true
}

func (server *ArgoCDServer) newGRPCServer() (*grpc.Server, application.AppResourceTreeFn) {
	var serverMetricsOptions []grpc_prometheus.ServerMetricsOption
	if enableGRPCTimeHistogram {
		serverMetricsOptions = append(serverMetricsOptions, grpc_prometheus.WithServerHandlingTimeHistogram())
	}
	serverMetrics := grpc_prometheus.NewServerMetrics(serverMetricsOptions...)
	reg := prometheus.NewRegistry()
	reg.MustRegister(serverMetrics)

	sOpts := []grpc.ServerOption{
		// Set the both send and receive the bytes limit to be 100MB
		// The proper way to achieve high performance is to have pagination
		// while we work toward that, we can have high limit first
		grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.MaxSendMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.ConnectionTimeout(300 * time.Second),
		grpc.KeepaliveEnforcementPolicy(
			keepalive.EnforcementPolicy{
				MinTime: common.GetGRPCKeepAliveEnforcementMinimum(),
			},
		),
	}
	sensitiveMethods := map[string]bool{
		"/cluster.ClusterService/Create":                               true,
		"/cluster.ClusterService/Update":                               true,
		"/session.SessionService/Create":                               true,
		"/account.AccountService/UpdatePassword":                       true,
		"/gpgkey.GPGKeyService/CreateGnuPGPublicKey":                   true,
		"/repository.RepositoryService/Create":                         true,
		"/repository.RepositoryService/Update":                         true,
		"/repository.RepositoryService/CreateRepository":               true,
		"/repository.RepositoryService/UpdateRepository":               true,
		"/repository.RepositoryService/ValidateAccess":                 true,
		"/repocreds.RepoCredsService/CreateRepositoryCredentials":      true,
		"/repocreds.RepoCredsService/UpdateRepositoryCredentials":      true,
		"/repository.RepositoryService/CreateWriteRepository":          true,
		"/repository.RepositoryService/UpdateWriteRepository":          true,
		"/repository.RepositoryService/ValidateWriteAccess":            true,
		"/repocreds.RepoCredsService/CreateWriteRepositoryCredentials": true,
		"/repocreds.RepoCredsService/UpdateWriteRepositoryCredentials": true,
		"/application.ApplicationService/PatchResource":                true,
		// Remove from logs both because the contents are sensitive and because they may be very large.
		"/application.ApplicationService/GetManifestsWithFiles": true,
	}
	// NOTE: notice we do not configure the gRPC server here with TLS (e.g. grpc.Creds(creds))
	// This is because TLS handshaking occurs in cmux handling
	sOpts = append(sOpts, grpc.ChainStreamInterceptor(
		otelgrpc.StreamServerInterceptor(), //nolint:staticcheck // TODO: ignore SA1019 for depreciation: see https://github.com/argoproj/argo-cd/issues/18258
		logging.StreamServerInterceptor(grpc_util.InterceptorLogger(server.log)),
		serverMetrics.StreamServerInterceptor(),
		grpc_auth.StreamServerInterceptor(server.Authenticate),
		grpc_util.UserAgentStreamServerInterceptor(common.ArgoCDUserAgentName, clientConstraint),
		grpc_util.PayloadStreamServerInterceptor(server.log, true, func(_ context.Context, c interceptors.CallMeta) bool {
			return !sensitiveMethods[c.FullMethod()]
		}),
		grpc_util.ErrorCodeK8sStreamServerInterceptor(),
		grpc_util.ErrorCodeGitStreamServerInterceptor(),
		recovery.StreamServerInterceptor(recovery.WithRecoveryHandler(grpc_util.LoggerRecoveryHandler(server.log))),
	))
	sOpts = append(sOpts, grpc.ChainUnaryInterceptor(
		bug21955WorkaroundInterceptor,
		otelgrpc.UnaryServerInterceptor(), //nolint:staticcheck // TODO: ignore SA1019 for depreciation: see https://github.com/argoproj/argo-cd/issues/18258
		logging.UnaryServerInterceptor(grpc_util.InterceptorLogger(server.log)),
		serverMetrics.UnaryServerInterceptor(),
		grpc_auth.UnaryServerInterceptor(server.Authenticate),
		grpc_util.UserAgentUnaryServerInterceptor(common.ArgoCDUserAgentName, clientConstraint),
		grpc_util.PayloadUnaryServerInterceptor(server.log, true, func(_ context.Context, c interceptors.CallMeta) bool {
			return !sensitiveMethods[c.FullMethod()]
		}),
		grpc_util.ErrorCodeK8sUnaryServerInterceptor(),
		grpc_util.ErrorCodeGitUnaryServerInterceptor(),
		recovery.UnaryServerInterceptor(recovery.WithRecoveryHandler(grpc_util.LoggerRecoveryHandler(server.log))),
	))
	grpcS := grpc.NewServer(sOpts...)

	versionpkg.RegisterVersionServiceServer(grpcS, server.serviceSet.VersionService)
	clusterpkg.RegisterClusterServiceServer(grpcS, server.serviceSet.ClusterService)
	applicationpkg.RegisterApplicationServiceServer(grpcS, server.serviceSet.ApplicationService)
	applicationsetpkg.RegisterApplicationSetServiceServer(grpcS, server.serviceSet.ApplicationSetService)
	notificationpkg.RegisterNotificationServiceServer(grpcS, server.serviceSet.NotificationService)
	repositorypkg.RegisterRepositoryServiceServer(grpcS, server.serviceSet.RepoService)
	repocredspkg.RegisterRepoCredsServiceServer(grpcS, server.serviceSet.RepoCredsService)
	sessionpkg.RegisterSessionServiceServer(grpcS, server.serviceSet.SessionService)
	settingspkg.RegisterSettingsServiceServer(grpcS, server.serviceSet.SettingsService)
	projectpkg.RegisterProjectServiceServer(grpcS, server.serviceSet.ProjectService)
	accountpkg.RegisterAccountServiceServer(grpcS, server.serviceSet.AccountService)
	certificatepkg.RegisterCertificateServiceServer(grpcS, server.serviceSet.CertificateService)
	gpgkeypkg.RegisterGPGKeyServiceServer(grpcS, server.serviceSet.GpgkeyService)
	// Register reflection service on gRPC server.
	reflection.Register(grpcS)
	serverMetrics.InitializeMetrics(grpcS)
	errorsutil.CheckError(server.serviceSet.ProjectService.NormalizeProjs())
	return grpcS, server.serviceSet.AppResourceTreeFn
}

type ArgoCDServiceSet struct {
	ClusterService        *cluster.Server
	RepoService           *repository.Server
	RepoCredsService      *repocreds.Server
	SessionService        *session.Server
	ApplicationService    applicationpkg.ApplicationServiceServer
	AppResourceTreeFn     application.AppResourceTreeFn
	ApplicationSetService applicationsetpkg.ApplicationSetServiceServer
	ProjectService        *project.Server
	SettingsService       *settings.Server
	AccountService        *account.Server
	NotificationService   notificationpkg.NotificationServiceServer
	CertificateService    *certificate.Server
	GpgkeyService         *gpgkey.Server
	VersionService        *version.Server
}

func newArgoCDServiceSet(a *ArgoCDServer) *ArgoCDServiceSet {
	kubectl := kubeutil.NewKubectl()
	clusterService := cluster.NewServer(a.db, a.enf, a.Cache, kubectl)
	repoService := repository.NewServer(a.RepoClientset, a.db, a.enf, a.Cache, a.appLister, a.projInformer, a.Namespace, a.settingsMgr, a.HydratorEnabled)
	repoCredsService := repocreds.NewServer(a.db, a.enf)
	var loginRateLimiter func() (utilio.Closer, error)
	if maxConcurrentLoginRequestsCount > 0 {
		loginRateLimiter = session.NewLoginRateLimiter(maxConcurrentLoginRequestsCount)
	}
	sessionService := session.NewServer(a.sessionMgr, a.settingsMgr, a, a.policyEnforcer, loginRateLimiter)
	projectLock := sync.NewKeyLock()
	applicationService, appResourceTreeFn := application.NewServer(
		a.Namespace,
		a.KubeClientset,
		a.AppClientset,
		a.appLister,
		a.appInformer,
		nil,
		a.RepoClientset,
		a.Cache,
		kubectl,
		a.db,
		a.enf,
		projectLock,
		a.settingsMgr,
		a.projInformer,
		a.ApplicationNamespaces,
		a.EnableK8sEvent,
		a.SyncWithReplaceAllowed,
	)

	applicationSetService := applicationset.NewServer(
		a.db,
		a.KubeClientset,
		a.DynamicClientset,
		a.KubeControllerClientset,
		a.enf,
		a.RepoClientset,
		a.AppClientset,
		a.appsetInformer,
		a.appsetLister,
		a.Namespace,
		projectLock,
		a.ApplicationNamespaces,
		a.GitSubmoduleEnabled,
		a.EnableNewGitFileGlobbing,
		a.ScmRootCAPath,
		a.AllowedScmProviders,
		a.EnableScmProviders,
		a.EnableK8sEvent,
	)

	projectService := project.NewServer(a.Namespace, a.KubeClientset, a.AppClientset, a.enf, projectLock, a.sessionMgr, a.policyEnforcer, a.projInformer, a.settingsMgr, a.db, a.EnableK8sEvent)
	appsInAnyNamespaceEnabled := len(a.ApplicationNamespaces) > 0
	settingsService := settings.NewServer(a.settingsMgr, a.RepoClientset, a, a.DisableAuth, appsInAnyNamespaceEnabled, a.HydratorEnabled)
	accountService := account.NewServer(a.sessionMgr, a.settingsMgr, a.enf)

	notificationService := notification.NewServer(a.apiFactory)
	certificateService := certificate.NewServer(a.db, a.enf)
	gpgkeyService := gpgkey.NewServer(a.db, a.enf)
	versionService := version.NewServer(a, func() (bool, error) {
		if a.DisableAuth {
			return true, nil
		}
		sett, err := a.settingsMgr.GetSettings()
		if err != nil {
			return false, err
		}
		return sett.AnonymousUserEnabled, err
	})

	return &ArgoCDServiceSet{
		ClusterService:        clusterService,
		RepoService:           repoService,
		RepoCredsService:      repoCredsService,
		SessionService:        sessionService,
		ApplicationService:    applicationService,
		AppResourceTreeFn:     appResourceTreeFn,
		ApplicationSetService: applicationSetService,
		ProjectService:        projectService,
		SettingsService:       settingsService,
		AccountService:        accountService,
		NotificationService:   notificationService,
		CertificateService:    certificateService,
		GpgkeyService:         gpgkeyService,
		VersionService:        versionService,
	}
}

// translateGrpcCookieHeader conditionally sets a cookie on the response.
func (server *ArgoCDServer) translateGrpcCookieHeader(ctx context.Context, w http.ResponseWriter, resp golang_proto.Message) error {
	if sessionResp, ok := resp.(*sessionpkg.SessionResponse); ok {
		token := sessionResp.Token
		err := server.setTokenCookie(token, w)
		if err != nil {
			return fmt.Errorf("error setting token cookie from session response: %w", err)
		}
	} else if md, ok := runtime.ServerMetadataFromContext(ctx); ok {
		renewToken := md.HeaderMD[renewTokenKey]
		if len(renewToken) > 0 {
			return server.setTokenCookie(renewToken[0], w)
		}
	}

	return nil
}

func (server *ArgoCDServer) setTokenCookie(token string, w http.ResponseWriter) error {
	cookiePath := "path=/" + strings.TrimRight(strings.TrimLeft(server.BaseHRef, "/"), "/")
	flags := []string{cookiePath, "SameSite=lax", "httpOnly"}
	if !server.Insecure {
		flags = append(flags, "Secure")
	}
	cookies, err := httputil.MakeCookieMetadata(common.AuthCookieName, token, flags...)
	if err != nil {
		return fmt.Errorf("error creating cookie metadata: %w", err)
	}
	for _, cookie := range cookies {
		w.Header().Add("Set-Cookie", cookie)
	}
	return nil
}

func withRootPath(handler http.Handler, a *ArgoCDServer) http.Handler {
	// If RootPath is empty, directly return the original handler
	if a.RootPath == "" {
		return handler
	}

	// get rid of slashes
	root := strings.Trim(a.RootPath, "/")

	mux := http.NewServeMux()
	mux.Handle("/"+root+"/", http.StripPrefix("/"+root, handler))

	healthz.ServeHealthCheck(mux, a.healthCheck)

	return mux
}

func compressHandler(handler http.Handler) http.Handler {
	compr := handlers.CompressHandler(handler)
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Accept") == "text/event-stream" {
			handler.ServeHTTP(writer, request)
		} else {
			compr.ServeHTTP(writer, request)
		}
	})
}

// newHTTPServer returns the HTTP server to serve HTTP/HTTPS requests. This is implemented
// using grpc-gateway as a proxy to the gRPC server.
func (server *ArgoCDServer) newHTTPServer(ctx context.Context, port int, grpcWebHandler http.Handler, appResourceTreeFn application.AppResourceTreeFn, conn *grpc.ClientConn, metricsReg HTTPMetricsRegistry) *http.Server {
	endpoint := fmt.Sprintf("localhost:%d", port)
	mux := http.NewServeMux()
	httpS := http.Server{
		Addr: endpoint,
		Handler: &handlerSwitcher{
			handler: mux,
			urlToHandler: map[string]http.Handler{
				"/api/badge":          badge.NewHandler(server.AppClientset, server.settingsMgr, server.Namespace, server.ApplicationNamespaces),
				common.LogoutEndpoint: logout.NewHandler(server.settingsMgr, server.sessionMgr, server.RootPath, server.BaseHRef),
			},
			contentTypeToHandler: map[string]http.Handler{
				"application/grpc-web+proto": grpcWebHandler,
			},
		},
	}

	// HTTP 1.1+JSON Server
	// grpc-ecosystem/grpc-gateway is used to proxy HTTP requests to the corresponding gRPC call
	// NOTE: if a marshaller option is not supplied, grpc-gateway will default to the jsonpb from
	// golang/protobuf. Which does not support types such as time.Time. gogo/protobuf does support
	// time.Time, but does not support custom UnmarshalJSON() and MarshalJSON() methods. Therefore
	// we use our own Marshaler
	gwMuxOpts := runtime.WithMarshalerOption(runtime.MIMEWildcard, new(grpc_util.JSONMarshaler))
	gwCookieOpts := runtime.WithForwardResponseOption(server.translateGrpcCookieHeader)
	gwmux := runtime.NewServeMux(gwMuxOpts, gwCookieOpts)

	var handler http.Handler = gwmux
	if server.EnableGZip {
		handler = compressHandler(handler)
	}
	if len(server.ContentTypes) > 0 {
		handler = enforceContentTypes(handler, server.ContentTypes)
	} else {
		log.WithField(common.SecurityField, common.SecurityHigh).Warnf("Content-Type enforcement is disabled, which may make your API vulnerable to CSRF attacks")
	}
	mux.Handle("/api/", handler)

	terminalOpts := application.TerminalOptions{DisableAuth: server.DisableAuth, Enf: server.enf}

	terminal := application.NewHandler(server.appLister, server.Namespace, server.ApplicationNamespaces, server.db, appResourceTreeFn, server.settings.ExecShells, server.sessionMgr, &terminalOpts).
		WithFeatureFlagMiddleware(server.settingsMgr.GetSettings)
	th := util_session.WithAuthMiddleware(server.DisableAuth, server.sessionMgr, terminal)
	mux.Handle("/terminal", th)

	// Proxy extension is currently an alpha feature and is disabled
	// by default.
	if server.EnableProxyExtension {
		// API server won't panic if extensions fail to register. In
		// this case an error log will be sent and no extension route
		// will be added in mux.
		registerExtensions(mux, server, metricsReg)
	}

	mustRegisterGWHandler(ctx, versionpkg.RegisterVersionServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, clusterpkg.RegisterClusterServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, applicationpkg.RegisterApplicationServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, applicationsetpkg.RegisterApplicationSetServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, notificationpkg.RegisterNotificationServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, repositorypkg.RegisterRepositoryServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, repocredspkg.RegisterRepoCredsServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, sessionpkg.RegisterSessionServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, settingspkg.RegisterSettingsServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, projectpkg.RegisterProjectServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, accountpkg.RegisterAccountServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, certificatepkg.RegisterCertificateServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, gpgkeypkg.RegisterGPGKeyServiceHandler, gwmux, conn)

	// Swagger UI
	swagger.ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui", server.RootPath)
	healthz.ServeHealthCheck(mux, server.healthCheck)

	// Dex reverse proxy and client app and OAuth2 login/callback
	server.registerDexHandlers(mux)

	// Webhook handler for git events (Note: cache timeouts are hardcoded because API server does not write to cache and not really using them)
	argoDB := db.NewDB(server.Namespace, server.settingsMgr, server.KubeClientset)
	acdWebhookHandler := webhook.NewHandler(server.Namespace, server.ApplicationNamespaces, server.WebhookParallelism, server.AppClientset, server.settings, server.settingsMgr, server.RepoServerCache, server.Cache, argoDB, server.settingsMgr.GetMaxWebhookPayloadSize())

	mux.HandleFunc("/api/webhook", acdWebhookHandler.Handler)

	// Serve cli binaries directly from API server
	registerDownloadHandlers(mux, "/download")

	// Serve extensions
	extensionsSharedPath := "/tmp/extensions/"

	var extensionsHandler http.Handler = http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		server.serveExtensions(extensionsSharedPath, writer)
	})
	if server.EnableGZip {
		extensionsHandler = compressHandler(extensionsHandler)
	}
	mux.Handle("/extensions.js", extensionsHandler)

	// Serve UI static assets
	var assetsHandler http.Handler = http.HandlerFunc(server.newStaticAssetsHandler())
	if server.EnableGZip {
		assetsHandler = compressHandler(assetsHandler)
	}
	mux.Handle("/", assetsHandler)
	return &httpS
}

func enforceContentTypes(handler http.Handler, types []string) http.Handler {
	allowedTypes := map[string]bool{}
	for _, t := range types {
		allowedTypes[strings.ToLower(t)] = true
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || allowedTypes[strings.ToLower(r.Header.Get("Content-Type"))] {
			handler.ServeHTTP(w, r)
		} else {
			http.Error(w, "Invalid content type", http.StatusUnsupportedMediaType)
		}
	})
}

// registerExtensions will try to register all configured extensions
// in the given mux. If any error is returned while registering
// extensions handlers, no route will be added in the given mux.
func registerExtensions(mux *http.ServeMux, a *ArgoCDServer, metricsReg HTTPMetricsRegistry) {
	a.log.Info("Registering extensions...")
	extHandler := http.HandlerFunc(a.extensionManager.CallExtension())
	authMiddleware := a.sessionMgr.AuthMiddlewareFunc(a.DisableAuth)
	// auth middleware ensures that requests to all extensions are authenticated first
	mux.Handle(extension.URLPrefix+"/", authMiddleware(extHandler))

	a.extensionManager.AddMetricsRegistry(metricsReg)

	err := a.extensionManager.RegisterExtensions()
	if err != nil {
		a.log.Errorf("Error registering extensions: %s", err)
	}
}

var extensionsPattern = regexp.MustCompile(`^extension(.*)\.js$`)

func (server *ArgoCDServer) serveExtensions(extensionsSharedPath string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/javascript")

	err := filepath.Walk(extensionsSharedPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("failed to iterate files in '%s': %w", extensionsSharedPath, err)
		}
		if !files.IsSymlink(info) && !info.IsDir() && extensionsPattern.MatchString(info.Name()) {
			processFile := func() error {
				if _, err = fmt.Fprintf(w, "// source: %s/%s \n", filePath, info.Name()); err != nil {
					return fmt.Errorf("failed to write to response: %w", err)
				}

				f, err := os.Open(filePath)
				if err != nil {
					return fmt.Errorf("failed to open file '%s': %w", filePath, err)
				}
				defer utilio.Close(f)

				if _, err := goio.Copy(w, f); err != nil {
					return fmt.Errorf("failed to copy file '%s': %w", filePath, err)
				}

				return nil
			}

			if processFile() != nil {
				return fmt.Errorf("failed to serve extension file '%s': %w", filePath, processFile())
			}
		}
		return nil
	})

	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		log.Errorf("Failed to walk extensions directory: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
}

// registerDexHandlers will register dex HTTP handlers, creating the OAuth client app
func (server *ArgoCDServer) registerDexHandlers(mux *http.ServeMux) {
	if !server.settings.IsSSOConfigured() {
		return
	}
	// Run dex OpenID Connect Identity Provider behind a reverse proxy (served at /api/dex)
	var err error
	mux.HandleFunc(common.DexAPIEndpoint+"/", dexutil.NewDexHTTPReverseProxy(server.DexServerAddr, server.BaseHRef, server.DexTLSConfig))
	server.ssoClientApp, err = oidc.NewClientApp(server.settings, server.DexServerAddr, server.DexTLSConfig, server.BaseHRef, cacheutil.NewRedisCache(server.RedisClient, server.settings.UserInfoCacheExpiration(), cacheutil.RedisCompressionNone))
	errorsutil.CheckError(err)
	mux.HandleFunc(common.LoginEndpoint, server.ssoClientApp.HandleLogin)
	mux.HandleFunc(common.CallbackEndpoint, server.ssoClientApp.HandleCallback)
}

// newRedirectServer returns an HTTP server which does a 307 redirect to the HTTPS server
func newRedirectServer(port int, rootPath string) *http.Server {
	var addr string
	if rootPath == "" {
		addr = fmt.Sprintf("localhost:%d", port)
	} else {
		addr = fmt.Sprintf("localhost:%d/%s", port, strings.Trim(rootPath, "/"))
	}

	return &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			target := "https://" + req.Host

			if rootPath != "" {
				root := strings.Trim(rootPath, "/")
				prefix := "/" + root

				// If the request path already starts with rootPath, no need to add rootPath again
				if strings.HasPrefix(req.URL.Path, prefix) {
					target += req.URL.Path
				} else {
					target += prefix + req.URL.Path
				}
			} else {
				target += req.URL.Path
			}

			if req.URL.RawQuery != "" {
				target += "?" + req.URL.RawQuery
			}
			http.Redirect(w, req, target, http.StatusTemporaryRedirect)
		}),
	}
}

// registerDownloadHandlers registers HTTP handlers to support downloads directly from the API server
// (e.g. argocd CLI)
func registerDownloadHandlers(mux *http.ServeMux, base string) {
	linuxPath, err := exec.LookPath("argocd")
	if err != nil {
		log.Warnf("argocd not in PATH")
	} else {
		mux.HandleFunc(base+"/argocd-linux-"+go_runtime.GOARCH, func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, linuxPath)
		})
	}
}

func (server *ArgoCDServer) getIndexData() ([]byte, error) {
	server.indexDataInit.Do(func() {
		data, err := ui.Embedded.ReadFile("dist/app/index.html")
		if err != nil {
			server.indexDataErr = err
			return
		}
		if server.BaseHRef == "/" || server.BaseHRef == "" {
			server.indexData = data
		} else {
			server.indexData = []byte(replaceBaseHRef(string(data), fmt.Sprintf(`<base href="/%s/">`, strings.Trim(server.BaseHRef, "/"))))
		}
	})

	return server.indexData, server.indexDataErr
}

func (server *ArgoCDServer) uiAssetExists(filename string) bool {
	f, err := server.staticAssets.Open(strings.Trim(filename, "/"))
	if err != nil {
		return false
	}
	defer utilio.Close(f)
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return !stat.IsDir()
}

// newStaticAssetsHandler returns an HTTP handler to serve UI static assets
func (server *ArgoCDServer) newStaticAssetsHandler() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		acceptHTML := false
		for _, acceptType := range strings.Split(r.Header.Get("Accept"), ",") {
			if acceptType == "text/html" || acceptType == "html" {
				acceptHTML = true
				break
			}
		}

		fileRequest := r.URL.Path != "/index.html" && server.uiAssetExists(r.URL.Path)

		// Set X-Frame-Options according to configuration
		if server.XFrameOptions != "" {
			w.Header().Set("X-Frame-Options", server.XFrameOptions)
		}
		// Set Content-Security-Policy according to configuration
		if server.ContentSecurityPolicy != "" {
			w.Header().Set("Content-Security-Policy", server.ContentSecurityPolicy)
		}
		w.Header().Set("X-XSS-Protection", "1")

		// serve index.html for non file requests to support HTML5 History API
		if acceptHTML && !fileRequest && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
			for k, v := range noCacheHeaders {
				w.Header().Set(k, v)
			}
			data, err := server.getIndexData()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			modTime, err := time.Parse(common.GetVersion().BuildDate, time.RFC3339)
			if err != nil {
				modTime = time.Now()
			}
			http.ServeContent(w, r, "index.html", modTime, utilio.NewByteReadSeeker(data))
		} else {
			if isMainJsBundle(r.URL) {
				cacheControl := "public, max-age=31536000, immutable"
				if !fileRequest {
					cacheControl = "no-cache"
				}
				w.Header().Set("Cache-Control", cacheControl)
			}
			http.FileServer(server.staticAssets).ServeHTTP(w, r)
		}
	}
}

var mainJsBundleRegex = regexp.MustCompile(`^main\.[0-9a-f]{20}\.js$`)

func isMainJsBundle(url *url.URL) bool {
	filename := path.Base(url.Path)
	return mainJsBundleRegex.MatchString(filename)
}

type registerFunc func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error

// mustRegisterGWHandler is a convenience function to register a gateway handler
func mustRegisterGWHandler(ctx context.Context, register registerFunc, mux *runtime.ServeMux, conn *grpc.ClientConn) {
	err := register(ctx, mux, conn)
	if err != nil {
		panic(err)
	}
}

func replaceBaseHRef(data string, replaceWith string) string {
	return baseHRefRegex.ReplaceAllString(data, replaceWith)
}

// Authenticate checks for the presence of a valid token when accessing server-side resources.
func (server *ArgoCDServer) Authenticate(ctx context.Context) (context.Context, error) {
	if server.DisableAuth {
		return ctx, nil
	}
	claims, newToken, claimsErr := server.getClaims(ctx)
	if claims != nil {
		// Add claims to the context to inspect for RBAC
		//nolint:staticcheck
		ctx = context.WithValue(ctx, "claims", claims)
		if newToken != "" {
			// Session tokens that are expiring soon should be regenerated if user stays active.
			// The renewed token is stored in outgoing ServerMetadata. Metadata is available to grpc-gateway
			// response forwarder that will translate it into Set-Cookie header.
			if err := grpc.SendHeader(ctx, metadata.New(map[string]string{renewTokenKey: newToken})); err != nil {
				log.Warnf("Failed to set %s header", renewTokenKey)
			}
		}
	}
	if claimsErr != nil {
		//nolint:staticcheck
		ctx = context.WithValue(ctx, util_session.AuthErrorCtxKey, claimsErr)
	}

	if claimsErr != nil {
		argoCDSettings, err := server.settingsMgr.GetSettings()
		if err != nil {
			return ctx, status.Errorf(codes.Internal, "unable to load settings: %v", err)
		}
		if !argoCDSettings.AnonymousUserEnabled {
			return ctx, claimsErr
		}
		//nolint:staticcheck
		ctx = context.WithValue(ctx, "claims", "")
	}

	return ctx, nil
}

func (server *ArgoCDServer) getClaims(ctx context.Context) (jwt.Claims, string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, "", ErrNoSession
	}
	tokenString := getToken(md)
	if tokenString == "" {
		return nil, "", ErrNoSession
	}
	claims, newToken, err := server.sessionMgr.VerifyToken(tokenString)
	if err != nil {
		return claims, "", status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
	}

	// Some SSO implementations (Okta) require a call to
	// the OIDC user info path to get attributes like groups
	// we assume that everywhere in argocd jwt.MapClaims is used as type for interface jwt.Claims
	// otherwise this would cause a panic
	var groupClaims jwt.MapClaims
	if groupClaims, ok = claims.(jwt.MapClaims); !ok {
		if tmpClaims, ok := claims.(*jwt.MapClaims); ok {
			groupClaims = *tmpClaims
		}
	}
	iss := jwtutil.StringField(groupClaims, "iss")
	if iss != util_session.SessionManagerClaimsIssuer && server.settings.UserInfoGroupsEnabled() && server.settings.UserInfoPath() != "" {
		userInfo, unauthorized, err := server.ssoClientApp.GetUserInfo(groupClaims, server.settings.IssuerURL(), server.settings.UserInfoPath())
		if unauthorized {
			log.Errorf("error while quering userinfo endpoint: %v", err)
			return claims, "", status.Errorf(codes.Unauthenticated, "invalid session")
		}
		if err != nil {
			log.Errorf("error fetching user info endpoint: %v", err)
			return claims, "", status.Errorf(codes.Internal, "invalid userinfo response")
		}
		if groupClaims["sub"] != userInfo["sub"] {
			return claims, "", status.Error(codes.Unknown, "subject of claims from user info endpoint didn't match subject of idToken, see https://openid.net/specs/openid-connect-core-1_0.html#UserInfo")
		}
		groupClaims["groups"] = userInfo["groups"]
	}

	return groupClaims, newToken, nil
}

// getToken extracts the token from gRPC metadata or cookie headers
func getToken(md metadata.MD) string {
	// check the "token" metadata
	{
		tokens, ok := md[apiclient.MetaDataTokenKey]
		if ok && len(tokens) > 0 {
			return tokens[0]
		}
	}

	// looks for the HTTP header `Authorization: Bearer ...`
	// argocd prefers bearer token over cookie
	for _, t := range md["authorization"] {
		token := strings.TrimPrefix(t, "Bearer ")
		if strings.HasPrefix(t, "Bearer ") && jwtutil.IsValid(token) {
			return token
		}
	}

	// check the HTTP cookie
	for _, t := range md["grpcgateway-cookie"] {
		header := http.Header{}
		header.Add("Cookie", t)
		request := http.Request{Header: header}
		token, err := httputil.JoinCookies(common.AuthCookieName, request.Cookies())
		if err == nil && jwtutil.IsValid(token) {
			return token
		}
	}

	return ""
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

// Workaround for https://github.com/golang/go/issues/21955 to support escaped URLs in URL path.
type bug21955Workaround struct {
	handler http.Handler
}

var pathPatters = []*regexp.Regexp{
	regexp.MustCompile(`/api/v1/clusters/[^/]+`),
	regexp.MustCompile(`/api/v1/repositories/[^/]+`),
	regexp.MustCompile(`/api/v1/repocreds/[^/]+`),
	regexp.MustCompile(`/api/v1/repositories/[^/]+/apps`),
	regexp.MustCompile(`/api/v1/repositories/[^/]+/apps/[^/]+`),
	regexp.MustCompile(`/settings/clusters/[^/]+`),
}

func (bf *bug21955Workaround) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, pattern := range pathPatters {
		if pattern.MatchString(r.URL.RawPath) {
			r.URL.Path = r.URL.RawPath
			break
		}
	}
	bf.handler.ServeHTTP(w, r)
}

func bug21955WorkaroundInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	if rq, ok := req.(*repositorypkg.RepoQuery); ok {
		repo, err := url.QueryUnescape(rq.Repo)
		if err != nil {
			return nil, err
		}
		rq.Repo = repo
	} else if rk, ok := req.(*repositorypkg.RepoAppsQuery); ok {
		repo, err := url.QueryUnescape(rk.Repo)
		if err != nil {
			return nil, err
		}
		rk.Repo = repo
	} else if rdq, ok := req.(*repositorypkg.RepoAppDetailsQuery); ok {
		repo, err := url.QueryUnescape(rdq.Source.RepoURL)
		if err != nil {
			return nil, err
		}
		rdq.Source.RepoURL = repo
	} else if ru, ok := req.(*repositorypkg.RepoUpdateRequest); ok {
		repo, err := url.QueryUnescape(ru.Repo.Repo)
		if err != nil {
			return nil, err
		}
		ru.Repo.Repo = repo
	} else if rk, ok := req.(*repocredspkg.RepoCredsQuery); ok {
		pattern, err := url.QueryUnescape(rk.Url)
		if err != nil {
			return nil, err
		}
		rk.Url = pattern
	} else if rk, ok := req.(*repocredspkg.RepoCredsDeleteRequest); ok {
		pattern, err := url.QueryUnescape(rk.Url)
		if err != nil {
			return nil, err
		}
		rk.Url = pattern
	} else if cq, ok := req.(*clusterpkg.ClusterQuery); ok {
		if cq.Id != nil {
			val, err := url.QueryUnescape(cq.Id.Value)
			if err != nil {
				return nil, err
			}
			cq.Id.Value = val
		}
	} else if cu, ok := req.(*clusterpkg.ClusterUpdateRequest); ok {
		if cu.Id != nil {
			val, err := url.QueryUnescape(cu.Id.Value)
			if err != nil {
				return nil, err
			}
			cu.Id.Value = val
		}
	}
	return handler(ctx, req)
}

// allowedApplicationNamespacesAsString returns a string containing comma-separated list
// of allowed application namespaces
func (server *ArgoCDServer) allowedApplicationNamespacesAsString() string {
	ns := server.Namespace
	if len(server.ApplicationNamespaces) > 0 {
		ns += ", "
		ns += strings.Join(server.ApplicationNamespaces, ", ")
	}
	return ns
}
