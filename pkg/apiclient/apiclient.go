package apiclient

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v4"
	"github.com/golang/protobuf/ptypes/empty"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	"github.com/hashicorp/go-retryablehttp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/common"
	accountpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/account"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	applicationsetpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/applicationset"
	certificatepkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/certificate"
	clusterpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/cluster"
	gpgkeypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/gpgkey"
	notificationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/notification"
	projectpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/project"
	repocredspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repocreds"
	repositorypkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/repository"
	sessionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/session"
	settingspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/settings"
	versionpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/env"
	grpc_util "github.com/argoproj/argo-cd/v2/util/grpc"
	http_util "github.com/argoproj/argo-cd/v2/util/http"
	argoio "github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/kube"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
	oidcutil "github.com/argoproj/argo-cd/v2/util/oidc"
	tls_util "github.com/argoproj/argo-cd/v2/util/tls"
)

const (
	MetaDataTokenKey = "token"
	// EnvArgoCDServer is the environment variable to look for an Argo CD server address
	EnvArgoCDServer = "ARGOCD_SERVER"
	// EnvArgoCDAuthToken is the environment variable to look for an Argo CD auth token
	EnvArgoCDAuthToken = "ARGOCD_AUTH_TOKEN"
)

// MaxGRPCMessageSize contains max grpc message size
var MaxGRPCMessageSize = env.ParseNumFromEnv(common.EnvGRPCMaxSizeMB, 200, 0, math.MaxInt32) * 1024 * 1024

// Client defines an interface for interaction with an Argo CD server.
type Client interface {
	ClientOptions() ClientOptions
	HTTPClient() (*http.Client, error)
	OIDCConfig(context.Context, *settingspkg.Settings) (*oauth2.Config, *oidc.Provider, error)
	NewRepoClient() (io.Closer, repositorypkg.RepositoryServiceClient, error)
	NewRepoClientOrDie() (io.Closer, repositorypkg.RepositoryServiceClient)
	NewRepoCredsClient() (io.Closer, repocredspkg.RepoCredsServiceClient, error)
	NewRepoCredsClientOrDie() (io.Closer, repocredspkg.RepoCredsServiceClient)
	NewCertClient() (io.Closer, certificatepkg.CertificateServiceClient, error)
	NewCertClientOrDie() (io.Closer, certificatepkg.CertificateServiceClient)
	NewClusterClient() (io.Closer, clusterpkg.ClusterServiceClient, error)
	NewClusterClientOrDie() (io.Closer, clusterpkg.ClusterServiceClient)
	NewGPGKeyClient() (io.Closer, gpgkeypkg.GPGKeyServiceClient, error)
	NewGPGKeyClientOrDie() (io.Closer, gpgkeypkg.GPGKeyServiceClient)
	NewApplicationClient() (io.Closer, applicationpkg.ApplicationServiceClient, error)
	NewApplicationSetClient() (io.Closer, applicationsetpkg.ApplicationSetServiceClient, error)
	NewApplicationClientOrDie() (io.Closer, applicationpkg.ApplicationServiceClient)
	NewApplicationSetClientOrDie() (io.Closer, applicationsetpkg.ApplicationSetServiceClient)
	NewNotificationClient() (io.Closer, notificationpkg.NotificationServiceClient, error)
	NewNotificationClientOrDie() (io.Closer, notificationpkg.NotificationServiceClient)
	NewSessionClient() (io.Closer, sessionpkg.SessionServiceClient, error)
	NewSessionClientOrDie() (io.Closer, sessionpkg.SessionServiceClient)
	NewSettingsClient() (io.Closer, settingspkg.SettingsServiceClient, error)
	NewSettingsClientOrDie() (io.Closer, settingspkg.SettingsServiceClient)
	NewVersionClient() (io.Closer, versionpkg.VersionServiceClient, error)
	NewVersionClientOrDie() (io.Closer, versionpkg.VersionServiceClient)
	NewProjectClient() (io.Closer, projectpkg.ProjectServiceClient, error)
	NewProjectClientOrDie() (io.Closer, projectpkg.ProjectServiceClient)
	NewAccountClient() (io.Closer, accountpkg.AccountServiceClient, error)
	NewAccountClientOrDie() (io.Closer, accountpkg.AccountServiceClient)
	WatchApplicationWithRetry(ctx context.Context, appName string, revision string) chan *v1alpha1.ApplicationWatchEvent
}

