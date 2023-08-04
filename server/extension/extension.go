package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	v1alpha1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"
)

const (
	URLPrefix                    = "/extensions"
	HeaderArgoCDApplicationName  = "Argocd-Application-Name"
	DefaultConnectionTimeout     = 2 * time.Second
	DefaultKeepAlive             = 15 * time.Second
	DefaultIdleConnectionTimeout = 60 * time.Second
	DefaultMaxIdleConnections    = 30
)

// ExtensionConfigs defines the configurations for all extensions
// retrieved from Argo CD configmap (argocd-cm).
type ExtensionConfigs struct {
	Extensions []ExtensionConfig `json:"extensions"`
}

// ExtensionConfig defines the configuration for one extension.
type ExtensionConfig struct {
	// Name defines the endpoint that will be used to register
	// the extension route. Mandatory field.
	Name    string        `json:"name"`
	Backend BackendConfig `json:"backend"`
}

// BackendConfig defines the backend service configurations that will
// be used by an specific extension. An extension can have multiple services
// associated. This is necessary when Argo CD is managing applications in
// external clusters. In this case, each cluster may have its own backend
// service.
type BackendConfig struct {
	ProxyConfig
	Services []ServiceConfig `json:"services"`
}

// ProxyConfig allows configuring connection behaviour between Argo CD
// API Server and the backend service.
type ProxyConfig struct {
	// ConnectionTimeout is the maximum amount of time a dial to
	// the extension server will wait for a connect to complete.
	// Default: 2 seconds
	ConnectionTimeout time.Duration `json:"connectionTimeout"`

	// KeepAlive specifies the interval between keep-alive probes
	// for an active network connection between the API server and
	// the extension server.
	// Default: 15 seconds
	KeepAlive time.Duration `json:"keepAlive"`

	// IdleConnectionTimeout is the maximum amount of time an idle
	// (keep-alive) connection between the API server and the extension
	// server will remain idle before closing itself.
	// Default: 60 seconds
	IdleConnectionTimeout time.Duration `json:"idleConnectionTimeout"`

	// MaxIdleConnections controls the maximum number of idle (keep-alive)
	// connections between the API server and the extension server.
	// Default: 30
	MaxIdleConnections int `json:"maxIdleConnections"`
}

// ServiceConfig provides the configuration for a backend service.
type ServiceConfig struct {
	// URL is the address where the extension backend must be available.
	// Mandatory field.
	URL string `json:"url"`

	// Cluster if provided, will have to match the application
	// destination name to have requests properly forwarded to this
	// service URL.
	Cluster string `json:"cluster"`
}

// SettingsGetter defines the contract to retrieve Argo CD Settings.
type SettingsGetter interface {
	Get() (*settings.ArgoCDSettings, error)
}

// DefaultSettingsGetter is the real settings getter implementation.
type DefaultSettingsGetter struct {
	settingsMgr *settings.SettingsManager
}

// NewDefaultSettingsGetter returns a new default settings getter.
func NewDefaultSettingsGetter(mgr *settings.SettingsManager) *DefaultSettingsGetter {
	return &DefaultSettingsGetter{
		settingsMgr: mgr,
	}
}

// Get will retrieve the Argo CD settings.
func (s *DefaultSettingsGetter) Get() (*settings.ArgoCDSettings, error) {
	return s.settingsMgr.GetSettings()
}

// ApplicationGetter defines the contract to retrieve the application resource.
type ApplicationGetter interface {
	Get(ns, name string) (*v1alpha1.Application, error)
}

// DefaultApplicationGetter is the real application getter implementation.
type DefaultApplicationGetter struct {
	svc applicationpkg.ApplicationServiceServer
}

// NewDefaultApplicationGetter returns the default application getter.
func NewDefaultApplicationGetter(appSvc applicationpkg.ApplicationServiceServer) *DefaultApplicationGetter {
	return &DefaultApplicationGetter{
		svc: appSvc,
	}
}

