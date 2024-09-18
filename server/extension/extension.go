package extension

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/felixge/httpsnoop"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/server/rbacpolicy"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/security"
	"github.com/argoproj/argo-cd/v2/util/session"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	URLPrefix                    = "/extensions"
	DefaultConnectionTimeout     = 2 * time.Second
	DefaultKeepAlive             = 15 * time.Second
	DefaultIdleConnectionTimeout = 60 * time.Second
	DefaultMaxIdleConnections    = 30

	// HeaderArgoCDApplicationName defines the name of the
	// expected application header to be passed to the extension
	// handler. The header value must follow the format:
	//     "<namespace>:<app-name>"
	// Example:
	//     Argocd-Application-Name: "namespace:app-name"
	HeaderArgoCDApplicationName = "Argocd-Application-Name"

	// HeaderArgoCDProjectName defines the name of the expected
	// project header to be passed to the extension handler.
	// Example:
	//     Argocd-Project-Name: "default"
	HeaderArgoCDProjectName = "Argocd-Project-Name"

	// HeaderArgoCDTargetClusterURL defines the target cluster URL
	// that the Argo CD application is associated with. This header
	// will be populated by the extension proxy and passed to the
	// configured backend service. If this header is passed by
	// the client, its value will be overridden by the extension
	// handler.
	//
	// Example:
	//     Argocd-Target-Cluster-URL: "https://kubernetes.default.svc.cluster.local"
	HeaderArgoCDTargetClusterURL = "Argocd-Target-Cluster-URL"

	// HeaderArgoCDTargetClusterName defines the target cluster name
	// that the Argo CD application is associated with. This header
	// will be populated by the extension proxy and passed to the
	// configured backend service. If this header is passed by
	// the client, its value will be overridden by the extension
	// handler.
	HeaderArgoCDTargetClusterName = "Argocd-Target-Cluster-Name"

	// HeaderArgoCDUsername is the header name that defines the logged
	// in user authenticated by Argo CD.
	HeaderArgoCDUsername = "Argocd-Username"

	// HeaderArgoCDGroups is the header name that provides the 'groups'
	// claim from the users authenticated in Argo CD.
	HeaderArgoCDGroups = "Argocd-User-Groups"
)

// RequestResources defines the authorization scope for
// an incoming request to a given extension. This struct
// is populated from pre-defined Argo CD headers.
type RequestResources struct {
	ApplicationName      string
	ApplicationNamespace string
	ProjectName          string
}

// ValidateHeaders will validate the pre-defined Argo CD
// request headers for extensions and extract the resources
// information populating and returning a RequestResources
// object.
// The pre-defined headers are:
// - Argocd-Application-Name
// - Argocd-Project-Name
//
// The headers expected format is documented in each of the constant
// types defined for them.
func ValidateHeaders(r *http.Request) (*RequestResources, error) {
	appHeader := r.Header.Get(HeaderArgoCDApplicationName)
	if appHeader == "" {
		return nil, fmt.Errorf("header %q must be provided", HeaderArgoCDApplicationName)
	}
	appNamespace, appName, err := getAppName(appHeader)
	if err != nil {
		return nil, fmt.Errorf("error getting app details: %w", err)
	}
	if !argo.IsValidNamespaceName(appNamespace) {
		return nil, errors.New("invalid value for namespace")
	}
	if !argo.IsValidAppName(appName) {
		return nil, errors.New("invalid value for application name")
	}

	projName := r.Header.Get(HeaderArgoCDProjectName)
	if projName == "" {
		return nil, fmt.Errorf("header %q must be provided", HeaderArgoCDProjectName)
	}
	if !argo.IsValidProjectName(projName) {
		return nil, errors.New("invalid value for project name")
	}
	return &RequestResources{
		ApplicationName:      appName,
		ApplicationNamespace: appNamespace,
		ProjectName:          projName,
	}, nil
}

func getAppName(appHeader string) (string, string, error) {
	parts := strings.Split(appHeader, ":")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid value for %q header: expected format: <namespace>:<app-name>", HeaderArgoCDApplicationName)
	}
	return parts[0], parts[1], nil
}

// ExtensionConfigs defines the configurations for all extensions
// retrieved from Argo CD configmap (argocd-cm).
type ExtensionConfigs struct {
	Extensions []ExtensionConfig `yaml:"extensions"`
}

