package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	// nolint:staticcheck
	golang_proto "github.com/golang/protobuf/proto"

	"github.com/argoproj/gitops-engine/pkg/utils/errors"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/pkg/jwt/zjwt"
	"github.com/argoproj/pkg/sync"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
	"github.com/gorilla/handlers"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	log "github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	netCtx "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient"
	accountpkg "github.com/argoproj/argo-cd/pkg/apiclient/account"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	certificatepkg "github.com/argoproj/argo-cd/pkg/apiclient/certificate"
	clusterpkg "github.com/argoproj/argo-cd/pkg/apiclient/cluster"
	gpgkeypkg "github.com/argoproj/argo-cd/pkg/apiclient/gpgkey"
	projectpkg "github.com/argoproj/argo-cd/pkg/apiclient/project"
	repocredspkg "github.com/argoproj/argo-cd/pkg/apiclient/repocreds"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	sessionpkg "github.com/argoproj/argo-cd/pkg/apiclient/session"
	settingspkg "github.com/argoproj/argo-cd/pkg/apiclient/settings"
	versionpkg "github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	repoapiclient "github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/server/account"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/badge"
	servercache "github.com/argoproj/argo-cd/server/cache"
	"github.com/argoproj/argo-cd/server/certificate"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/gpgkey"
	"github.com/argoproj/argo-cd/server/metrics"
	"github.com/argoproj/argo-cd/server/project"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/server/repocreds"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/server/settings"
	"github.com/argoproj/argo-cd/server/version"
	"github.com/argoproj/argo-cd/util/assets"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/dex"
	dexutil "github.com/argoproj/argo-cd/util/dex"
	"github.com/argoproj/argo-cd/util/env"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/healthz"
	httputil "github.com/argoproj/argo-cd/util/http"
	"github.com/argoproj/argo-cd/util/oidc"
	"github.com/argoproj/argo-cd/util/rbac"
	util_session "github.com/argoproj/argo-cd/util/session"
	settings_util "github.com/argoproj/argo-cd/util/settings"
	"github.com/argoproj/argo-cd/util/swagger"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	"github.com/argoproj/argo-cd/util/webhook"
)

const maxConcurrentLoginRequestsCountEnv = "ARGOCD_MAX_CONCURRENT_LOGIN_REQUESTS_COUNT"
const replicasCountEnv = "ARGOCD_API_SERVER_REPLICAS"
const enableGRPCTimeHistogramEnv = "ARGOCD_ENABLE_GRPC_TIME_HISTOGRAM"

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
	clientConstraint = fmt.Sprintf(">= %s", common.MinClientVersion)
	baseHRefRegex    = regexp.MustCompile(`<base href="(.*)">`)
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
	enableGRPCTimeHistogram = os.Getenv(enableGRPCTimeHistogramEnv) != "false"
}

// ArgoCDServer is the API server for Argo CD
type ArgoCDServer struct {
	ArgoCDServerOpts

	ssoClientApp   *oidc.ClientApp
	settings       *settings_util.ArgoCDSettings
	log            *log.Entry
	sessionMgr     *util_session.SessionManager
	settingsMgr    *settings_util.SettingsManager
	enf            *rbac.Enforcer
	projInformer   cache.SharedIndexInformer
	policyEnforcer *rbacpolicy.RBACPolicyEnforcer
	appInformer    cache.SharedIndexInformer
	appLister      applisters.ApplicationNamespaceLister

	// stopCh is the channel which when closed, will shutdown the Argo CD server
	stopCh chan struct{}
}

type ArgoCDServerOpts struct {
	DisableAuth         bool
	EnableGZip          bool
	Insecure            bool
	ListenPort          int
	MetricsPort         int
	Namespace           string
	DexServerAddr       string
	StaticAssetsDir     string
	BaseHRef            string
	RootPath            string
	KubeClientset       kubernetes.Interface
	AppClientset        appclientset.Interface
	RepoClientset       repoapiclient.Clientset
	Cache               *servercache.Cache
	RedisClient         *redis.Client
	TLSConfigCustomizer tlsutil.ConfigCustomizer
	XFrameOptions       string
}

