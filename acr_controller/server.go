package acr_controller

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	appclient "github.com/argoproj/argo-cd/v2/acr_controller/application"
	acr_controller "github.com/argoproj/argo-cd/v2/acr_controller/controller"

	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	appinformer "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/util/healthz"
	settings_util "github.com/argoproj/argo-cd/v2/util/settings"
)

var backoff = wait.Backoff{
	Steps:    5,
	Duration: 500 * time.Millisecond,
	Factor:   1.0,
	Jitter:   0.1,
}

type ACRServer struct {
	ACRServerOpts

	settings             *settings_util.ArgoCDSettings
	log                  *log.Entry
	appInformer          cache.SharedIndexInformer
	appLister            applisters.ApplicationLister
	applicationClientset appclientset.Interface

	// stopCh is the channel which when closed, will shutdown the Event Reporter server
	stopCh     chan struct{}
	serviceSet *ACRServerSet
}

type ACRServerSet struct{}

type ACRServerOpts struct {
	ListenPort               int
	ListenHost               string
	Namespace                string
	KubeClientset            kubernetes.Interface
	AppClientset             appclientset.Interface
	ApplicationServiceClient appclient.ApplicationClient
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

type Listeners struct {
	Main net.Listener
}

func (l *Listeners) Close() error {
	if l.Main != nil {
		if err := l.Main.Close(); err != nil {
			return err
		}
		l.Main = nil
	}
	return nil
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

func (a *ACRServer) healthCheck(r *http.Request) error {
	return nil
}

// Init starts informers used by the API server
func (a *ACRServer) Init(ctx context.Context) {
	go a.appInformer.Run(ctx.Done())
	svcSet := newApplicationChangeRevisionServiceSet()
	a.serviceSet = svcSet
}

func (a *ACRServer) RunController(ctx context.Context) {
	controller := acr_controller.NewApplicationChangeRevisionController(a.appInformer, a.Cache, a.ApplicationServiceClient, a.appLister, a.applicationClientset)
	go controller.Run(ctx)
}

// newHTTPServer returns the HTTP server to serve HTTP/HTTPS requests. This is implemented
// using grpc-gateway as a proxy to the gRPC server.
func (a *ACRServer) newHTTPServer(ctx context.Context, port int) *http.Server {
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

func (a *ACRServer) checkServeErr(name string, err error) {
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

func (a *ACRServer) Listen() (*Listeners, error) {
	mainLn, err := startListener(a.ListenHost, a.ListenPort)
	if err != nil {
		return nil, err
	}
	return &Listeners{Main: mainLn}, nil
}

// Run runs the API Server
// We use k8s.io/code-generator/cmd/go-to-protobuf to generate the .proto files from the API types.
// k8s.io/ go-to-protobuf uses protoc-gen-gogo, which comes from gogo/protobuf (a fork of
// golang/protobuf).
func (a *ACRServer) Run(ctx context.Context, lns *Listeners) {
	httpS := a.newHTTPServer(ctx, a.ListenPort)
	tlsConfig := tls.Config{}
	tlsConfig.GetCertificate = func(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
		return a.settings.Certificate, nil
	}
	go func() { a.checkServeErr("httpS", httpS.Serve(lns.Main)) }()
	go a.RunController(ctx)

	if !cache.WaitForCacheSync(ctx.Done(), a.appInformer.HasSynced) {
		log.Fatal("Timed out waiting for project cache to sync")
	}

	a.stopCh = make(chan struct{})
	<-a.stopCh
}

// NewServer returns a new instance of the Event Reporter server
func NewApplicationChangeRevisionServer(ctx context.Context, opts ACRServerOpts) *ACRServer {
	appInformerNs := opts.Namespace
	if len(opts.ApplicationNamespaces) > 0 {
		appInformerNs = ""
	}
	appFactory := appinformer.NewSharedInformerFactoryWithOptions(opts.AppClientset, 0, appinformer.WithNamespace(appInformerNs), appinformer.WithTweakListOptions(func(options *metav1.ListOptions) {}))

	appInformer := appFactory.Argoproj().V1alpha1().Applications().Informer()
	appLister := appFactory.Argoproj().V1alpha1().Applications().Lister()

	server := &ACRServer{
		ACRServerOpts:        opts,
		log:                  log.NewEntry(log.StandardLogger()),
		appInformer:          appInformer,
		appLister:            appLister,
		applicationClientset: opts.AppClientset,
	}

	return server
}

func newApplicationChangeRevisionServiceSet() *ACRServerSet {
	return &ACRServerSet{}
}