// ExtensionConfig defines the configuration for one extension.
type ExtensionConfig struct {
	// Name defines the endpoint that will be used to register
	// the extension route. Mandatory field.
	Name    string        `yaml:"name"`
	Backend BackendConfig `yaml:"backend"`
}

// BackendConfig defines the backend service configurations that will
// be used by an specific extension. An extension can have multiple services
// associated. This is necessary when Argo CD is managing applications in
// external clusters. In this case, each cluster may have its own backend
// service.
type BackendConfig struct {
	ProxyConfig
	Services []ServiceConfig `yaml:"services"`
}

// ServiceConfig provides the configuration for a backend service.
type ServiceConfig struct {
	// URL is the address where the extension backend must be available.
	// Mandatory field.
	URL string `yaml:"url"`

	// Cluster if provided, will have to match the application
	// destination name to have requests properly forwarded to this
	// service URL.
	Cluster *ClusterConfig `yaml:"cluster,omitempty"`

	// Headers if provided, the headers list will be added on all
	// outgoing requests for this service config.
	Headers []Header `yaml:"headers"`
}

// Header defines the header to be added in the proxy requests.
type Header struct {
	// Name defines the name of the header. It is a mandatory field if
	// a header is provided.
	Name string `yaml:"name"`
	// Value defines the value of the header. The actual value can be
	// provided as verbatim or as a reference to an Argo CD secret key.
	// In order to provide it as a reference, it is necessary to prefix
	// it with a dollar sign.
	// Example:
	//   value: '$some.argocd.secret.key'
	// In the example above, the value will be replaced with the one from
	// the argocd-secret with key 'some.argocd.secret.key'.
	Value string `yaml:"value"`
}

type ClusterConfig struct {
	// Server specifies the URL of the target cluster's Kubernetes control plane API. This must be set if Name is not set.
	Server string `yaml:"server"`

	// Name is an alternate way of specifying the target cluster by its symbolic name. This must be set if Server is not set.
	Name string `yaml:"name"`
}

