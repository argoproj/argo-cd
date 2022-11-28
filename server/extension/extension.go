package extension

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
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
	URLPrefix                   = "/extensions"
	HeaderArgoCDApplicationName = "Argocd-Application-Name"
)

type ExtensionConfigs struct {
	Extensions []ExtensionConfig `json:"extensions"`
}

type ExtensionConfig struct {
	Name    string        `json:"name"`
	Enabled bool          `json:"enabled"`
	Backend BackendConfig `json:"backend"`
}

type BackendConfig struct {
	ProxyConfig
	Services []ServiceConfig `json:"services"`
}

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

type ServiceConfig struct {
	URL         string `json:"url"`
	ClusterName string `json:"clusterName"`
}

type SettingsGetter interface {
	Get() (*settings.ArgoCDSettings, error)
}

type DefaultSettingsGetter struct {
	settingsMgr *settings.SettingsManager
}

func NewDefaultSettingsGetter(mgr *settings.SettingsManager) *DefaultSettingsGetter {
	return &DefaultSettingsGetter{
		settingsMgr: mgr,
	}
}

func (s *DefaultSettingsGetter) Get() (*settings.ArgoCDSettings, error) {
	return s.settingsMgr.GetSettings()
}

type ApplicationGetter interface {
	Get(ns, name string) (*v1alpha1.Application, error)
}

type DefaultApplicationGetter struct {
	svc applicationpkg.ApplicationServiceServer
}

func NewDefaultApplicationGetter(appSvc applicationpkg.ApplicationServiceServer) *DefaultApplicationGetter {
	return &DefaultApplicationGetter{
		svc: appSvc,
	}
}

func (a *DefaultApplicationGetter) Get(ns, name string) (*v1alpha1.Application, error) {
	query := &applicationpkg.ApplicationQuery{
		Name:         pointer.String(name),
		AppNamespace: pointer.String(ns),
	}
	return a.svc.Get(context.Background(), query)
}

// type ClusterGetter interface {
// 	Get(*cluster.ClusterQuery) (*v1alpha1.Cluster, error)
// }
//
// type DefaultClusterGetter struct {
// 	service *cluster.ClusterServiceServer
// }
//
// func NewDefaultClusterGetter(service *cluster.ClusterServiceServer) *DefaultClusterGetter {
// 	return &DefaultClusterGetter{
// 		service: service,
// 	}
// }

type manager struct {
	log         *log.Entry
	settings    SettingsGetter
	application ApplicationGetter
	// cluster     ClusterGetter
}

func NewManager(sg SettingsGetter, ag ApplicationGetter, log *log.Entry) *manager {
	return &manager{
		log:         log,
		settings:    sg,
		application: ag,
	}
}

func parseConfig(config string) (*ExtensionConfigs, error) {
	configs := ExtensionConfigs{}
	err := yaml.Unmarshal([]byte(config), &configs)
	if err != nil {
		return nil, err
	}
	return &configs, nil
}

func NewProxy(targetURL string, config ProxyConfig) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	proxy := httputil.NewSingleHostReverseProxy(url)
	proxy.Transport = newTransport(config)
	return proxy, nil
}

// newTransport will build a new transport to be used in the proxy
// applying default values if not defined in the given config.
func newTransport(config ProxyConfig) *http.Transport {
	if config.ConnectionTimeout == 0 {
		config.ConnectionTimeout = 2 * time.Second
	}
	if config.KeepAlive == 0 {
		config.KeepAlive = 15 * time.Second
	}
	if config.IdleConnectionTimeout == 0 {
		config.IdleConnectionTimeout = 60 * time.Second
	}
	if config.MaxIdleConnections == 0 {
		config.MaxIdleConnections = 30
	}
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

func (m *manager) RegisterHandlers(r *mux.Router) error {
	m.log.Info("Registering extension handlers...")
	config, err := m.settings.Get()
	if err != nil {
		return fmt.Errorf("error getting settings: %s", err)
	}

	if config.ExtensionConfig == "" {
		m.log.Info("No extensions configurations found...")
		return nil
	}

	extConfigs, err := parseConfig(config.ExtensionConfig)
	if err != nil {
		return fmt.Errorf("error parsing extension config: %s", err)
	}
	return m.registerExtensions(r, extConfigs)
}

func (m *manager) registerExtensions(r *mux.Router, extConfigs *ExtensionConfigs) error {
	extRouter := r.PathPrefix(fmt.Sprintf("%s/", URLPrefix)).Subrouter()
	for _, ext := range extConfigs.Extensions {
		proxyByCluster := make(map[string]*httputil.ReverseProxy)
		for _, service := range ext.Backend.Services {
			proxy, err := NewProxy(service.URL, ext.Backend.ProxyConfig)
			if err != nil {
				return fmt.Errorf("error creating proxy: %s", err)
			}
			proxyByCluster[service.ClusterName] = proxy
		}
		m.log.Infof("Registering handler for %s/%s...", URLPrefix, ext.Name)
		extRouter.PathPrefix(fmt.Sprintf("/%s/", ext.Name)).
			HandlerFunc(m.ProxyHandler(ext.Name, proxyByCluster))
	}
	extRouter.HandleFunc("/", m.ListExtensions(extConfigs))
	return nil
}
func (m *manager) ListExtensions(extConfigs *ExtensionConfigs) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		extJson, err := json.Marshal(extConfigs)
		if err != nil {
			msg := fmt.Sprintf("error building extensions list response: %s", err)
			m.writeErrorResponse(http.StatusInternalServerError, msg, w)
			return
		}
		_, err = w.Write(extJson)
		if err != nil {
			msg := fmt.Sprintf("error writing extensions list response: %s", err)
			m.writeErrorResponse(http.StatusInternalServerError, msg, w)
			return
		}
	}
}

func (m *manager) ProxyHandler(extName string, proxyByCluster map[string]*httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := sanitizeRequest(r, extName)
		if err != nil {
			msg := fmt.Sprintf("Error validating request: %s", err)
			m.writeErrorResponse(http.StatusBadRequest, msg, w)
			return
		}
		var app *v1alpha1.Application
		var appNamespace, appName string
		appHeader := r.Header.Get(HeaderArgoCDApplicationName)
		if appHeader != "" {
			appNamespace, appName, err = getAppName(appHeader)
			if err != nil {
				msg := fmt.Sprintf("Error getting application name: %s", err)
				m.writeErrorResponse(http.StatusBadRequest, msg, w)
				return
			}
			app, err = m.application.Get(appNamespace, appName)
			if err != nil {
				msg := fmt.Sprintf("Error getting application: %s", err)
				m.writeErrorResponse(http.StatusBadRequest, msg, w)
				return
			}

		}
		if len(proxyByCluster) == 1 {
			for _, proxy := range proxyByCluster {
				proxy.ServeHTTP(w, r)
				return
			}
		}

		clusterName := app.Spec.Destination.Name
		if clusterName == "" {
			clusterName = app.Spec.Destination.Server
		}

		proxy, ok := proxyByCluster[clusterName]
		if !ok {
			msg := fmt.Sprintf("No extension configured for cluster %q", appHeader)
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

func sanitizeRequest(r *http.Request, extName string) error {
	r.URL.Path = strings.TrimPrefix(r.URL.String(), fmt.Sprintf("%s/%s", URLPrefix, extName))
	return nil
}

func (m *manager) writeErrorResponse(status int, message string, w http.ResponseWriter) {
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