// ClientOptions hold address, security, and other settings for the API client.
type ClientOptions struct {
	ServerAddr           string
	PlainText            bool
	Insecure             bool
	CertFile             string
	ClientCertFile       string
	ClientCertKeyFile    string
	AuthToken            string
	ConfigPath           string
	Context              string
	UserAgent            string
	GRPCWeb              bool
	GRPCWebRootPath      string
	Core                 bool
	PortForward          bool
	PortForwardNamespace string
	Headers              []string
	HttpRetryMax         int
	KubeOverrides        *clientcmd.ConfigOverrides
	AppControllerName    string
	ServerName           string
	RedisHaProxyName     string
	RedisName            string
	RepoServerName       string
}

type client struct {
	ServerAddr      string
	PlainText       bool
	Insecure        bool
	CertPEMData     []byte
	ClientCert      *tls.Certificate
	AuthToken       string
	RefreshToken    string
	UserAgent       string
	GRPCWeb         bool
	GRPCWebRootPath string
	Headers         []string

	proxyMutex      *sync.Mutex
	proxyListener   net.Listener
	proxyServer     *grpc.Server
	proxyUsersCount int
	httpClient      *http.Client
}

// NewClient creates a new API client from a set of config options.
func NewClient(opts *ClientOptions) (Client, error) {
	var c client
	localCfg, err := localconfig.ReadLocalConfig(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	c.proxyMutex = &sync.Mutex{}
	var ctxName string
	if localCfg != nil {
		configCtx, err := localCfg.ResolveContext(opts.Context)
		if err != nil {
			return nil, err
		}
		if configCtx != nil {
			c.ServerAddr = configCtx.Server.Server
			if configCtx.Server.CACertificateAuthorityData != "" {
				c.CertPEMData, err = base64.StdEncoding.DecodeString(configCtx.Server.CACertificateAuthorityData)
				if err != nil {
					return nil, err
				}
			}
			if configCtx.Server.ClientCertificateData != "" && configCtx.Server.ClientCertificateKeyData != "" {
				clientCertData, err := base64.StdEncoding.DecodeString(configCtx.Server.ClientCertificateData)
				if err != nil {
					return nil, err
				}
				clientCertKeyData, err := base64.StdEncoding.DecodeString(configCtx.Server.ClientCertificateKeyData)
				if err != nil {
					return nil, err
				}
				clientCert, err := tls.X509KeyPair(clientCertData, clientCertKeyData)
				if err != nil {
					return nil, err
				}
				c.ClientCert = &clientCert
			} else if configCtx.Server.ClientCertificateData != "" || configCtx.Server.ClientCertificateKeyData != "" {
				return nil, errors.New("ClientCertificateData and ClientCertificateKeyData must always be specified together")
			}
			c.PlainText = configCtx.Server.PlainText
			c.Insecure = configCtx.Server.Insecure
			c.GRPCWeb = configCtx.Server.GRPCWeb
			c.GRPCWebRootPath = configCtx.Server.GRPCWebRootPath
			c.AuthToken = configCtx.User.AuthToken
			c.RefreshToken = configCtx.User.RefreshToken
			ctxName = configCtx.Name
		}
	}
	if opts.UserAgent != "" {
		c.UserAgent = opts.UserAgent
	} else {
		c.UserAgent = fmt.Sprintf("%s/%s", common.ArgoCDUserAgentName, common.GetVersion().Version)
	}
	// Override server address if specified in env or CLI flag
	c.ServerAddr = env.StringFromEnv(EnvArgoCDServer, c.ServerAddr)
	if opts.PortForward || opts.PortForwardNamespace != "" {
		if opts.KubeOverrides == nil {
			opts.KubeOverrides = &clientcmd.ConfigOverrides{}
		}
		serverPodLabelSelector := common.LabelKeyAppName + "=" + opts.ServerName
		port, err := kube.PortForward(8080, opts.PortForwardNamespace, opts.KubeOverrides, serverPodLabelSelector)
		if err != nil {
			return nil, err
		}
		opts.ServerAddr = fmt.Sprintf("127.0.0.1:%d", port)
		opts.Insecure = true
	}
	if opts.ServerAddr != "" {
		c.ServerAddr = opts.ServerAddr
	}
	// Make sure we got the server address and auth token from somewhere
	if c.ServerAddr == "" {
		return nil, errors.New("Argo CD server address unspecified")
	}
	// Override auth-token if specified in env variable or CLI flag
	c.AuthToken = env.StringFromEnv(EnvArgoCDAuthToken, c.AuthToken)
	if opts.AuthToken != "" {
		c.AuthToken = strings.TrimSpace(opts.AuthToken)
	}
	// Override certificate data if specified from CLI flag
	if opts.CertFile != "" {
		b, err := os.ReadFile(opts.CertFile)
		if err != nil {
			return nil, err
		}
		c.CertPEMData = b
	}
	// Override client certificate data if specified from CLI flag
	if opts.ClientCertFile != "" && opts.ClientCertKeyFile != "" {
		clientCert, err := tls.LoadX509KeyPair(opts.ClientCertFile, opts.ClientCertKeyFile)
		if err != nil {
			return nil, err
		}
		c.ClientCert = &clientCert
	} else if opts.ClientCertFile != "" || opts.ClientCertKeyFile != "" {
		return nil, errors.New("--client-crt and --client-crt-key must always be specified together")
	}
	// Override insecure/plaintext options if specified from CLI
	if opts.PlainText {
		c.PlainText = true
	}
	if opts.Insecure {
		c.Insecure = true
	}
	if opts.GRPCWeb {
		c.GRPCWeb = true
	}
	if opts.GRPCWebRootPath != "" {
		c.GRPCWebRootPath = opts.GRPCWebRootPath
	}

	if opts.HttpRetryMax > 0 {
		retryClient := retryablehttp.NewClient()
		retryClient.RetryMax = opts.HttpRetryMax
		c.httpClient = retryClient.StandardClient()
	} else {
		c.httpClient = &http.Client{}
	}

	if !c.PlainText {
		tlsConfig, err := c.tlsConfig()
		if err != nil {
			return nil, err
		}
		c.httpClient.Transport = &http.Transport{
			TLSClientConfig: tlsConfig,
		}
	}
	if !c.GRPCWeb {
		if parts := strings.Split(c.ServerAddr, ":"); len(parts) == 1 {
			// If port is unspecified, assume the most likely port
			c.ServerAddr += ":443"
		}
		// test if we need to set it to true
		// if a call to grpc failed, then try again with GRPCWeb
		conn, versionIf, err := c.NewVersionClient()
		if err == nil {
			defer argoio.Close(conn)
			_, err = versionIf.Version(context.Background(), &empty.Empty{})
		}
		if err != nil {
			c.GRPCWeb = true
			conn, versionIf := c.NewVersionClientOrDie()
			defer argoio.Close(conn)

			_, err := versionIf.Version(context.Background(), &empty.Empty{})
			if err == nil {
				log.Warnf("Failed to invoke grpc call. Use flag --grpc-web in grpc calls. To avoid this warning message, use flag --grpc-web.")
			} else {
				c.GRPCWeb = false
			}
		}
	}
	if localCfg != nil {
		err = c.refreshAuthToken(localCfg, ctxName, opts.ConfigPath)
		if err != nil {
			return nil, err
		}
	}
	c.Headers = opts.Headers

	return &c, nil
}

// OIDCConfig returns OAuth2 client config and a OpenID Provider based on Argo CD settings
// ctx can hold an appropriate http.Client to use for the exchange
func (c *client) OIDCConfig(ctx context.Context, set *settingspkg.Settings) (*oauth2.Config, *oidc.Provider, error) {
	var clientID string
	var issuerURL string
	var scopes []string
	if set.OIDCConfig != nil && set.OIDCConfig.Issuer != "" {
		if set.OIDCConfig.CLIClientID != "" {
			clientID = set.OIDCConfig.CLIClientID
		} else {
			clientID = set.OIDCConfig.ClientID
		}
		issuerURL = set.OIDCConfig.Issuer
		scopes = set.OIDCConfig.Scopes
	} else if set.DexConfig != nil && len(set.DexConfig.Connectors) > 0 {
		clientID = common.ArgoCDCLIClientAppID
		issuerURL = fmt.Sprintf("%s%s", set.URL, common.DexAPIEndpoint)
	} else {
		return nil, nil, fmt.Errorf("%s is not configured with SSO", c.ServerAddr)
	}
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to query provider %q: %w", issuerURL, err)
	}
	oidcConf, err := oidcutil.ParseConfig(provider)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse provider config: %w", err)
	}
	scopes = oidcutil.GetScopesOrDefault(scopes)
	if oidcutil.OfflineAccess(oidcConf.ScopesSupported) {
		scopes = append(scopes, oidc.ScopeOfflineAccess)
	}
	oauth2conf := oauth2.Config{
		ClientID: clientID,
		Scopes:   scopes,
		Endpoint: provider.Endpoint(),
	}
	return &oauth2conf, provider, nil
}