// ProxyConfig allows configuring connection behaviour between Argo CD
// API Server and the backend service.
type ProxyConfig struct {
	// ConnectionTimeout is the maximum amount of time a dial to
	// the extension server will wait for a connect to complete.
	// Default: 2 seconds
	ConnectionTimeout time.Duration `yaml:"connectionTimeout"`

	// KeepAlive specifies the interval between keep-alive probes
	// for an active network connection between the API server and
	// the extension server.
	// Default: 15 seconds
	KeepAlive time.Duration `yaml:"keepAlive"`

	// IdleConnectionTimeout is the maximum amount of time an idle
	// (keep-alive) connection between the API server and the extension
	// server will remain idle before closing itself.
	// Default: 60 seconds
	IdleConnectionTimeout time.Duration `yaml:"idleConnectionTimeout"`

	// MaxIdleConnections controls the maximum number of idle (keep-alive)
	// connections between the API server and the extension server.
	// Default: 30
	MaxIdleConnections int `yaml:"maxIdleConnections"`
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

// ProjectGetter defines the contract to retrieve Argo CD Project.
type ProjectGetter interface {
	Get(name string) (*v1alpha1.AppProject, error)
	GetClusters(project string) ([]*v1alpha1.Cluster, error)
}

// DefaultProjectGetter is the real ProjectGetter implementation.
type DefaultProjectGetter struct {
	projLister applisters.AppProjectNamespaceLister
	db         db.ArgoDB
}

// NewDefaultProjectGetter returns a new default project getter
func NewDefaultProjectGetter(lister applisters.AppProjectNamespaceLister, db db.ArgoDB) *DefaultProjectGetter {
	return &DefaultProjectGetter{
		projLister: lister,
		db:         db,
	}
}

// Get will retrieve the live AppProject state.
func (p *DefaultProjectGetter) Get(name string) (*v1alpha1.AppProject, error) {
	return p.projLister.Get(name)
}

// GetClusters will retrieve the clusters configured by a project.
func (p *DefaultProjectGetter) GetClusters(project string) ([]*v1alpha1.Cluster, error) {
	return p.db.GetProjectClusters(context.TODO(), project)
}

// UserGetter defines the contract to retrieve info from the logged in user.
type UserGetter interface {
	GetUser(ctx context.Context) string
	GetGroups(ctx context.Context) []string
}

// DefaultUserGetter is the main UserGetter implementation.
type DefaultUserGetter struct {
	policyEnf *rbacpolicy.RBACPolicyEnforcer
}

// NewDefaultUserGetter return a new default UserGetter
func NewDefaultUserGetter(policyEnf *rbacpolicy.RBACPolicyEnforcer) *DefaultUserGetter {
	return &DefaultUserGetter{
		policyEnf: policyEnf,
	}
}

// GetUser will return the current logged in user
func (u *DefaultUserGetter) GetUser(ctx context.Context) string {
	return session.Username(ctx)
}

// GetGroups will return the groups associated with the logged in user.
func (u *DefaultUserGetter) GetGroups(ctx context.Context) []string {
	return session.Groups(ctx, u.policyEnf.GetScopes())
}

// ApplicationGetter defines the contract to retrieve the application resource.
type ApplicationGetter interface {
	Get(ns, name string) (*v1alpha1.Application, error)
}

// DefaultApplicationGetter is the real application getter implementation.
type DefaultApplicationGetter struct {
	appLister applisters.ApplicationLister
}

// NewDefaultApplicationGetter returns the default application getter.
func NewDefaultApplicationGetter(al applisters.ApplicationLister) *DefaultApplicationGetter {
	return &DefaultApplicationGetter{
		appLister: al,
	}
}

// Get will retrieve the application resource for the given namespace and name.
func (a *DefaultApplicationGetter) Get(ns, name string) (*v1alpha1.Application, error) {
	return a.appLister.Applications(ns).Get(name)
}

// RbacEnforcer defines the contract to enforce rbac rules
type RbacEnforcer interface {
	EnforceErr(rvals ...interface{}) error
}

// Manager is the object that will be responsible for registering
// and handling proxy extensions.
type Manager struct {
	log         *log.Entry
	settings    SettingsGetter
	application ApplicationGetter
	project     ProjectGetter
	rbac        RbacEnforcer
	registry    ExtensionRegistry
	metricsReg  ExtensionMetricsRegistry
	userGetter  UserGetter
}

// ExtensionMetricsRegistry exposes operations to update http metrics in the Argo CD
// API server.
type ExtensionMetricsRegistry interface {
	// IncExtensionRequestCounter will increase the request counter for the given
	// extension with the given status.
	IncExtensionRequestCounter(extension string, status int)
	// ObserveExtensionRequestDuration will register the request roundtrip duration
	// between Argo CD API Server and the extension backend service for the given
	// extension.
	ObserveExtensionRequestDuration(extension string, duration time.Duration)
}

// NewManager will initialize a new manager.
func NewManager(log *log.Entry, sg SettingsGetter, ag ApplicationGetter, pg ProjectGetter, rbac RbacEnforcer, ug UserGetter) *Manager {
	return &Manager{
		log:         log,
		settings:    sg,
		application: ag,
		project:     pg,
		rbac:        rbac,
		userGetter:  ug,
	}
}

// ExtensionRegistry is an in memory registry that contains contains all
// proxies for all extensions. The key is the extension name defined in
// the Argo CD configmap.
type ExtensionRegistry map[string]ProxyRegistry

// ProxyRegistry is an in memory registry that contains all proxies for a
// given extension. Different extensions will have independent proxy registries.
// This is required to address the use case when one extension is configured with
// multiple backend services in different clusters.
type ProxyRegistry map[ProxyKey]*httputil.ReverseProxy

// NewProxyRegistry will instantiate a new in memory registry for proxies.
func NewProxyRegistry() ProxyRegistry {
	r := make(map[ProxyKey]*httputil.ReverseProxy)
	return r
}

// ProxyKey defines the struct used as a key in the proxy registry
// map (ProxyRegistry).
type ProxyKey struct {
	extensionName string
	clusterName   string
	clusterServer string
}

// proxyKey will build the key to be used in the proxyByCluster
// map.
func proxyKey(extName, cName, cServer string) ProxyKey {
	return ProxyKey{
		extensionName: extName,
		clusterName:   cName,
		clusterServer: cServer,
	}
}

func parseAndValidateConfig(s *settings.ArgoCDSettings) (*ExtensionConfigs, error) {
	if s.ExtensionConfig == "" {
		return nil, fmt.Errorf("no extensions configurations found")
	}

	extConfigMap := map[string]interface{}{}
	err := yaml.Unmarshal([]byte(s.ExtensionConfig), &extConfigMap)
	if err != nil {
		return nil, fmt.Errorf("invalid extension config: %w", err)
	}

	parsedExtConfig := settings.ReplaceMapSecrets(extConfigMap, s.Secrets)
	parsedExtConfigBytes, err := yaml.Marshal(parsedExtConfig)
	if err != nil {
		return nil, fmt.Errorf("error marshaling parsed extension config: %w", err)
	}

	configs := ExtensionConfigs{}
	err = yaml.Unmarshal(parsedExtConfigBytes, &configs)
	if err != nil {
		return nil, fmt.Errorf("invalid parsed extension config: %w", err)
	}
	err = validateConfigs(&configs)
	if err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}
	return &configs, nil
}