// Get will retrieve the application resorce for the given namespace and name.
func (a *DefaultApplicationGetter) Get(ns, name string) (*v1alpha1.Application, error) {
	query := &applicationpkg.ApplicationQuery{
		Name:         pointer.String(name),
		AppNamespace: pointer.String(ns),
	}
	return a.svc.Get(context.Background(), query)
}

// Manager is the object that will be responsible for registering
// and handling proxy extensions.
type Manager struct {
	log         *log.Entry
	settings    SettingsGetter
	application ApplicationGetter
}

// NewManager will initialize a new manager.
func NewManager(sg SettingsGetter, ag ApplicationGetter, log *log.Entry) *Manager {
	return &Manager{
		log:         log,
		settings:    sg,
		application: ag,
	}
}

func parseAndValidateConfig(config string) (*ExtensionConfigs, error) {
	configs := ExtensionConfigs{}
	err := yaml.Unmarshal([]byte(config), &configs)
	if err != nil {
		return nil, fmt.Errorf("invalid yaml: %s", err)
	}
	err = validateConfigs(&configs)
	if err != nil {
		return nil, fmt.Errorf("validation error: %s", err)
	}
	return &configs, nil
}

func validateConfigs(configs *ExtensionConfigs) error {
	nameSafeRegex := regexp.MustCompile(`^[A-Za-z0-9-_]+$`)
	for _, ext := range configs.Extensions {
		if ext.Name == "" {
			return fmt.Errorf("extensions.name must be configured")
		}
		if !nameSafeRegex.MatchString(ext.Name) {
			return fmt.Errorf("invalid extensions.name: only alphanumeric characters, hyphens, and underscores are allowed")
		}
		svcTotal := len(ext.Backend.Services)
		for _, svc := range ext.Backend.Services {
			if svc.URL == "" {
				return fmt.Errorf("extensions.backend.services.url must be configured")
			}
			if svcTotal > 1 && svc.Cluster == "" {
				return fmt.Errorf("extensions.backend.services.cluster must be configured when defining more than one service per extension")
			}
		}
	}
	return nil
}

// NewProxy will instantiate a new reverse proxy based on the provided
// targetURL and config.
func NewProxy(targetURL string, config ProxyConfig) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %s", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.Transport = newTransport(config)
	return proxy, nil
}

// newTransport will build a new transport to be used in the proxy
// applying default values if not defined in the given config.
func newTransport(config ProxyConfig) *http.Transport {
	applyProxyConfigDefaults(&config)
	return &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.ConnectionTimeout,
			KeepAlive: config.KeepAlive,
		}).DialContext,
		MaxIdleConns:          config.MaxIdleConnections,
		IdleConnTimeout:       config.IdleConnectionTimeout,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func applyProxyConfigDefaults(c *ProxyConfig) {
	if c.ConnectionTimeout == 0 {
		c.ConnectionTimeout = DefaultConnectionTimeout
	}
	if c.KeepAlive == 0 {
		c.KeepAlive = DefaultKeepAlive
	}
	if c.IdleConnectionTimeout == 0 {
		c.IdleConnectionTimeout = DefaultIdleConnectionTimeout
	}
	if c.MaxIdleConnections == 0 {
		c.MaxIdleConnections = DefaultMaxIdleConnections
	}
}

// RegisterHandlers will retrieve all configured extensions
// and register the respective http handlers in the given
// router.
func (m *Manager) RegisterHandlers(r *mux.Router) error {
	m.log.Info("Registering extension handlers...")
	config, err := m.settings.Get()
	if err != nil {
		return fmt.Errorf("error getting settings: %s", err)
	}

	if config.ExtensionConfig == "" {
		return fmt.Errorf("No extensions configurations found")
	}

	extConfigs, err := parseAndValidateConfig(config.ExtensionConfig)
	if err != nil {
		return fmt.Errorf("error parsing extension config: %s", err)
	}
	return m.registerExtensions(r, extConfigs)
}