// HTTPClient returns a HTTPClient appropriate for performing OAuth, based on TLS settings
func (c *client) HTTPClient() (*http.Client, error) {
	tlsConfig, err := c.tlsConfig()
	if err != nil {
		return nil, err
	}

	headers, err := parseHeaders(c.Headers)
	if err != nil {
		return nil, err
	}

	if c.UserAgent != "" {
		headers.Set("User-Agent", c.UserAgent)
	}

	return &http.Client{
		Transport: &http_util.TransportWithHeader{
			RoundTripper: &http.Transport{
				TLSClientConfig: tlsConfig,
				Proxy:           http.ProxyFromEnvironment,
				Dial: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).Dial,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			},
			Header: headers,
		},
	}, nil
}

// refreshAuthToken refreshes a JWT auth token if it is invalid (e.g. expired)
func (c *client) refreshAuthToken(localCfg *localconfig.LocalConfig, ctxName, configPath string) error {
	if c.RefreshToken == "" {
		// If we have no refresh token, there's no point in doing anything
		return nil
	}
	configCtx, err := localCfg.ResolveContext(ctxName)
	if err != nil {
		return err
	}
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	var claims jwt.RegisteredClaims
	_, _, err = parser.ParseUnverified(configCtx.User.AuthToken, &claims)
	if err != nil {
		return err
	}
	if claims.Valid() == nil {
		// token is still valid
		return nil
	}

	log.Debug("Auth token no longer valid. Refreshing")
	rawIDToken, refreshToken, err := c.redeemRefreshToken()
	if err != nil {
		return err
	}
	c.AuthToken = rawIDToken
	c.RefreshToken = refreshToken
	localCfg.UpsertUser(localconfig.User{
		Name:         ctxName,
		AuthToken:    c.AuthToken,
		RefreshToken: c.RefreshToken,
	})
	err = localconfig.WriteLocalConfig(*localCfg, configPath)
	if err != nil {
		return err
	}
	return nil
}