func validateConfigs(configs *ExtensionConfigs) error {
	nameSafeRegex := regexp.MustCompile(`^[A-Za-z0-9-_]+$`)
	exts := make(map[string]struct{})
	for _, ext := range configs.Extensions {
		if ext.Name == "" {
			return fmt.Errorf("extensions.name must be configured")
		}
		if !nameSafeRegex.MatchString(ext.Name) {
			return fmt.Errorf("invalid extensions.name: only alphanumeric characters, hyphens, and underscores are allowed")
		}
		if _, found := exts[ext.Name]; found {
			return fmt.Errorf("duplicated extension found in the configs for %q", ext.Name)
		}
		exts[ext.Name] = struct{}{}
		svcTotal := len(ext.Backend.Services)
		if svcTotal == 0 {
			return fmt.Errorf("no backend service configured for extension %s", ext.Name)
		}
		for _, svc := range ext.Backend.Services {
			if svc.URL == "" {
				return fmt.Errorf("extensions.backend.services.url must be configured")
			}
			if svcTotal > 1 && svc.Cluster == nil {
				return fmt.Errorf("extensions.backend.services.cluster must be configured when defining more than one service per extension")
			}
			if svc.Cluster != nil {
				if svc.Cluster.Name == "" && svc.Cluster.Server == "" {
					return fmt.Errorf("cluster.name or cluster.server must be defined when cluster is provided in the configuration")
				}
			}
			if len(svc.Headers) > 0 {
				for _, header := range svc.Headers {
					if header.Name == "" {
						return fmt.Errorf("header.name must be defined when providing service headers in the configuration")
					}
					if header.Value == "" {
						return fmt.Errorf("header.value must be defined when providing service headers in the configuration")
					}
				}
			}
		}
	}
	return nil
}

// NewProxy will instantiate a new reverse proxy based on the provided
// targetURL and config. It will remove sensitive information from the
// incoming request such as the Authorization and Cookie headers.
func NewProxy(targetURL string, headers []Header, config ProxyConfig) (*httputil.ReverseProxy, error) {
	url, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proxy URL: %w", err)
	}
	proxy := &httputil.ReverseProxy{
		Transport: newTransport(config),
		Director: func(req *http.Request) {
			req.Host = url.Host
			req.URL.Scheme = url.Scheme
			req.URL.Host = url.Host
			req.Header.Set("Host", url.Host)
			req.Header.Del("Authorization")
			req.Header.Del("Cookie")
			for _, header := range headers {
				req.Header.Set(header.Name, header.Value)
			}
		},
	}
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

// RegisterExtensions will retrieve all extensions configurations
// and update the extension registry.
func (m *Manager) RegisterExtensions() error {
	settings, err := m.settings.Get()
	if err != nil {
		return fmt.Errorf("error getting settings: %w", err)
	}
	if settings.ExtensionConfig == "" {
		m.log.Infof("No extensions configured.")
		return nil
	}
	err = m.UpdateExtensionRegistry(settings)
	if err != nil {
		return fmt.Errorf("error updating extension registry: %w", err)
	}
	return nil
}

// UpdateExtensionRegistry will first parse and validate the extensions
// configurations from the given settings. If no errors are found, it will
// iterate over the given configurations building a new extension registry.
// At the end, it will update the manager with the newly created registry.
func (m *Manager) UpdateExtensionRegistry(s *settings.ArgoCDSettings) error {
	extConfigs, err := parseAndValidateConfig(s)
	if err != nil {
		return fmt.Errorf("error parsing extension config: %w", err)
	}
	extReg := make(map[string]ProxyRegistry)
	for _, ext := range extConfigs.Extensions {
		proxyReg := NewProxyRegistry()
		singleBackend := len(ext.Backend.Services) == 1
		for _, service := range ext.Backend.Services {
			proxy, err := NewProxy(service.URL, service.Headers, ext.Backend.ProxyConfig)
			if err != nil {
				return fmt.Errorf("error creating proxy: %w", err)
			}
			err = appendProxy(proxyReg, ext.Name, service, proxy, singleBackend)
			if err != nil {
				return fmt.Errorf("error appending proxy: %w", err)
			}
		}
		extReg[ext.Name] = proxyReg
	}
	m.registry = extReg
	return nil
}

