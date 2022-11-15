package extension

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/ghodss/yaml"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

const (
	URLPrefix               = "/extensions"
	HeaderArgoCDClusterName = "Argocd-Cluster-Name"
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
	IdleConnectionTimeout time.Duration   `json:"idleConnectionTimeout"`
	Services              []ServiceConfig `json:"services"`
}

type ServiceConfig struct {
	URL         string `json:"url"`
	ClusterName string `json:"clusterName"`
}

type manager struct {
	log         *log.Entry
	settingsMgr *settings.SettingsManager
}

func NewManager(settingsMgr *settings.SettingsManager, log *log.Entry) *manager {
	return &manager{
		log:         log,
		settingsMgr: settingsMgr,
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

func NewProxy(targetURL string) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(url), nil
}

func (m *manager) MustRegisterHandlers(r *mux.Router) {
	m.log.Info("Registering extension handlers...")
	err := m.RegisterHandlers(r)
	if err != nil {
		panic(fmt.Sprintf("error registering extension handlers: %s", err))
	}
}

func (m *manager) RegisterHandlers(r *mux.Router) error {
	config, err := m.settingsMgr.GetSettings()
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
			proxy, err := NewProxy(service.URL)
			if err != nil {
				return fmt.Errorf("error creating proxy: %s", err)
			}
			proxyByCluster[service.ClusterName] = proxy
		}
		m.log.Infof("Registering handler for /%s/%s...", URLPrefix, ext.Name)
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
		m.log.Infof("Request for extension received: %s", r.URL.String())
		r.URL.Path = strings.TrimPrefix(r.URL.String(), fmt.Sprintf("%s/%s", URLPrefix, extName))
		if len(proxyByCluster) == 1 {
			for _, proxy := range proxyByCluster {
				proxy.ServeHTTP(w, r)
				return
			}
		}

		clusterName := r.Header.Get(HeaderArgoCDClusterName)
		if clusterName == "" {
			clusterName = "kubernetes.local"
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