// redeemRefreshToken performs the exchange of a refresh_token for a new id_token and refresh_token
func (c *client) redeemRefreshToken() (string, string, error) {
	setConn, setIf, err := c.NewSettingsClient()
	if err != nil {
		return "", "", err
	}
	defer func() { _ = setConn.Close() }()
	httpClient, err := c.HTTPClient()
	if err != nil {
		return "", "", err
	}
	ctx := oidc.ClientContext(context.Background(), httpClient)
	acdSet, err := setIf.Get(ctx, &settingspkg.SettingsQuery{})
	if err != nil {
		return "", "", err
	}
	oauth2conf, _, err := c.OIDCConfig(ctx, acdSet)
	if err != nil {
		return "", "", err
	}
	t := &oauth2.Token{
		RefreshToken: c.RefreshToken,
	}
	token, err := oauth2conf.TokenSource(ctx, t).Token()
	if err != nil {
		return "", "", err
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return "", "", errors.New("no id_token in token response")
	}
	refreshToken, _ := token.Extra("refresh_token").(string)
	return rawIDToken, refreshToken, nil
}

// NewClientOrDie creates a new API client from a set of config options, or fails fatally if the new client creation fails.
func NewClientOrDie(opts *ClientOptions) Client {
	client, err := NewClient(opts)
	if err != nil {
		log.Fatal(err)
	}
	return client
}