// initializeDefaultProject creates the default project if it does not already exist
func initializeDefaultProject(opts ArgoCDServerOpts) error {
	defaultProj := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: common.DefaultAppProjectName, Namespace: opts.Namespace},
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
func NewServer(ctx context.Context, opts ArgoCDServerOpts) *ArgoCDServer {
	settingsMgr := settings_util.NewSettingsManager(ctx, opts.KubeClientset, opts.Namespace)
	settings, err := settingsMgr.InitializeSettings(opts.Insecure)
	errors.CheckError(err)
	err = initializeDefaultProject(opts)
	errors.CheckError(err)

	sessionMgr := util_session.NewSessionManager(settingsMgr, opts.DexServerAddr, opts.Cache)

	factory := appinformer.NewFilteredSharedInformerFactory(opts.AppClientset, 0, opts.Namespace, func(options *metav1.ListOptions) {})
	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	projLister := factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(opts.Namespace)

	appInformer := factory.Argoproj().V1alpha1().Applications().Informer()
	appLister := factory.Argoproj().V1alpha1().Applications().Lister().Applications(opts.Namespace)

	enf := rbac.NewEnforcer(opts.KubeClientset, opts.Namespace, common.ArgoCDRBACConfigMapName, nil)
	enf.EnableEnforce(!opts.DisableAuth)
	err = enf.SetBuiltinPolicy(assets.BuiltinPolicyCSV)
	errors.CheckError(err)
	enf.EnableLog(os.Getenv(common.EnvVarRBACDebug) == "1")

	policyEnf := rbacpolicy.NewRBACPolicyEnforcer(enf, projLister)
	enf.SetClaimsEnforcerFunc(policyEnf.EnforceClaims)

	return &ArgoCDServer{
		ArgoCDServerOpts: opts,
		log:              log.NewEntry(log.StandardLogger()),
		settings:         settings,
		sessionMgr:       sessionMgr,
		settingsMgr:      settingsMgr,
		enf:              enf,
		projInformer:     projInformer,
		appInformer:      appInformer,
		appLister:        appLister,
		policyEnforcer:   policyEnf,
	}
}

