package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"

	golang_proto "github.com/golang/protobuf/proto"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_logrus "github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/improbable-eng/grpc-web/go/grpcweb"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"github.com/soheilhy/cmux"
	netCtx "golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	argocd "github.com/argoproj/argo-cd"
	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/errors"
	"github.com/argoproj/argo-cd/pkg/apiclient"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformer "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/server/account"
	"github.com/argoproj/argo-cd/server/application"
	"github.com/argoproj/argo-cd/server/cluster"
	"github.com/argoproj/argo-cd/server/project"
	"github.com/argoproj/argo-cd/server/rbacpolicy"
	"github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/server/session"
	"github.com/argoproj/argo-cd/server/settings"
	"github.com/argoproj/argo-cd/server/version"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/assets"
	argocache "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/dex"
	dexutil "github.com/argoproj/argo-cd/util/dex"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/healthz"
	httputil "github.com/argoproj/argo-cd/util/http"
	jsonutil "github.com/argoproj/argo-cd/util/json"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/oidc"
	"github.com/argoproj/argo-cd/util/rbac"
	util_session "github.com/argoproj/argo-cd/util/session"
	settings_util "github.com/argoproj/argo-cd/util/settings"
	"github.com/argoproj/argo-cd/util/swagger"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
	"github.com/argoproj/argo-cd/util/webhook"
)

var (
	// ErrNoSession indicates no auth token was supplied as part of a request
	ErrNoSession = status.Errorf(codes.Unauthenticated, "no session information")
)

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
)

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

	// stopCh is the channel which when closed, will shutdown the Argo CD server
	stopCh chan struct{}
}

type ArgoCDServerOpts struct {
	DisableAuth         bool
	Insecure            bool
	Namespace           string
	DexServerAddr       string
	StaticAssetsDir     string
	BaseHRef            string
	KubeClientset       kubernetes.Interface
	AppClientset        appclientset.Interface
	RepoClientset       reposerver.Clientset
	Cache               *argocache.Cache
	TLSConfigCustomizer tlsutil.ConfigCustomizer
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

	_, err := opts.AppClientset.ArgoprojV1alpha1().AppProjects(opts.Namespace).Create(defaultProj)
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

// NewServer returns a new instance of the Argo CD API server
func NewServer(ctx context.Context, opts ArgoCDServerOpts) *ArgoCDServer {
	settingsMgr := settings_util.NewSettingsManager(ctx, opts.KubeClientset, opts.Namespace)
	settings, err := settingsMgr.InitializeSettings()
	errors.CheckError(err)
	err = initializeDefaultProject(opts)
	errors.CheckError(err)
	sessionMgr := util_session.NewSessionManager(settingsMgr, opts.DexServerAddr)

	factory := appinformer.NewFilteredSharedInformerFactory(opts.AppClientset, 0, opts.Namespace, func(options *metav1.ListOptions) {})
	projInformer := factory.Argoproj().V1alpha1().AppProjects().Informer()
	projLister := factory.Argoproj().V1alpha1().AppProjects().Lister().AppProjects(opts.Namespace)

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
		policyEnforcer:   policyEnf,
	}
}