// appendProxy will append the given proxy in the given registry. Will use
// the provided extName and service to determine the map key. The key must
// be unique in the map. If the map already has the key and error is returned.
func appendProxy(registry ProxyRegistry,
	extName string,
	service ServiceConfig,
	proxy *httputil.ReverseProxy,
	singleBackend bool,
) error {
	if singleBackend {
		key := proxyKey(extName, "", "")
		if _, exist := registry[key]; exist {
			return fmt.Errorf("duplicated proxy configuration found for extension key %q", key)
		}
		registry[key] = proxy
		return nil
	}

	// This is the case where there are more than one backend configured
	// for this extension. In this case we need to add the provided cluster
	// configurations for proper correlation to find which proxy to use
	// while handling requests.
	if service.Cluster.Name != "" {
		key := proxyKey(extName, service.Cluster.Name, "")
		if _, exist := registry[key]; exist {
			return fmt.Errorf("duplicated proxy configuration found for extension key %q", key)
		}
		registry[key] = proxy
	}
	if service.Cluster.Server != "" {
		key := proxyKey(extName, "", service.Cluster.Server)
		if _, exist := registry[key]; exist {
			return fmt.Errorf("duplicated proxy configuration found for extension key %q", key)
		}
		registry[key] = proxy
	}
	return nil
}

// authorize will enforce rbac rules are satified for the given RequestResources.
// The following validations are executed:
//   - enforce the subject has permission to read application/project provided
//     in HeaderArgoCDApplicationName and HeaderArgoCDProjectName.
//   - enforce the subject has permission to invoke the extension identified by
//     extName.
//   - enforce that the project has permission to access the destination cluster.
//
// If all validations are satified it will return the Application resource
func (m *Manager) authorize(ctx context.Context, rr *RequestResources, extName string) (*v1alpha1.Application, error) {
	if m.rbac == nil {
		return nil, fmt.Errorf("rbac enforcer not set in extension manager")
	}
	appRBACName := security.RBACName(rr.ApplicationNamespace, rr.ProjectName, rr.ApplicationNamespace, rr.ApplicationName)
	if err := m.rbac.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceApplications, rbacpolicy.ActionGet, appRBACName); err != nil {
		return nil, fmt.Errorf("application authorization error: %w", err)
	}

	if err := m.rbac.EnforceErr(ctx.Value("claims"), rbacpolicy.ResourceExtensions, rbacpolicy.ActionInvoke, extName); err != nil {
		return nil, fmt.Errorf("unauthorized to invoke extension %q: %w", extName, err)
	}

	// just retrieve the app after checking if subject has access to it
	app, err := m.application.Get(rr.ApplicationNamespace, rr.ApplicationName)
	if err != nil {
		return nil, fmt.Errorf("error getting application: %w", err)
	}
	if app == nil {
		return nil, fmt.Errorf("invalid Application provided in the %q header", HeaderArgoCDApplicationName)
	}

	if app.Spec.GetProject() != rr.ProjectName {
		return nil, fmt.Errorf("project mismatch provided in the %q header", HeaderArgoCDProjectName)
	}

	proj, err := m.project.Get(app.Spec.GetProject())
	if err != nil {
		return nil, fmt.Errorf("error getting project: %w", err)
	}
	if proj == nil {
		return nil, fmt.Errorf("invalid project provided in the %q header", HeaderArgoCDProjectName)
	}
	permitted, err := proj.IsDestinationPermitted(app.Spec.Destination, m.project.GetClusters)
	if err != nil {
		return nil, fmt.Errorf("error validating project destinations: %w", err)
	}
	if !permitted {
		return nil, fmt.Errorf("the provided project is not allowed to access the cluster configured in the Application destination")
	}

	return app, nil
}

// findProxy will search the given registry to find the correct proxy to use
// based on the given extName and dest.
func findProxy(registry ProxyRegistry, extName string, dest v1alpha1.ApplicationDestination) (*httputil.ReverseProxy, error) {
	// First try to find the proxy in the registry just by the extension name.
	// This is the simple case for extensions with only one backend service.
	key := proxyKey(extName, "", "")
	if proxy, found := registry[key]; found {
		return proxy, nil
	}

	// If extension has multiple backend services configured, the correct proxy
	// needs to be searched by the ApplicationDestination.
	key = proxyKey(extName, dest.Name, dest.Server)
	if proxy, found := registry[key]; found {
		return proxy, nil
	}

	return nil, fmt.Errorf("no proxy found for extension %q", extName)
}