// Run runs the API Server
// We use k8s.io/code-generator/cmd/go-to-protobuf to generate the .proto files from the API types.
// k8s.io/ go-to-protobuf uses protoc-gen-gogo, which comes from gogo/protobuf (a fork of
// golang/protobuf).
func (a *ArgoCDServer) Run(ctx context.Context, port int, metricsPort int) {
	grpcS := a.newGRPCServer()
	grpcWebS := grpcweb.WrapServer(grpcS)
	var httpS *http.Server
	var httpsS *http.Server
	if a.useTLS() {
		httpS = newRedirectServer(port, a.RootPath)
		httpsS = a.newHTTPServer(ctx, port, grpcWebS)
	} else {
		httpS = a.newHTTPServer(ctx, port, grpcWebS)
	}
	if a.RootPath != "" {
		httpS.Handler = withRootPath(httpS.Handler, a)

		if httpsS != nil {
			httpsS.Handler = withRootPath(httpsS.Handler, a)
		}
	}
	httpS.Handler = &bug21955Workaround{handler: httpS.Handler}
	if httpsS != nil {
		httpsS.Handler = &bug21955Workaround{handler: httpsS.Handler}
	}

	metricsServ := metrics.NewMetricsServer(metricsPort)
	if a.RedisClient != nil {
		cacheutil.CollectMetrics(a.RedisClient, metricsServ)
	}

	// Start listener
	var conn net.Listener
	var realErr error
	_ = wait.ExponentialBackoff(backoff, func() (bool, error) {
		conn, realErr = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if realErr != nil {
			a.log.Warnf("failed listen: %v", realErr)
			return false, nil
		}
		return true, nil
	})
	errors.CheckError(realErr)

	// Cmux is used to support servicing gRPC and HTTP1.1+JSON on the same port
	tcpm := cmux.New(conn)
	var tlsm cmux.CMux
	var grpcL net.Listener
	var httpL net.Listener
	var httpsL net.Listener
	if !a.useTLS() {
		httpL = tcpm.Match(cmux.HTTP1Fast())
		grpcL = tcpm.Match(cmux.HTTP2HeaderField("content-type", "application/grpc"))
	} else {
		// We first match on HTTP 1.1 methods.
		httpL = tcpm.Match(cmux.HTTP1Fast())

		// If not matched, we assume that its TLS.
		tlsl := tcpm.Match(cmux.Any())
		tlsConfig := tls.Config{
			Certificates: []tls.Certificate{*a.settings.Certificate},
		}
		if a.TLSConfigCustomizer != nil {
			a.TLSConfigCustomizer(&tlsConfig)
		}
		tlsl = tls.NewListener(tlsl, &tlsConfig)

		// Now, we build another mux recursively to match HTTPS and gRPC.
		tlsm = cmux.New(tlsl)
		httpsL = tlsm.Match(cmux.HTTP1Fast())
		grpcL = tlsm.Match(cmux.Any())
	}

	// Start the muxed listeners for our servers
	log.Infof("argocd %s serving on port %d (url: %s, tls: %v, namespace: %s, sso: %v)",
		common.GetVersion(), port, a.settings.URL, a.useTLS(), a.Namespace, a.settings.IsSSOConfigured())

	go a.projInformer.Run(ctx.Done())
	go a.appInformer.Run(ctx.Done())
	go func() { a.checkServeErr("grpcS", grpcS.Serve(grpcL)) }()
	go func() { a.checkServeErr("httpS", httpS.Serve(httpL)) }()
	if a.useTLS() {
		go func() { a.checkServeErr("httpsS", httpsS.Serve(httpsL)) }()
		go func() { a.checkServeErr("tlsm", tlsm.Serve()) }()
	}
	go a.watchSettings()
	go a.rbacPolicyLoader(ctx)
	go func() { a.checkServeErr("tcpm", tcpm.Serve()) }()
	go func() { a.checkServeErr("metrics", metricsServ.ListenAndServe()) }()
	if !cache.WaitForCacheSync(ctx.Done(), a.projInformer.HasSynced, a.appInformer.HasSynced) {
		log.Fatal("Timed out waiting for project cache to sync")
	}

	a.stopCh = make(chan struct{})
	<-a.stopCh
	errors.CheckError(conn.Close())
}

