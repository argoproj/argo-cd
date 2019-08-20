package apiclient

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc"
	"github.com/dgrijalva/jwt-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/common"
	accountpkg "github.com/argoproj/argo-cd/pkg/apiclient/account"
	applicationpkg "github.com/argoproj/argo-cd/pkg/apiclient/application"
	certificatepkg "github.com/argoproj/argo-cd/pkg/apiclient/certificate"
	clusterpkg "github.com/argoproj/argo-cd/pkg/apiclient/cluster"
	projectpkg "github.com/argoproj/argo-cd/pkg/apiclient/project"
	repositorypkg "github.com/argoproj/argo-cd/pkg/apiclient/repository"
	sessionpkg "github.com/argoproj/argo-cd/pkg/apiclient/session"
	settingspkg "github.com/argoproj/argo-cd/pkg/apiclient/settings"
	versionpkg "github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/localconfig"
	oidcutil "github.com/argoproj/argo-cd/util/oidc"
	tls_util "github.com/argoproj/argo-cd/util/tls"
)

const (
	MetaDataTokenKey = "token"
	// EnvArgoCDServer is the environment variable to look for an Argo CD server address
	EnvArgoCDServer = "ARGOCD_SERVER"
	// EnvArgoCDAuthToken is the environment variable to look for an Argo CD auth token
	EnvArgoCDAuthToken = "ARGOCD_AUTH_TOKEN"
	// MaxGRPCMessageSize contains max grpc message size
	MaxGRPCMessageSize = 100 * 1024 * 1024
)

// Client defines an interface for interaction with an Argo CD server.
type Client interface {
	ClientOptions() ClientOptions
	HTTPClient() (*http.Client, error)
	OIDCConfig(context.Context, *settingspkg.Settings) (*oauth2.Config, *oidc.Provider, error)
	NewRepoClient() (io.Closer, repositorypkg.RepositoryServiceClient, error)
	NewRepoClientOrDie() (io.Closer, repositorypkg.RepositoryServiceClient)
	NewCertClient() (io.Closer, certificatepkg.CertificateServiceClient, error)
	NewCertClientOrDie() (io.Closer, certificatepkg.CertificateServiceClient)
	NewClusterClient() (io.Closer, clusterpkg.ClusterServiceClient, error)
	NewClusterClientOrDie() (io.Closer, clusterpkg.ClusterServiceClient)
	NewApplicationClient() (io.Closer, applicationpkg.ApplicationServiceClient, error)
	NewApplicationClientOrDie() (io.Closer, applicationpkg.ApplicationServiceClient)
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
	WatchApplicationWithRetry(ctx context.Context, appName string) chan *argoappv1.ApplicationWatchEvent
}

// ClientOptions hold address, security, and other settings for the API client.
type ClientOptions struct {
	ServerAddr string
	PlainText  bool
	Insecure   bool
	CertFile   string
	AuthToken  string
	ConfigPath string
	Context    string
	UserAgent  string
	GRPCWeb    bool
}