// registerExtensions will iterate over the given extConfigs and register
// http handlers for every extension. It also registers a list extensions
// handler under the "/extensions/" endpoint.
func (m *Manager) registerExtensions(r *mux.Router, extConfigs *ExtensionConfigs) error {
	extRouter := r.PathPrefix(fmt.Sprintf("%s/", URLPrefix)).Subrouter()
	for _, ext := range extConfigs.Extensions {
		proxyByCluster := make(map[string]*httputil.ReverseProxy)
		for _, service := range ext.Backend.Services {
			proxy, err := NewProxy(service.URL, ext.Backend.ProxyConfig)
			if err != nil {
				return fmt.Errorf("error creating proxy: %s", err)
			}
			proxyByCluster[service.Cluster] = proxy
		}
		m.log.Infof("Registering handler for %s/%s...", URLPrefix, ext.Name)
		extRouter.PathPrefix(fmt.Sprintf("/%s/", ext.Name)).
			HandlerFunc(m.CallExtension(ext.Name, proxyByCluster))
	}
	return nil
}

// CallExtension returns a handler func responsible for forwarding requests to the
// extension service. The request will be sanitized by removing sensitive headers.
func (m *Manager) CallExtension(extName string, proxyByCluster map[string]*httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sanitizeRequest(r, extName)
		if len(proxyByCluster) == 1 {
			for _, proxy := range proxyByCluster {
				proxy.ServeHTTP(w, r)
				return
			}
		}
		appHeader := r.Header.Get(HeaderArgoCDApplicationName)
		if appHeader == "" {
			msg := fmt.Sprintf("Header %q must be provided", HeaderArgoCDApplicationName)
			m.writeErrorResponse(http.StatusBadRequest, msg, w)
			return
		}
		appNamespace, appName, err := getAppName(appHeader)
		if err != nil {
			msg := fmt.Sprintf("Error getting application name: %s", err)
			m.writeErrorResponse(http.StatusBadRequest, msg, w)
			return
		}
		app, err := m.application.Get(appNamespace, appName)
		if err != nil {
			msg := fmt.Sprintf("Error getting application: %s", err)
			m.writeErrorResponse(http.StatusBadRequest, msg, w)
			return
		}
		if app == nil {
			msg := fmt.Sprintf("Invalid Application: %s", appHeader)
			m.writeErrorResponse(http.StatusBadRequest, msg, w)
			return
		}
		clusterName := app.Spec.Destination.Name
		if clusterName == "" {
			clusterName = app.Spec.Destination.Server
		}

		proxy, ok := proxyByCluster[clusterName]
		if !ok {
			msg := fmt.Sprintf("No extension configured for cluster %q", clusterName)
			m.writeErrorResponse(http.StatusBadRequest, msg, w)
			return
		}
		proxy.ServeHTTP(w, r)
	}
}

func getAppName(appHeader string) (string, string, error) {
	parts := strings.Split(appHeader, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid header value %q: expected format: <namespace>/<app-name>", appHeader)
	}
	return parts[0], parts[1], nil
}

func sanitizeRequest(r *http.Request, extName string) {
	r.URL.Path = strings.TrimPrefix(r.URL.String(), fmt.Sprintf("%s/%s", URLPrefix, extName))
}

func (m *Manager) writeErrorResponse(status int, message string, w http.ResponseWriter) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")
	resp := make(map[string]string)
	resp["status"] = http.StatusText(status)
	resp["message"] = message
	jsonResp, err := json.Marshal(resp)
	if err != nil {
		m.log.Errorf("Error marshaling response for extension: %s", err)
		return
	}
	_, err = w.Write(jsonResp)
	if err != nil {
		m.log.Errorf("Error writing response for extension: %s", err)
		return
	}
}