// Run runs the API Server
// We use k8s.io/code-generator/cmd/go-to-protobuf to generate the .proto files from the API types.
// k8s.io/ go-to-protobuf uses protoc-gen-gogo, which comes from gogo/protobuf (a fork of
// golang/protobuf).
func (a *ArgoCDServer) Run(ctx context.Context, port int) {
	grpcS := a.newGRPCServer()
	grpcWebS := grpcweb.WrapServer(grpcS)
	var httpS *http.Server
	var httpsS *http.Server
	if a.useTLS() {
		httpS = newRedirectServer(port)
		httpsS = a.newHTTPServer(ctx, port, grpcWebS)
	} else {
		httpS = a.newHTTPServer(ctx, port, grpcWebS)
	}
	metricsServ := newAPIServerMetricsServer()

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
		argocd.GetVersion(), port, a.settings.URL, a.useTLS(), a.Namespace, a.settings.IsSSOConfigured())

	go a.projInformer.Run(ctx.Done())
	go func() { a.checkServeErr("grpcS", grpcS.Serve(grpcL)) }()
	go func() { a.checkServeErr("httpS", httpS.Serve(httpL)) }()
	if a.useTLS() {
		go func() { a.checkServeErr("httpsS", httpsS.Serve(httpsL)) }()
		go func() { a.checkServeErr("tlsm", tlsm.Serve()) }()
	}
	go a.watchSettings(ctx)
	go a.rbacPolicyLoader(ctx)
	go func() { a.checkServeErr("tcpm", tcpm.Serve()) }()
	go func() { a.checkServeErr("metrics", metricsServ.ListenAndServe()) }()
	if !cache.WaitForCacheSync(ctx.Done(), a.projInformer.HasSynced) {
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
func (a *ArgoCDServer) watchSettings(ctx context.Context) {
	updateCh := make(chan *settings_util.ArgoCDSettings, 1)
	a.settingsMgr.Subscribe(updateCh)

	prevURL := a.settings.URL
	prevOIDCConfig := a.settings.OIDCConfigRAW
	prevDexCfgBytes, err := dex.GenerateDexConfigYAML(a.settings)
	errors.CheckError(err)
	prevGitHubSecret := a.settings.WebhookGitHubSecret
	prevGitLabSecret := a.settings.WebhookGitLabSecret
	prevBitBucketUUID := a.settings.WebhookBitbucketUUID
	var prevCert, prevCertKey string
	if a.settings.Certificate != nil {
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
		if prevBitBucketUUID != a.settings.WebhookBitbucketUUID {
			log.Infof("bitbucket uuid modified. restarting")
			break
		}
		var newCert, newCertKey string
		if a.settings.Certificate != nil {
			newCert, newCertKey = tlsutil.EncodeX509KeyPairString(*a.settings.Certificate)
		}
		if newCert != prevCert || newCertKey != prevCertKey {
			log.Infof("tls certificate modified. restarting")
			break
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
	sOpts := []grpc.ServerOption{
		// Set the both send and receive the bytes limit to be 100MB
		// The proper way to achieve high performance is to have pagination
		// while we work toward that, we can have high limit first
		grpc.MaxRecvMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.MaxSendMsgSize(apiclient.MaxGRPCMessageSize),
		grpc.ConnectionTimeout(300 * time.Second),
	}
	sensitiveMethods := map[string]bool{
		"/cluster.ClusterService/Create":                true,
		"/cluster.ClusterService/Update":                true,
		"/session.SessionService/Create":                true,
		"/account.AccountService/UpdatePassword":        true,
		"/repository.RepositoryService/Create":          true,
		"/repository.RepositoryService/Update":          true,
		"/application.ApplicationService/PatchResource": true,
	}
	// NOTE: notice we do not configure the gRPC server here with TLS (e.g. grpc.Creds(creds))
	// This is because TLS handshaking occurs in cmux handling
	sOpts = append(sOpts, grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
		grpc_logrus.StreamServerInterceptor(a.log),
		grpc_prometheus.StreamServerInterceptor,
		grpc_auth.StreamServerInterceptor(a.authenticate),
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
		grpc_auth.UnaryServerInterceptor(a.authenticate),
		grpc_util.UserAgentUnaryServerInterceptor(common.ArgoCDUserAgentName, clientConstraint),
		grpc_util.PayloadUnaryServerInterceptor(a.log, true, func(ctx netCtx.Context, fullMethodName string, servingObject interface{}) bool {
			return !sensitiveMethods[fullMethodName]
		}),
		grpc_util.ErrorCodeUnaryServerInterceptor(),
		grpc_util.PanicLoggerUnaryServerInterceptor(a.log),
	)))
	grpcS := grpc.NewServer(sOpts...)
	db := db.NewDB(a.Namespace, a.settingsMgr, a.KubeClientset)
	clusterService := cluster.NewServer(db, a.enf, a.Cache)
	repoService := repository.NewServer(a.RepoClientset, db, a.enf, a.Cache)
	sessionService := session.NewServer(a.sessionMgr)
	projectLock := util.NewKeyLock()
	applicationService := application.NewServer(a.Namespace, a.KubeClientset, a.AppClientset, a.RepoClientset, a.Cache, kube.KubectlCmd{}, db, a.enf, projectLock, a.settingsMgr)
	projectService := project.NewServer(a.Namespace, a.KubeClientset, a.AppClientset, a.enf, projectLock, a.sessionMgr)
	settingsService := settings.NewServer(a.settingsMgr)
	accountService := account.NewServer(a.sessionMgr, a.settingsMgr)
	version.RegisterVersionServiceServer(grpcS, &version.Server{})
	cluster.RegisterClusterServiceServer(grpcS, clusterService)
	application.RegisterApplicationServiceServer(grpcS, applicationService)
	repository.RegisterRepositoryServiceServer(grpcS, repoService)
	session.RegisterSessionServiceServer(grpcS, sessionService)
	settings.RegisterSettingsServiceServer(grpcS, settingsService)
	project.RegisterProjectServiceServer(grpcS, projectService)
	account.RegisterAccountServiceServer(grpcS, accountService)
	// Register reflection service on gRPC server.
	reflection.Register(grpcS)
	grpc_prometheus.Register(grpcS)
	return grpcS
}

// TranslateGrpcCookieHeader conditionally sets a cookie on the response.
func (a *ArgoCDServer) translateGrpcCookieHeader(ctx context.Context, w http.ResponseWriter, resp golang_proto.Message) error {
	if sessionResp, ok := resp.(*session.SessionResponse); ok {
		flags := []string{"path=/"}
		if !a.Insecure {
			flags = append(flags, "Secure")
		}
		cookie, err := httputil.MakeCookieMetadata(common.AuthCookieName, sessionResp.Token, flags...)
		if err != nil {
			return err
		}

		w.Header().Set("Set-Cookie", cookie)
	}
	return nil
}

// newHTTPServer returns the HTTP server to serve HTTP/HTTPS requests. This is implemented
// using grpc-gateway as a proxy to the gRPC server.
func (a *ArgoCDServer) newHTTPServer(ctx context.Context, port int, grpcWebHandler http.Handler) *http.Server {
	endpoint := fmt.Sprintf("localhost:%d", port)
	mux := http.NewServeMux()
	httpS := http.Server{
		Addr: endpoint,
		Handler: &handlerSwitcher{
			handler: &bug21955Workaround{handler: mux},
			contentTypeToHandler: map[string]http.Handler{
				"application/grpc-web+proto": grpcWebHandler,
			},
		},
	}
	var dOpts []grpc.DialOption
	dOpts = append(dOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(apiclient.MaxGRPCMessageSize)))
	dOpts = append(dOpts, grpc.WithUserAgent(fmt.Sprintf("%s/%s", common.ArgoCDUserAgentName, argocd.GetVersion().Version)))
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
	gwMuxOpts := runtime.WithMarshalerOption(runtime.MIMEWildcard, new(jsonutil.JSONMarshaler))
	gwCookieOpts := runtime.WithForwardResponseOption(a.translateGrpcCookieHeader)
	gwmux := runtime.NewServeMux(gwMuxOpts, gwCookieOpts)
	mux.Handle("/api/", gwmux)
	mustRegisterGWHandler(version.RegisterVersionServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(cluster.RegisterClusterServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(application.RegisterApplicationServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(repository.RegisterRepositoryServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(session.RegisterSessionServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(settings.RegisterSettingsServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)
	mustRegisterGWHandler(project.RegisterProjectServiceHandlerFromEndpoint, ctx, gwmux, endpoint, dOpts)

	// Swagger UI
	swagger.ServeSwaggerUI(mux, assets.SwaggerJSON, "/swagger-ui")
	healthz.ServeHealthCheck(mux, func() error {
		_, err := a.KubeClientset.(*kubernetes.Clientset).ServerVersion()
		return err
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
		mux.HandleFunc("/", newStaticAssetsHandler(a.StaticAssetsDir, a.BaseHRef))
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
	mux.HandleFunc(common.DexAPIEndpoint+"/", dexutil.NewDexHTTPReverseProxy(a.DexServerAddr))
	tlsConfig := a.settings.TLSConfig()
	tlsConfig.InsecureSkipVerify = true
	a.ssoClientApp, err = oidc.NewClientApp(a.settings, a.Cache, a.DexServerAddr)
	errors.CheckError(err)
	mux.HandleFunc(common.LoginEndpoint, a.ssoClientApp.HandleLogin)
	mux.HandleFunc(common.CallbackEndpoint, a.ssoClientApp.HandleCallback)
}

// newRedirectServer returns an HTTP server which does a 307 redirect to the HTTPS server
func newRedirectServer(port int) *http.Server {
	return &http.Server{
		Addr: fmt.Sprintf("localhost:%d", port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			target := "https://" + req.Host + req.URL.Path
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
	defer util.Close(f)

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

// newAPIServerMetricsServer returns HTTP server which serves prometheus metrics on gRPC requests
func newAPIServerMetricsServer() *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	return &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", common.PortArgoCDAPIServerMetrics),
		Handler: mux,
	}
}

// newStaticAssetsHandler returns an HTTP handler to serve UI static assets
func newStaticAssetsHandler(dir string, baseHRef string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		acceptHTML := false
		for _, acceptType := range strings.Split(r.Header.Get("Accept"), ",") {
			if acceptType == "text/html" || acceptType == "html" {
				acceptHTML = true
				break
			}
		}
		fileRequest := r.URL.Path != "/index.html" && strings.Contains(r.URL.Path, ".")

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
func (a *ArgoCDServer) authenticate(ctx context.Context) (context.Context, error) {
	if a.DisableAuth {
		return ctx, nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx, ErrNoSession
	}
	tokenString := getToken(md)
	if tokenString == "" {
		return ctx, ErrNoSession
	}
	claims, err := a.sessionMgr.VerifyToken(tokenString)
	if err != nil {
		return ctx, status.Errorf(codes.Unauthenticated, "invalid session: %v", err)
	}
	// Add claims to the context to inspect for RBAC
	ctx = context.WithValue(ctx, "claims", claims)
	return ctx, nil
}

// getToken extracts the token from gRPC metadata or cookie headers
func getToken(md metadata.MD) string {
	// check the "token" metadata
	tokens, ok := md[apiclient.MetaDataTokenKey]
	if ok && len(tokens) > 0 {
		return tokens[0]
	}
	// check the HTTP cookie
	for _, cookieToken := range md["grpcgateway-cookie"] {
		header := http.Header{}
		header.Add("Cookie", cookieToken)
		request := http.Request{Header: header}
		token, err := request.Cookie(common.AuthCookieName)
		if err == nil {
			return token.Value
		}
	}
	return ""
}

type handlerSwitcher struct {
	handler              http.Handler
	contentTypeToHandler map[string]http.Handler
}

func (s *handlerSwitcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if contentHandler, ok := s.contentTypeToHandler[r.Header.Get("content-type")]; ok {
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
	regexp.MustCompile(`/api/v1/repositories/[^/]+/apps`),
	regexp.MustCompile(`/api/v1/repositories/[^/]+/apps/[^/]+`),
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
	if rq, ok := req.(*repository.RepoQuery); ok {
		repo, err := url.QueryUnescape(rq.Repo)
		if err != nil {
			return nil, err
		}
		rq.Repo = repo
	} else if rk, ok := req.(*repository.RepoAppsQuery); ok {
		repo, err := url.QueryUnescape(rk.Repo)
		if err != nil {
			return nil, err
		}
		rk.Repo = repo
	} else if rdq, ok := req.(*repository.RepoAppDetailsQuery); ok {
		repo, err := url.QueryUnescape(rdq.Repo)
		if err != nil {
			return nil, err
		}
		path, err := url.QueryUnescape(rdq.Path)
		if err != nil {
			return nil, err
		}
		rdq.Repo = repo
		rdq.Path = path
	} else if ru, ok := req.(*repository.RepoUpdateRequest); ok {
		repo, err := url.QueryUnescape(ru.Repo.Repo)
		if err != nil {
			return nil, err
		}
		ru.Repo.Repo = repo
	} else if cq, ok := req.(*cluster.ClusterQuery); ok {
		server, err := url.QueryUnescape(cq.Server)
		if err != nil {
			return nil, err
		}
		cq.Server = server
	} else if cu, ok := req.(*cluster.ClusterUpdateRequest); ok {
		server, err := url.QueryUnescape(cu.Cluster.Server)
		if err != nil {
			return nil, err
		}
		cu.Cluster.Server = server
	}
	return handler(ctx, req)
}