// JwtCredentials implements the gRPC credentials.Credentials interface which we is used to do
// grpc.WithPerRPCCredentials(), for authentication
type jwtCredentials struct {
	Token string
}

func (c jwtCredentials) RequireTransportSecurity() bool {
	return false
}

func (c jwtCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		MetaDataTokenKey: c.Token,
	}, nil
}

func (c *client) newConn() (*grpc.ClientConn, io.Closer, error) {
	closers := make([]io.Closer, 0)
	serverAddr := c.ServerAddr
	network := "tcp"
	if c.GRPCWeb || c.GRPCWebRootPath != "" {
		// start local grpc server which proxies requests using grpc-web protocol
		addr, closer, err := c.useGRPCProxy()
		if err != nil {
			return nil, nil, err
		}
		network = addr.Network()
		serverAddr = addr.String()
		closers = append(closers, closer)
	}

	var creds credentials.TransportCredentials
	if !c.PlainText && !c.GRPCWeb && c.GRPCWebRootPath == "" {
		tlsConfig, err := c.tlsConfig()
		if err != nil {
			return nil, nil, err
		}
		creds = credentials.NewTLS(tlsConfig)
	}
	endpointCredentials := jwtCredentials{
		Token: c.AuthToken,
	}
	retryOpts := []grpc_retry.CallOption{
		grpc_retry.WithMax(3),
		grpc_retry.WithBackoff(grpc_retry.BackoffLinear(1000 * time.Millisecond)),
	}
	var dialOpts []grpc.DialOption
	dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(endpointCredentials))
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxGRPCMessageSize), grpc.MaxCallSendMsgSize(MaxGRPCMessageSize)))
	dialOpts = append(dialOpts, grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(retryOpts...)))
	dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(grpc_middleware.ChainUnaryClient(grpc_retry.UnaryClientInterceptor(retryOpts...))))
	dialOpts = append(dialOpts, grpc.WithUnaryInterceptor(grpc_util.OTELUnaryClientInterceptor()))
	dialOpts = append(dialOpts, grpc.WithStreamInterceptor(grpc_util.OTELStreamClientInterceptor()))

	ctx := context.Background()

	headers, err := parseHeaders(c.Headers)
	if err != nil {
		return nil, nil, err
	}
	for k, vs := range headers {
		for _, v := range vs {
			ctx = metadata.AppendToOutgoingContext(ctx, k, v)
		}
	}

	if c.UserAgent != "" {
		dialOpts = append(dialOpts, grpc.WithUserAgent(c.UserAgent))
	}
	conn, e := grpc_util.BlockingDial(ctx, network, serverAddr, creds, dialOpts...)
	closers = append(closers, conn)
	return conn, argoio.NewCloser(func() error {
		var firstErr error
		for i := range closers {
			err := closers[i].Close()
			if err != nil {
				firstErr = err
			}
		}
		return firstErr
	}), e
}