// checkServeErr checks the error from a .Serve() call to decide if it was a graceful shutdown
func (a *ArgoCDServer) checkServeErr(name string, err error) {
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

// Shutdown stops the Argo CD server
func (a *ArgoCDServer) Shutdown() {
	log.Info("Shut down requested")
	stopCh := a.stopCh
	a.stopCh = nil
	if stopCh != nil {
		close(stopCh)
	}
}

// watchSettings watches the configmap and secret for any setting updates that would warrant a
// restart of the API server.
func (a *ArgoCDServer) watchSettings() {
	updateCh := make(chan *settings_util.ArgoCDSettings, 1)
	a.settingsMgr.Subscribe(updateCh)

	prevURL := a.settings.URL
	prevOIDCConfig := a.settings.OIDCConfigRAW
	prevDexCfgBytes, err := dex.GenerateDexConfigYAML(a.settings)
	errors.CheckError(err)
	prevGitHubSecret := a.settings.WebhookGitHubSecret
	prevGitLabSecret := a.settings.WebhookGitLabSecret
	prevBitbucketUUID := a.settings.WebhookBitbucketUUID
	prevBitbucketServerSecret := a.settings.WebhookBitbucketServerSecret
	prevGogsSecret := a.settings.WebhookGogsSecret
	var prevCert, prevCertKey string
	if a.settings.Certificate != nil && !a.ArgoCDServerOpts.Insecure {
		prevCert, prevCertKey = tlsutil.EncodeX509KeyPairString(*a.settings.Certificate)
	}

	for {
		newSettings := <-updateCh
		a.settings = newSettings
		newDexCfgBytes, err := dex.GenerateDexConfigYAML(a.settings)
		errors.CheckError(err)
		if string(newDexCfgBytes) != string(prevDexCfgBytes) {
			log.Infof("dex config modified. restarting")
			break
		}
		if prevOIDCConfig != a.settings.OIDCConfigRAW {
			log.Infof("odic config modified. restarting")
			break
		}
		if prevURL != a.settings.URL {
			log.Infof("url modified. restarting")
			break
		}
		if prevGitHubSecret != a.settings.WebhookGitHubSecret {
			log.Infof("github secret modified. restarting")
			break
		}
		if prevGitLabSecret != a.settings.WebhookGitLabSecret {
			log.Infof("gitlab secret modified. restarting")
			break
		}
		if prevBitbucketUUID != a.settings.WebhookBitbucketUUID {
			log.Infof("bitbucket uuid modified. restarting")
			break
		}
		if prevBitbucketServerSecret != a.settings.WebhookBitbucketServerSecret {
			log.Infof("bitbucket server secret modified. restarting")
			break
		}
		if prevGogsSecret != a.settings.WebhookGogsSecret {
			log.Infof("gogs secret modified. restarting")
			break
		}
		if !a.ArgoCDServerOpts.Insecure {
			var newCert, newCertKey string
			if a.settings.Certificate != nil {
				newCert, newCertKey = tlsutil.EncodeX509KeyPairString(*a.settings.Certificate)
			}
			if newCert != prevCert || newCertKey != prevCertKey {
				log.Infof("tls certificate modified. restarting")
				break
			}
		}
	}
	log.Info("shutting down settings watch")
	a.Shutdown()
	a.settingsMgr.Unsubscribe(updateCh)
	close(updateCh)
}

func (a *ArgoCDServer) rbacPolicyLoader(ctx context.Context) {
	err := a.enf.RunPolicyLoader(ctx, func(cm *v1.ConfigMap) error {
		var scopes []string
		if scopesStr, ok := cm.Data[rbac.ConfigMapScopesKey]; len(scopesStr) > 0 && ok {
			scopes = make([]string, 0)
			err := yaml.Unmarshal([]byte(scopesStr), &scopes)
			if err != nil {
				return err
			}
		}

		a.policyEnforcer.SetScopes(scopes)
		return nil
	})
	errors.CheckError(err)
}

func (a *ArgoCDServer) useTLS() bool {
	if a.Insecure || a.settings.Certificate == nil {
		return false
	}
	return true
}

func (a *ArgoCDServer) newGRPCServer() *grpc.Server {
	if enableGRPCTimeHistogram {
		grpc_prometheus.EnableHandlingTimeHistogram()
	}

	sOpts := []grpc.ServerOption{
		// Set the both send and receive the bytes limit to be 100MB
		// The proper way to achieve high performance is to have pagination
		// while we work toward that, we can have high limit first
		grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.MaxSendMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.ConnectionTimeout(300 * time.Second),
	}
	sensitiveMethods := map[string]bool{
		"/cluster.ClusterService/Create":                          true,
		"/cluster.ClusterService/Update":                          true,
		"/session.SessionService/Create":                          true,
		"/account.AccountService/UpdatePassword":                  true,
		"/gpgkey.GPGKeyService/CreateGnuPGPublicKey":              true,
		"/repository.RepositoryService/Create":                    true,
		"/repository.RepositoryService/Update":                    true,
		"/repository.RepositoryService/CreateRepository":          true,
		"/repository.RepositoryService/UpdateRepository":          true,
		"/repository.RepositoryService/ValidateAccess":            true,
		"/repocreds.RepoCredsService/CreateRepositoryCredentials": true,
		"/repocreds.RepoCredsService/UpdateRepositoryCredentials": true,
		"/application.ApplicationService/PatchResource":           true,
	}
	// NOTE: notice we do not configure the gRPC server here with TLS (e.g. grpc.Creds(creds))
	// This is because TLS handshaking occurs in cmux handling
	sOpts = append(sOpts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		grpc_logrus.StreamServerInterceptor(a.log),
		grpc_prometheus.StreamServerInterceptor,
		grpc_auth.StreamServerInterceptor(a.Authenticate),
		grpc_util.UserAgentStreamServerInterceptor(common.ArgoCDUserAgentName, clientConstraint),
		grpc_util.PayloadStreamServerInterceptor(a.log, true, func(ctx netCtx.Context, fullMethodName string, servingObject interface{}) bool {
			return !sensitiveMethods[fullMethodName]
		}),
		grpc_util.ErrorCodeStreamServerInterceptor(),
		grpc_util.PanicLoggerStreamServerInterceptor(a.log),
	)))
	sOpts = append(sOpts, grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
		bug21955WorkaroundInterceptor,
		grpc_logrus.UnaryServerInterceptor(a.log),
		grpc_prometheus.UnaryServerInterceptor,
		grpc_auth.UnaryServerInterceptor(a.Authenticate),
		grpc_util.UserAgentUnaryServerInterceptor(common.ArgoCDUserAgentName, clientConstraint),
		grpc_util.PayloadUnaryServerInterceptor(a.log, true, func(ctx netCtx.Context, fullMethodName string, servingObject interface{}) bool {
			return !sensitiveMethods[fullMethodName]
		}),
		grpc_util.ErrorCodeUnaryServerInterceptor(),
		grpc_util.PanicLoggerUnaryServerInterceptor(a.log),
	)))
	grpcS := grpc.NewServer(sOpts...)
	db := db.NewDB(a.Namespace, a.settingsMgr, a.KubeClientset)
	kubectl := &kube.KubectlCmd{}
	clusterService := cluster.NewServer(db, a.enf, a.Cache, kubectl)
	repoService := repository.NewServer(a.RepoClientset, db, a.enf, a.Cache, a.settingsMgr)
	repoCredsService := repocreds.NewServer(a.RepoClientset, db, a.enf, a.settingsMgr)
	var loginRateLimiter func() (io.Closer, error)
	if maxConcurrentLoginRequestsCount > 0 {
		loginRateLimiter = session.NewLoginRateLimiter(maxConcurrentLoginRequestsCount)
	}
	sessionService := session.NewServer(a.sessionMgr, a, a.policyEnforcer, loginRateLimiter)
	projectLock := sync.NewKeyLock()
	applicationService := application.NewServer(
		a.Namespace,
		a.KubeClientset,
		a.AppClientset,
		a.appLister,
		a.appInformer,
		a.RepoClientset,
		a.Cache,
		kubectl,
		db,
		a.enf,
		projectLock,
		a.settingsMgr,
		a.projInformer)
	projectService := project.NewServer(a.Namespace, a.KubeClientset, a.AppClientset, a.enf, projectLock, a.sessionMgr, a.policyEnforcer)
	settingsService := settings.NewServer(a.settingsMgr, a, a.DisableAuth)
	accountService := account.NewServer(a.sessionMgr, a.settingsMgr, a.enf)
	certificateService := certificate.NewServer(a.RepoClientset, db, a.enf)
	gpgkeyService := gpgkey.NewServer(a.RepoClientset, db, a.enf)
	versionpkg.RegisterVersionServiceServer(grpcS, &version.Server{})
	clusterpkg.RegisterClusterServiceServer(grpcS, clusterService)
	applicationpkg.RegisterApplicationServiceServer(grpcS, applicationService)
	repositorypkg.RegisterRepositoryServiceServer(grpcS, repoService)
	repocredspkg.RegisterRepoCredsServiceServer(grpcS, repoCredsService)
	sessionpkg.RegisterSessionServiceServer(grpcS, sessionService)
	settingspkg.RegisterSettingsServiceServer(grpcS, settingsService)
	projectpkg.RegisterProjectServiceServer(grpcS, projectService)
	accountpkg.RegisterAccountServiceServer(grpcS, accountService)
	certificatepkg.RegisterCertificateServiceServer(grpcS, certificateService)
	gpgkeypkg.RegisterGPGKeyServiceServer(grpcS, gpgkeyService)
	// Register reflection service on gRPC server.
	reflection.Register(grpcS)
	grpc_prometheus.Register(grpcS)
	errors.CheckError(projectService.NormalizeProjs())
	return grpcS
}