// ProxyRegistry returns the proxy registry associated for the given
// extension name.
func (m *Manager) ProxyRegistry(name string) (ProxyRegistry, bool) {
	pReg, found := m.registry[name]
	return pReg, found
}

// CallExtension returns a handler func responsible for forwarding requests to the
// extension service. The request will be sanitized by removing sensitive headers.
func (m *Manager) CallExtension() func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		segments := strings.Split(strings.TrimPrefix(r.URL.Path, "/"), "/")
		if segments[0] != "extensions" {
			http.Error(w, fmt.Sprintf("Invalid URL: first segment must be %s", URLPrefix), http.StatusBadRequest)
			return
		}
		extName := segments[1]
		if extName == "" {
			http.Error(w, "Invalid URL: extension name must be provided", http.StatusBadRequest)
			return
		}
		extName = strings.ReplaceAll(extName, "\n", "")
		extName = strings.ReplaceAll(extName, "\r", "")
		reqResources, err := ValidateHeaders(r)
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid headers: %s", err), http.StatusBadRequest)
			return
		}
		app, err := m.authorize(r.Context(), reqResources, extName)
		if err != nil {
			m.log.Infof("unauthorized extension request: %s", err)
			http.Error(w, "Unauthorized extension request", http.StatusUnauthorized)
			return
		}

		proxyRegistry, ok := m.ProxyRegistry(extName)
		if !ok {
			m.log.Warnf("proxy extension warning: attempt to call unregistered extension: %s", extName)
			http.Error(w, "Extension not found", http.StatusNotFound)
			return
		}
		proxy, err := findProxy(proxyRegistry, extName, app.Spec.Destination)
		if err != nil {
			m.log.Errorf("findProxy error: %s", err)
			http.Error(w, "invalid extension", http.StatusBadRequest)
			return
		}

		user := m.userGetter.GetUser(r.Context())
		groups := m.userGetter.GetGroups(r.Context())
		prepareRequest(r, extName, app, user, groups)
		m.log.Debugf("proxing request for extension %q", extName)
		// httpsnoop package is used to properly wrap the responseWriter
		// and avoid optional intefaces issue:
		// https://github.com/felixge/httpsnoop#why-this-package-exists
		// CaptureMetrics will call the proxy and return the metrics from it.
		metrics := httpsnoop.CaptureMetrics(proxy, w, r)

		go registerMetrics(extName, metrics, m.metricsReg)
	}
}

func registerMetrics(extName string, metrics httpsnoop.Metrics, extensionMetricsRegistry ExtensionMetricsRegistry) {
	if extensionMetricsRegistry != nil {
		extensionMetricsRegistry.IncExtensionRequestCounter(extName, metrics.Code)
		extensionMetricsRegistry.ObserveExtensionRequestDuration(extName, metrics.Duration)
	}
}

// prepareRequest is responsible for cleaning the incoming request URL removing
// the Argo CD extension API section from it. It provides additional information to
// the backend service appending them in the outgoing request headers. The appended
// headers are:
//   - Cluster destination name
//   - Cluster destination server
//   - Argo CD authenticated username
func prepareRequest(r *http.Request, extName string, app *v1alpha1.Application, username string, groups []string) {
	r.URL.Path = strings.TrimPrefix(r.URL.Path, fmt.Sprintf("%s/%s", URLPrefix, extName))
	if app.Spec.Destination.Name != "" {
		r.Header.Set(HeaderArgoCDTargetClusterName, app.Spec.Destination.Name)
	}
	if app.Spec.Destination.Server != "" {
		r.Header.Set(HeaderArgoCDTargetClusterURL, app.Spec.Destination.Server)
	}
	if username != "" {
		r.Header.Set(HeaderArgoCDUsername, username)
	}
	if len(groups) > 0 {
		r.Header.Set(HeaderArgoCDGroups, strings.Join(groups, ","))
	}
}

// AddMetricsRegistry will associate the given metricsReg in the Manager.
func (m *Manager) AddMetricsRegistry(metricsReg ExtensionMetricsRegistry) {
	m.metricsReg = metricsReg
}