func (c *client) tlsConfig() (*tls.Config, error) {
	var tlsConfig tls.Config
	if len(c.CertPEMData) > 0 {
		cp := tls_util.BestEffortSystemCertPool()
		if !cp.AppendCertsFromPEM(c.CertPEMData) {
			return nil, fmt.Errorf("credentials: failed to append certificates")
		}
		tlsConfig.RootCAs = cp
	}
	if c.ClientCert != nil {
		tlsConfig.Certificates = append(tlsConfig.Certificates, *c.ClientCert)
	}
	if c.Insecure {
		tlsConfig.InsecureSkipVerify = true
	}
	return &tlsConfig, nil
}

func (c *client) ClientOptions() ClientOptions {
	return ClientOptions{
		ServerAddr: c.ServerAddr,
		PlainText:  c.PlainText,
		Insecure:   c.Insecure,
		AuthToken:  c.AuthToken,
	}
}

func (c *client) NewRepoClient() (io.Closer, repositorypkg.RepositoryServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	repoIf := repositorypkg.NewRepositoryServiceClient(conn)
	return closer, repoIf, nil
}

func (c *client) NewRepoClientOrDie() (io.Closer, repositorypkg.RepositoryServiceClient) {
	conn, repoIf, err := c.NewRepoClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, repoIf
}

func (c *client) NewRepoCredsClient() (io.Closer, repocredspkg.RepoCredsServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	repoIf := repocredspkg.NewRepoCredsServiceClient(conn)
	return closer, repoIf, nil
}

func (c *client) NewRepoCredsClientOrDie() (io.Closer, repocredspkg.RepoCredsServiceClient) {
	conn, repoIf, err := c.NewRepoCredsClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, repoIf
}

func (c *client) NewCertClient() (io.Closer, certificatepkg.CertificateServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	certIf := certificatepkg.NewCertificateServiceClient(conn)
	return closer, certIf, nil
}

func (c *client) NewCertClientOrDie() (io.Closer, certificatepkg.CertificateServiceClient) {
	conn, certIf, err := c.NewCertClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, certIf
}

func (c *client) NewClusterClient() (io.Closer, clusterpkg.ClusterServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	clusterIf := clusterpkg.NewClusterServiceClient(conn)
	return closer, clusterIf, nil
}

func (c *client) NewClusterClientOrDie() (io.Closer, clusterpkg.ClusterServiceClient) {
	conn, clusterIf, err := c.NewClusterClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, clusterIf
}

func (c *client) NewGPGKeyClient() (io.Closer, gpgkeypkg.GPGKeyServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	gpgkeyIf := gpgkeypkg.NewGPGKeyServiceClient(conn)
	return closer, gpgkeyIf, nil
}

func (c *client) NewGPGKeyClientOrDie() (io.Closer, gpgkeypkg.GPGKeyServiceClient) {
	conn, gpgkeyIf, err := c.NewGPGKeyClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, gpgkeyIf
}

func (c *client) NewApplicationClient() (io.Closer, applicationpkg.ApplicationServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	appIf := applicationpkg.NewApplicationServiceClient(conn)
	return closer, appIf, nil
}

func (c *client) NewApplicationSetClient() (io.Closer, applicationsetpkg.ApplicationSetServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	appIf := applicationsetpkg.NewApplicationSetServiceClient(conn)
	return closer, appIf, nil
}

func (c *client) NewApplicationClientOrDie() (io.Closer, applicationpkg.ApplicationServiceClient) {
	conn, appIf, err := c.NewApplicationClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, appIf
}

func (c *client) NewNotificationClient() (io.Closer, notificationpkg.NotificationServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	notifIf := notificationpkg.NewNotificationServiceClient(conn)
	return closer, notifIf, nil
}

func (c *client) NewNotificationClientOrDie() (io.Closer, notificationpkg.NotificationServiceClient) {
	conn, notifIf, err := c.NewNotificationClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, notifIf
}

func (c *client) NewApplicationSetClientOrDie() (io.Closer, applicationsetpkg.ApplicationSetServiceClient) {
	conn, repoIf, err := c.NewApplicationSetClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, repoIf
}