type client struct {
	ServerAddr   string
	PlainText    bool
	Insecure     bool
	CertPEMData  []byte
	AuthToken    string
	RefreshToken string
	UserAgent    string
	GRPCWeb      bool

	proxyMutex      *sync.Mutex
	proxyListener   net.Listener
	proxyServer     *grpc.Server
	proxyUsersCount int
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
			c.PlainText = configCtx.Server.PlainText
			c.Insecure = configCtx.Server.Insecure
			c.GRPCWeb = configCtx.Server.GRPCWeb
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
	if serverFromEnv := os.Getenv(EnvArgoCDServer); serverFromEnv != "" {
		c.ServerAddr = serverFromEnv
	}
	if opts.ServerAddr != "" {
		c.ServerAddr = opts.ServerAddr
	}
	// Make sure we got the server address and auth token from somewhere
	if c.ServerAddr == "" {
		return nil, errors.New("Argo CD server address unspecified")
	}
	if parts := strings.Split(c.ServerAddr, ":"); len(parts) == 1 {
		// If port is unspecified, assume the most likely port
		c.ServerAddr += ":443"
	}
	// Override auth-token if specified in env variable or CLI flag
	if authFromEnv := os.Getenv(EnvArgoCDAuthToken); authFromEnv != "" {
		c.AuthToken = authFromEnv
	}
	if opts.AuthToken != "" {
		c.AuthToken = opts.AuthToken
	}
	// Override certificate data if specified from CLI flag
	if opts.CertFile != "" {
		b, err := ioutil.ReadFile(opts.CertFile)
		if err != nil {
			return nil, err
		}
		c.CertPEMData = b
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
	if localCfg != nil {
		err = c.refreshAuthToken(localCfg, ctxName, opts.ConfigPath)
		if err != nil {
			return nil, err
		}
	}
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
		return nil, nil, fmt.Errorf("Failed to query provider %q: %v", issuerURL, err)
	}
	oidcConf, err := oidcutil.ParseConfig(provider)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to parse provider config: %v", err)
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
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
			Proxy:           http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
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
	parser := &jwt.Parser{
		SkipClaimsValidation: true,
	}
	var claims jwt.StandardClaims
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
	if c.GRPCWeb {
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
	if !c.PlainText && !c.GRPCWeb {
		tlsConfig, err := c.tlsConfig()
		if err != nil {
			return nil, nil, err
		}
		creds = credentials.NewTLS(tlsConfig)
	}
	endpointCredentials := jwtCredentials{
		Token: c.AuthToken,
	}
	var dialOpts []grpc.DialOption
	dialOpts = append(dialOpts, grpc.WithPerRPCCredentials(endpointCredentials))
	dialOpts = append(dialOpts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(MaxGRPCMessageSize)))
	if c.UserAgent != "" {
		dialOpts = append(dialOpts, grpc.WithUserAgent(c.UserAgent))
	}
	conn, e := grpc_util.BlockingDial(context.Background(), network, serverAddr, creds, dialOpts...)
	closers = append(closers, conn)
	return conn, &inlineCloser{close: func() error {
		var firstErr error
		for i := range closers {
			err := closers[i].Close()
			if err != nil {
				firstErr = err
			}
		}
		return firstErr
	}}, e
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

func (c *client) NewApplicationClient() (io.Closer, applicationpkg.ApplicationServiceClient, error) {
	conn, closer, err := c.newConn()
	if err != nil {
		return nil, nil, err
	}
	appIf := applicationpkg.NewApplicationServiceClient(conn)
	return closer, appIf, nil
}

func (c *client) NewApplicationClientOrDie() (io.Closer, applicationpkg.ApplicationServiceClient) {
	conn, repoIf, err := c.NewApplicationClient()
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
func (c *client) WatchApplicationWithRetry(ctx context.Context, appName string) chan *argoappv1.ApplicationWatchEvent {
	appEventsCh := make(chan *argoappv1.ApplicationWatchEvent)
	cancelled := false
	go func() {
		defer close(appEventsCh)
		for !cancelled {
			conn, appIf, err := c.NewApplicationClient()
			if err == nil {
				var wc applicationpkg.ApplicationService_WatchClient
				wc, err = appIf.Watch(ctx, &applicationpkg.ApplicationQuery{Name: &appName})
				if err == nil {
					for {
						var appEvent *v1alpha1.ApplicationWatchEvent
						appEvent, err = wc.Recv()
						if err != nil {
							break
						}
						appEventsCh <- appEvent
					}
				}
			}
			if err != nil {
				if isCanceledContextErr(err) {
					cancelled = true
				} else {
					if err != io.EOF {
						log.Warnf("watch err: %v", err)
					}
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
	if err == context.Canceled {
		return true
	}
	if stat, ok := status.FromError(err); ok {
		if stat.Code() == codes.Canceled || stat.Code() == codes.DeadlineExceeded {
			return true
		}
	}
	return false
}