// TranslateGrpcCookieHeader conditionally sets a cookie on the response.
func (a *ArgoCDServer) translateGrpcCookieHeader(ctx context.Context, w http.ResponseWriter, resp golang_proto.Message) error {
	if sessionResp, ok := resp.(*sessionpkg.SessionResponse); ok {
		cookiePath := fmt.Sprintf("path=/%s", strings.TrimRight(strings.TrimLeft(a.ArgoCDServerOpts.RootPath, "/"), "/"))
		flags := []string{cookiePath, "SameSite=lax", "httpOnly"}
		if !a.Insecure {
			flags = append(flags, "Secure")
		}
		token := sessionResp.Token
		if token != "" {
			var err error
			token, err = zjwt.ZJWT(token)
			if err != nil {
				return err
			}
		}
		cookie, err := httputil.MakeCookieMetadata(common.AuthCookieName, token, flags...)
		if err != nil {
			return err
		}
		w.Header().Set("Set-Cookie", cookie)
	}
	return nil
}

func withRootPath(handler http.Handler, a *ArgoCDServer) http.Handler {
	// get rid of slashes
	root := strings.TrimRight(strings.TrimLeft(a.RootPath, "/"), "/")

	mux := http.NewServeMux()
	mux.Handle("/"+root+"/", http.StripPrefix("/"+root, handler))

	healthz.ServeHealthCheck(mux, func() error {
		return nil
	})

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
func (a *ArgoCDServer) newHTTPServer(ctx context.Context, port int, grpcWebHandler http.Handler) *http.Server {
	endpoint := fmt.Sprintf("localhost:%d", port)
	mux := http.NewServeMux()
	httpS := http.Server{
		Addr: endpoint,
		Handler: &handlerSwitcher{
			handler: mux,
			urlToHandler: map[string]http.Handler{
				"/api/badge": badge.NewHandler(a.AppClientset, a.settingsMgr, a.Namespace),
			},
			contentTypeToHandler: map[string]http.Handler{
				"application/grpc-web+proto": grpcWebHandler,
			},
		},
	}
	var dOpts []grpc.DialOption
	dOpts = append(dOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(apiclient.MaxGRPCMessageSize)))
	dOpts = append(dOpts, grpc.WithUserAgent(fmt.Sprintf("%s/%s", common.ArgoCDUserAgentName, common.GetVersion().Version)))
	if a.useTLS() {
		// The following sets up the dial Options for grpc-gateway to talk to gRPC server over TLS.
		// grpc-gateway is just translating HTTP/HTTPS requests as gRPC requests over localhost,
		// so we need to supply the same certificates to establish the connections that a normal,
		// external gRPC client would need.
		tlsConfig := a.settings.TLSConfig()
		if a.TLSConfigCustomizer != nil {
			a.TLSConfigCustomizer(tlsConfig)
		}
		tlsConfig.InsecureSkipVerify = true
		dCreds := credentials.NewTLS(tlsConfig)
		dOpts = append(dOpts, grpc.WithTransportCredentials(dCreds))
	} else {
		dOpts = append(dOpts, grpc.WithInsecure())
	}

	// HTTP 1.1+JSON Server
	// grpc-ecosystem/grpc-gateway is used to proxy HTTP requests to the corresponding gRPC call
	// NOTE: if a marshaller option is not supplied, grpc-gateway will default to the jsonpb from
	// golang/protobuf. Which does not support types such as time.Time. gogo/protobuf does support
	// time.Time, but does not support custom UnmarshalJSON() and MarshalJSON() methods. Therefore
	// we use our own Marshaler
	gwMuxOpts := runtime.WithMarshalerOption(runtime.MIMEWildcard, new(grpc_util.JSONMarshaler))
	gwCookieOpts := runtime.WithForwardResponseOption(a.translateGrpcCookieHeader)
	gwmux := runtime.NewServeMux(gwMuxOpts, gwCookieOpts)

	var handler http.Handler = gwmux
	if a.EnableGZip {
		handler = compressHandler(handler)
	}
	mux.Handle("/api/", handler)

	mustRegisterGWHandler(versionpkg.RegisterVersionServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(clusterpkg.RegisterClusterServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(applicationpkg.RegisterApplicationServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(repositorypkg.RegisterRepositoryServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(repocredspkg.RegisterRepoCredsServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(sessionpkg.RegisterSessionServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(settingspkg.RegisterSettingsServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(projectpkg.RegisterProjectServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(accountpkg.RegisterAccountServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(certificatepkg.RegisterCertificateServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(gpgkeypkg.RegisterGPGKeyServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)

	// Swagger UI
	swagger.ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui", a.RootPath)
	healthz.ServeHealthCheck(mux, func() error {
		return nil
	})

	// Dex reverse proxy and client app and OAuth2 login/callback
	a.registerDexHandlers(mux)

	// Webhook handler for git events
	acdWebhookHandler := webhook.NewHandler(a.Namespace, a.AppClientset, a.settings)
	mux.HandleFunc("/api/webhook", acdWebhookHandler.Handler)

	// Serve cli binaries directly from API server
	registerDownloadHandlers(mux, "/download")

	// Serve UI static assets
	if a.StaticAssetsDir != "" {
		mux.HandleFunc("/", a.newStaticAssetsHandler(a.StaticAssetsDir, a.BaseHRef))
	}
	return &httpS
}

// registerDexHandlers will register dex HTTP handlers, creating the the OAuth client app
func (a *ArgoCDServer) registerDexHandlers(mux *http.ServeMux) {
	if !a.settings.IsSSOConfigured() {
		return
	}
	// Run dex OpenID Connect Identity Provider behind a reverse proxy (served at /api/dex)
	var err error
	mux.HandleFunc(common.DexAPIEndpoint+"/", dexutil.NewDexHTTPReverseProxy(a.DexServerAddr, a.BaseHRef))
	if a.useTLS() {
		tlsConfig := a.settings.TLSConfig()
		tlsConfig.InsecureSkipVerify = true
	}
	a.ssoClientApp, err = oidc.NewClientApp(a.settings, a.Cache, a.DexServerAddr, a.BaseHRef)
	errors.CheckError(err)
	mux.HandleFunc(common.LoginEndpoint, a.ssoClientApp.HandleLogin)
	mux.HandleFunc(common.CallbackEndpoint, a.ssoClientApp.HandleCallback)
}

// newRedirectServer returns an HTTP server which does a 307 redirect to the HTTPS server
func newRedirectServer(port int, rootPath string) *http.Server {
	addr := fmt.Sprintf("localhost:%d/%s", port, strings.TrimRight(strings.TrimLeft(rootPath, "/"), "/"))
	return &http.Server{
		Addr: addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			target := "https://" + req.Host
			if rootPath != "" {
				target += "/" + strings.TrimRight(strings.TrimLeft(rootPath, "/"), "/")
			}
			target += req.URL.Path
			if len(req.URL.RawQuery) > 0 {
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
		mux.HandleFunc(base+"/argocd-linux-amd64", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, linuxPath)
		})
	}
	darwinPath, err := exec.LookPath("argocd-darwin-amd64")
	if err != nil {
		log.Warnf("argocd-darwin-amd64 not in PATH")
	} else {
		mux.HandleFunc(base+"/argocd-darwin-amd64", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, darwinPath)
		})
	}
	windowsPath, err := exec.LookPath("argocd-windows-amd64.exe")
	if err != nil {
		log.Warnf("argocd-windows-amd64.exe not in PATH")
	} else {
		mux.HandleFunc(base+"/argocd-windows-amd64.exe", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, windowsPath)
		})
	}
}

func indexFilePath(srcPath string, baseHRef string) (string, error) {
	if baseHRef == "/" {
		return srcPath, nil
	}
	filePath := path.Join(os.TempDir(), fmt.Sprintf("index_%s.html", strings.Replace(strings.Trim(baseHRef, "/"), "/", "_", -1)))
	f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return filePath, nil
		}
		return "", err
	}
	defer io.Close(f)

	data, err := ioutil.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	if baseHRef != "/" {
		data = []byte(baseHRefRegex.ReplaceAllString(string(data), fmt.Sprintf(`<base href="/%s/">`, strings.Trim(baseHRef, "/"))))
	}
	_, err = f.Write(data)
	if err != nil {
		return "", err
	}

	return filePath, nil
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// newStaticAssetsHandler returns an HTTP handler to serve UI static assets
func (server *ArgoCDServer) newStaticAssetsHandler(dir string, baseHRef string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		acceptHTML := false
		for _, acceptType := range strings.Split(r.Header.Get("Accept"), ",") {
			if acceptType == "text/html" || acceptType == "html" {
				acceptHTML = true
				break
			}
		}
		fileRequest := r.URL.Path != "/index.html" && fileExists(path.Join(dir, r.URL.Path))

		// Set X-Frame-Options according to configuration
		if server.XFrameOptions != "" {
			w.Header().Set("X-Frame-Options", server.XFrameOptions)
		}

		// serve index.html for non file requests to support HTML5 History API
		if acceptHTML && !fileRequest && (r.Method == "GET" || r.Method == "HEAD") {
			for k, v := range noCacheHeaders {
				w.Header().Set(k, v)
			}
			indexHtmlPath, err := indexFilePath(path.Join(dir, "index.html"), baseHRef)
			if err != nil {
				http.Error(w, fmt.Sprintf("Unable to access index.html: %v", err), http.StatusInternalServerError)
				return
			}
			http.ServeFile(w, r, indexHtmlPath)
		} else {
			http.ServeFile(w, r, path.Join(dir, r.URL.Path))
		}
	}
}

type registerFunc func(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) error

// mustRegisterGWHandler is a convenience function to register a gateway handler
func mustRegisterGWHandler(register registerFunc, ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) {
	err := register(ctx, mux, endpoint, opts)
	if err != nil {
		panic(err)
	}
}

// Authenticate checks for the presence of a valid token when accessing server-side resources.
func (a *ArgoCDServer) Authenticate(ctx context.Context) (context.Context, error) {
	if a.DisableAuth {
		return ctx, nil
	}
	claims, claimsErr := a.getClaims(ctx)
	if claims != nil {
		// Add claims to the context to inspect for RBAC
		// nolint:staticcheck
		ctx = context.WithValue(ctx, "claims", claims)
	}

	if claimsErr != nil {
		argoCDSettings, err := a.settingsMgr.GetSettings()
		if err != nil {
			return ctx, status.Errorf(codes.Internal, "unable to load settings: %v", err)
		}
		if !argoCDSettings.AnonymousUserEnabled {
			return ctx, claimsErr
		}
	}

	return ctx, nil
}

func (a *ArgoCDServer) getClaims(ctx context.Context) (jwt.Claims, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, ErrNoSession
	}
	tokenString := getToken(md)
	if tokenString == "" {
		return nil, ErrNoSession
	}
	claims, err := a.sessionMgr.VerifyToken(tokenString)
	if err != nil {
		return claims, status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
	}
	return claims, nil
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

	var tokens []string

	// looks for the HTTP header `Authorization: Bearer ...`
	for _, t := range md["authorization"] {
		if strings.HasPrefix(t, "Bearer ") {
			tokens = append(tokens, strings.TrimPrefix(t, "Bearer "))
		}
	}

	// check the HTTP cookie
	for _, t := range md["grpcgateway-cookie"] {
		header := http.Header{}
		header.Add("Cookie", t)
		request := http.Request{Header: header}
		token, err := request.Cookie(common.AuthCookieName)
		if err == nil {
			tokens = append(tokens, token.Value)
		}
	}

	for _, t := range tokens {
		value, err := zjwt.JWT(t)
		if err == nil {
			return value
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

func bug21955WorkaroundInterceptor(ctx context.Context, req interface{}, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
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
		server, err := url.QueryUnescape(cq.Server)
		if err != nil {
			return nil, err
		}
		cq.Server = server
	} else if cu, ok := req.(*clusterpkg.ClusterUpdateRequest); ok {
		server, err := url.QueryUnescape(cu.Cluster.Server)
		if err != nil {
			return nil, err
		}
		cu.Cluster.Server = server
	}
	return handler(ctx, req)
}