func (c *client) NewSessionClient() (io.Closer, sessionpkg.SessionServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	sessionIf := sessionpkg.NewSessionServiceClient(conn)
	return closer, sessionIf, nil
}

func (c *client) NewSessionClientOrDie() (io.Closer, sessionpkg.SessionServiceClient) {
	conn, sessionIf, err := c.NewSessionClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, sessionIf
}

func (c *client) NewSettingsClient() (io.Closer, settingspkg.SettingsServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	setIf := settingspkg.NewSettingsServiceClient(conn)
	return closer, setIf, nil
}

func (c *client) NewSettingsClientOrDie() (io.Closer, settingspkg.SettingsServiceClient) {
	conn, setIf, err := c.NewSettingsClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, setIf
}

func (c *client) NewVersionClient() (io.Closer, versionpkg.VersionServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	versionIf := versionpkg.NewVersionServiceClient(conn)
	return closer, versionIf, nil
}

func (c *client) NewVersionClientOrDie() (io.Closer, versionpkg.VersionServiceClient) {
	conn, versionIf, err := c.NewVersionClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, versionIf
}

func (c *client) NewProjectClient() (io.Closer, projectpkg.ProjectServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	projIf := projectpkg.NewProjectServiceClient(conn)
	return closer, projIf, nil
}

func (c *client) NewProjectClientOrDie() (io.Closer, projectpkg.ProjectServiceClient) {
	conn, projIf, err := c.NewProjectClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, projIf
}

func (c *client) NewAccountClient() (io.Closer, accountpkg.AccountServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	usrIf := accountpkg.NewAccountServiceClient(conn)
	return closer, usrIf, nil
}

func (c *client) NewAccountClientOrDie() (io.Closer, accountpkg.AccountServiceClient) {
	conn, usrIf, err := c.NewAccountClient()
	if err != nil {
		log.Fatalf("Failed to establish connection to %s: %v", c.ServerAddr, err)
	}
	return conn, usrIf
}

// WatchApplicationWithRetry returns a channel of watch events for an application, retrying the
// watch upon errors. Closes the returned channel when the context is cancelled.
func (c *client) WatchApplicationWithRetry(ctx context.Context, appName string, revision string) chan *v1alpha1.ApplicationWatchEvent {
	appEventsCh := make(chan *v1alpha1.ApplicationWatchEvent)
	cancelled := false
	appName, appNs := argo.ParseFromQualifiedName(appName, "")
	go func() {
		defer close(appEventsCh)
		for !cancelled {
			conn, appIf, err := c.NewApplicationClient()
			if err == nil {
				var wc applicationpkg.ApplicationService_WatchClient
				wc, err = appIf.Watch(ctx, &applicationpkg.ApplicationQuery{
					Name:            &appName,
					AppNamespace:    &appNs,
					ResourceVersion: &revision,
				})
				if err == nil {
					for {
						var appEvent *v1alpha1.ApplicationWatchEvent
						appEvent, err = wc.Recv()
						if err != nil {
							break
						}
						revision = appEvent.Application.ResourceVersion
						appEventsCh <- appEvent
					}
				}
			}
			if err != nil {
				if isCanceledContextErr(err) {
					cancelled = true
				} else {
					time.Sleep(1 * time.Second)
				}
			}
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()
	return appEventsCh
}

func isCanceledContextErr(err error) bool {
	if err != nil && errors.Is(err, context.Canceled) {
		return true
	}
	if stat, ok := status.FromError(err); ok {
		if stat.Code() == codes.Canceled || stat.Code() == codes.DeadlineExceeded {
			return true
		}
	}
	return false
}

func parseHeaders(headerStrings []string) (http.Header, error) {
	headers := http.Header{}
	for _, kv := range headerStrings {
		i := strings.IndexByte(kv, ':')
		// zero means meaningless empty header name
		if i <= 0 {
			return nil, fmt.Errorf("additional headers must be colon(:)-separated: %s", kv)
		}
		headers.Add(kv[0:i], kv[i+1:])
	}
	return headers, nil
}
