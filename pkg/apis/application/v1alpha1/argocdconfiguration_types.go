package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Validation notes:
// - Prefer OpenAPI Pattern / items:Pattern for regexes on lists (cheap; safe for unbounded lists).
// - Avoid CEL matches() / self.all(...matches...) for regex — list CEL has high cost and can hit the budget.
// - Prefer OpenAPI Pattern for URL fields too: CEL isURL()/url() is expensive and can blow the
//   per-schema cost budget on large CRDs even with MaxLength set.
// - Kind/APIGroup Patterns allow '*' where Argo CD wildcards are used.
// - *Template fields (Go templates) are not validated as plain URLs/literals.

// ArgoCDConfiguration is the Schema for the argocdconfigurations API.
// It is a pure data store (spec only; no status subresource).
//
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:path=argocdconfigurations,scope=Namespaced,shortName=argocdconfig
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'argocd-config'",message="ArgoCDConfiguration must be named 'argocd-config'"
type ArgoCDConfiguration struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Spec holds Argo CD configuration values that replace or override
	// argocd-cm, argocd-cmd-params-cm, and argocd-rbac-cm.
	Spec ArgoCDConfigurationSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
}

// ArgoCDConfigurationList contains a list of ArgoCDConfiguration.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
type ArgoCDConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of ArgoCDConfiguration objects.
	Items []ArgoCDConfiguration `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ArgoCDConfigurationSpec holds Argo CD configuration.
//
// Fields remain optional so unset (absent) is distinguishable from empty and
// gradual CRD adoption is possible. Where set, values are validated maximally
// in the CRD (CEL / OpenAPI) rather than only in application code.
//
// Naming conventions:
//   - *URL — absolute URL; CEL-validated (usually http/https)
//   - *Glob — value may contain glob wildcards (*, ?)
//   - *Regex — regular expression string (Go RE2 / regexp); distinguished from Glob
//   - *Expr — expression language string (e.g. deep-link "if" condition)
//   - *Template — Go text/template (or similar); not validated as a plain URL
//   - *Lua — string whose value is a Lua script (not a YAML/document wrapper)
//   - *Size — byte/payload size as resource.Quantity (not unit-suffixed ints)
//   - tlsEnabled — whether TLS is used (server or client); not "insecure"/"plaintext"
//   - insecureSkipVerify — skip cert verification; not inverted "strictTLS"
//   - *Enabled — past-tense suffix for feature toggles (e.g. tlsEnabled, profileEnabled);
//     not enable*/disable* prefixes and not *Disabled double-negatives.
//     Bare "enabled" OK on a feature settings object. Invert mapping when the legacy key
//     uses the opposite polarity (disable.*, *.insecure, *.plaintext).
//   - *ParallelismLimit — concurrency caps (bare parallelismLimit when the parent is the
//     limited component; otherwise subjectParallelismLimit). Not concurrent*Max.
//
// Secrets that Argo owns end-to-end use SecretKeySelector. $string interpolation
// is reserved for opaque/external structures (Dex connector config) or intentional
// partial string insertion.
type ArgoCDConfigurationSpec struct {
	// Server holds API server, UI, and auth-facing settings (argocd-server).
	// Migration: component group; no legacy key — see child fields.
	// +optional
	Server *ServerConfig `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Controller holds application-controller settings and application/resource
	// product configuration historically stored in argocd-cm.
	// Migration: component group; no legacy key — see child fields.
	// +optional
	Controller *ControllerConfig `json:"controller,omitempty" protobuf:"bytes,2,opt,name=controller"`
	// RepoServer holds repo-server address, client TLS, and manifest-tool settings.
	// Migration: component group; no legacy key — see child fields.
	// +optional
	RepoServer *RepoServerConfig `json:"repoServer,omitempty" protobuf:"bytes,3,opt,name=repoServer"`
	// CommitServer holds commit-server address and hydrator commit identity.
	// Migration: component group; no legacy key — see child fields.
	// +optional
	CommitServer *CommitServerConfig `json:"commitServer,omitempty" protobuf:"bytes,4,opt,name=commitServer"`
	// ApplicationSet holds ApplicationSet controller settings.
	// Migration: component group; no legacy key — see child fields.
	// +optional
	ApplicationSet *ApplicationSetConfig `json:"applicationSet,omitempty" protobuf:"bytes,5,opt,name=applicationSet"`
	// Cluster holds cluster-registration policy (argocd-cm: cluster.*).
	// Migration: component group; no legacy key — see child fields.
	// +optional
	Cluster *ClusterRegistrationConfig `json:"cluster,omitempty" protobuf:"bytes,6,opt,name=cluster"`
	// Redis holds shared Redis connection settings used by multiple components.
	// Migration: component group; no legacy key — see child fields.
	// +optional
	Redis *RedisConfig `json:"redis,omitempty" protobuf:"bytes,7,opt,name=redis"`
	// OTLP holds OpenTelemetry collector settings shared across components.
	// Migration: component group; no legacy key — see child fields.
	// +optional
	OTLP *OTLPConfig `json:"otlp,omitempty" protobuf:"bytes,8,opt,name=otlp"`
	// ApplicationNamespaceGlobs lists additional namespaces where Applications may
	// be created and reconciled (cmd-params: application.namespaces). Supports
	// globs such as "team-*". The Argo CD install namespace is always allowed.
	// Migration: if present, takes precedence over argocd-cmd-params-cm application.namespaces; replaces the whole collection.
	// +optional
	ApplicationNamespaceGlobs []string `json:"applicationNamespaceGlobs,omitempty" protobuf:"bytes,9,rep,name=applicationNamespaceGlobs"`
	// InstallationID uniquely identifies this Argo CD installation for resource
	// tracking (argocd-cm: installationID). Empty disables the feature.
	// Migration: if set, takes precedence over argocd-cm installationID.
	// +optional
	InstallationID string `json:"installationID,omitempty" protobuf:"bytes,10,opt,name=installationID"`
	// ExecTimeout is the shared timeout for forked commands (git, helm, kustomize,
	// CMP, GPG, …) via util/exec (env: ARGOCD_EXEC_TIMEOUT; default 90s).
	// This is not the UI pod-terminal feature under server.exec.
	// Migration: if set, takes precedence over env ARGOCD_EXEC_TIMEOUT. Does not round-trip through ConfigMaps.
	// +optional
	ExecTimeout *metav1.Duration `json:"execTimeout,omitempty" protobuf:"bytes,11,opt,name=execTimeout"`
	// DexServer holds Dex server process runtime settings (cmd-params: dexserver.*).
	// Migration: component group; no legacy key — see child fields.
	// +optional
	DexServer *DexServerConfig `json:"dexServer,omitempty" protobuf:"bytes,12,opt,name=dexServer"`
	// Notifications holds notifications-controller settings
	// (cmd-params: notificationscontroller.*).
	// Migration: component group; no legacy key — see child fields.
	// +optional
	Notifications *NotificationsConfig `json:"notifications,omitempty" protobuf:"bytes,13,opt,name=notifications"`
	// Logging holds cross-component logging options (e.g. shared timestamp format).
	// Migration: component group; no legacy key — see child fields.
	// +optional
	Logging *LoggingConfig `json:"logging,omitempty" protobuf:"bytes,14,opt,name=logging"`
}

// ServerConfig holds API server, UI, and auth-facing settings.
type ServerConfig struct {
	// URLs are externally facing base URLs for this Argo CD instance
	// (argocd-cm: url + additionalUrls). The first entry is the primary URL used for
	// SSO redirects (argocd-cm: url); any further entries are additional SSO-capable
	// base URLs (argocd-cm: additionalUrls).
	// Migration: if present, takes precedence over argocd-cm url and additionalUrls as a pair; replaces both.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=32
	// +kubebuilder:validation:items:MaxLength=2048
	// +kubebuilder:validation:items:Pattern=`^https?://.+`
	URLs []string `json:"urls,omitempty" protobuf:"bytes,1,rep,name=urls"`

	// TLSEnabled controls whether the API server serves TLS (cmd-params: server.insecure —
	// inverted: insecure=true means tlsEnabled=false). TLS is on by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.insecure (inverted).
	// +optional
	TLSEnabled *bool `json:"tlsEnabled,omitempty" protobuf:"varint,2,opt,name=tlsEnabled"`
	// Log holds API server log format and level (cmd-params: server.log.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm server.log.* as a group.
	// +optional
	Log *LogConfig `json:"log,omitempty" protobuf:"bytes,3,opt,name=log"`
	// BaseHref is the path Argo CD is hosted under with a reverse proxy that
	// cannot strip prefixes (cmd-params: server.basehref). Example: "/argo-cd".
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.basehref.
	// +optional
	BaseHref string `json:"baseHref,omitempty" protobuf:"bytes,4,opt,name=baseHref"`
	// RootPath is the path Argo CD is hosted under when the reverse proxy strips
	// the prefix (cmd-params: server.rootpath). Example: "/argo-cd".
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.rootpath.
	// +optional
	RootPath string `json:"rootPath,omitempty" protobuf:"bytes,5,opt,name=rootPath"`
	// StaticAssetsPath is the directory of UI static assets
	// (cmd-params: server.staticassets).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.staticassets.
	// +optional
	StaticAssetsPath string `json:"staticAssetsPath,omitempty" protobuf:"bytes,6,opt,name=staticAssetsPath"`
	// Listen holds API server listen host/port and metrics listen host/port
	// (cmd-params: server.listen.address, server.metrics.listen.address;
	// flags: --port, --metrics-port).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Listen *ServerListenConfig `json:"listen,omitempty" protobuf:"bytes,7,opt,name=listen"`
	// AuthEnabled controls API authentication (cmd-params: server.disable.auth —
	// inverted: disable.auth=true means authEnabled=false). Auth is on by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.disable.auth (inverted).
	// +optional
	AuthEnabled *bool `json:"authEnabled,omitempty" protobuf:"varint,8,opt,name=authEnabled"`
	// Compression is the HTTP response compression algorithm
	// (cmd-params: server.enable.gzip): "disabled" or "gzip". Gzip is on by default in Argo CD.
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.enable.gzip
	// (true maps to gzip, false to disabled).
	// +optional
	// +kubebuilder:validation:Enum=disabled;gzip
	Compression string `json:"compression,omitempty" protobuf:"bytes,9,opt,name=compression"`
	// ProxyExtensionEnabled enables UI proxy extensions
	// (cmd-params: server.enable.proxy.extension).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.enable.proxy.extension.
	// +optional
	ProxyExtensionEnabled *bool `json:"proxyExtensionEnabled,omitempty" protobuf:"varint,10,opt,name=proxyExtensionEnabled"`
	// SyncReplaceAllowed allows clients to request sync with replace
	// (cmd-params: server.sync.replace.allowed).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.sync.replace.allowed.
	// +optional
	SyncReplaceAllowed *bool `json:"syncReplaceAllowed,omitempty" protobuf:"varint,11,opt,name=syncReplaceAllowed"`
	// XFrameOptions sets the X-Frame-Options HTTP header
	// (cmd-params: server.x.frame.options).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.x.frame.options.
	// +optional
	XFrameOptions string `json:"xFrameOptions,omitempty" protobuf:"bytes,12,opt,name=xFrameOptions"`
	// APIContentTypes is the Content-Type allowlist for API requests
	// (cmd-params: server.api.content.types).
	// Migration: if present, takes precedence over argocd-cmd-params-cm server.api.content.types; replaces the whole collection.
	// +optional
	APIContentTypes []string `json:"apiContentTypes,omitempty" protobuf:"bytes,13,rep,name=apiContentTypes"`
	// ContentSecurityPolicy sets the Content-Security-Policy HTTP header
	// (cmd-params: server.content.security.policy).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.content.security.policy.
	// +optional
	ContentSecurityPolicy string `json:"contentSecurityPolicy,omitempty" protobuf:"bytes,14,opt,name=contentSecurityPolicy"`
	// HTTPCookieMaxNumber caps the number of cookies the API server will accept
	// (cmd-params: server.http.cookie.maxnumber).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.http.cookie.maxnumber.
	// +optional
	// +kubebuilder:validation:Minimum=0
	HTTPCookieMaxNumber *int32 `json:"httpCookieMaxNumber,omitempty" protobuf:"varint,15,opt,name=httpCookieMaxNumber"`
	// ApplicationTreeShardSize is the max number of resources stored in one
	// application-tree cache shard (env: ARGOCD_APPLICATION_TREE_SHARD_SIZE).
	// Migration: if set, takes precedence over env ARGOCD_APPLICATION_TREE_SHARD_SIZE. Does not round-trip through ConfigMaps.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=1000
	ApplicationTreeShardSize *int32 `json:"applicationTreeShardSize,omitempty" protobuf:"varint,16,opt,name=applicationTreeShardSize"`
	// ProfileEnabled enables pprof profiling endpoints (cmd-params: server.profile.enabled).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.profile.enabled.
	// +optional
	ProfileEnabled *bool `json:"profileEnabled,omitempty" protobuf:"varint,17,opt,name=profileEnabled"`
	// GRPCTXTServiceConfigEnabled enables gRPC TXT service config
	// (cmd-params: server.grpc.enable.txt.service.config).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.grpc.enable.txt.service.config.
	// +optional
	GRPCTXTServiceConfigEnabled *bool `json:"grpcTXTServiceConfigEnabled,omitempty" protobuf:"varint,18,opt,name=grpcTXTServiceConfigEnabled"`
	// DexConnection holds how the API server connects to Dex
	// (cmd-params: server.dex.server*). Named distinctly from spec.dexServer
	// (Dex process runtime) to avoid overloaded keys.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm server.dex.server* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	DexConnection *DexConnectionConfig `json:"dexConnection,omitempty" protobuf:"bytes,19,opt,name=dexConnection"`
	// Cache holds API server cache TTLs and sizes (cmd-params: server.*.cache.*).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Cache *ServerCacheConfig `json:"cache,omitempty" protobuf:"bytes,20,opt,name=cache"`
	// TLS holds API server TLS cipher/version settings (cmd-params: server.tls.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm server.tls.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	TLS *TLSVersionConfig `json:"tls,omitempty" protobuf:"bytes,21,opt,name=tls"`
	// K8sClient holds Kubernetes client tuning for the API server
	// (cmd-params: server.k8s.* / server.k8sclient.*).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	K8sClient *K8sClientConfig `json:"k8sClient,omitempty" protobuf:"bytes,22,opt,name=k8sClient"`
	// Logs holds UI pod-log viewing settings (argocd-cm: server.maxPodLogsToRender).
	// Migration: if non-nil, takes precedence over argocd-cm server.maxPodLogsToRender as a group.
	// +optional
	Logs *ServerLogsConfig `json:"logs,omitempty" protobuf:"bytes,23,opt,name=logs"`
	// Dex replaces dex.config wholesale when non-nil.
	// Migration: if non-nil, takes precedence over argocd-cm dex.config as a whole, including all child fields.
	// +optional
	Dex *DexConfig `json:"dex,omitempty" protobuf:"bytes,24,opt,name=dex"`
	// OIDC replaces oidc.config wholesale when non-nil (direct OIDC, alternative to Dex).
	// Migration: if non-nil, takes precedence over argocd-cm oidc.config as a whole, including all child fields.
	// Note: InsecureSkipVerify maps to a separate argocd-cm key (oidc.tls.insecure.skip.verify),
	// not the oidc.config blob — it is nested here for discoverability.
	// +optional
	OIDC *OIDCConfig `json:"oidc,omitempty" protobuf:"bytes,25,opt,name=oidc"`
	// RBAC holds authorization policy from argocd-rbac-cm.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	RBAC *RBACConfig `json:"rbac,omitempty" protobuf:"bytes,26,opt,name=rbac"`
	// UI holds web UI customization (banner, CSS, login button text).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	UI *UIConfig `json:"ui,omitempty" protobuf:"bytes,27,opt,name=ui"`
	// Accounts configures local (non-SSO) user accounts and their capabilities.
	// Migration: if present, takes precedence over argocd-cm accounts.* / admin.enabled; replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=name
	Accounts []AccountConfig `json:"accounts,omitempty" protobuf:"bytes,28,rep,name=accounts"`
	// Extensions configures UI proxy extensions (argocd-cm: extension.config).
	// Migration: if present, takes precedence over argocd-cm extension.config; replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=name
	Extensions []ExtensionConfig `json:"extensions,omitempty" protobuf:"bytes,29,rep,name=extensions"`
	// DeepLinks configures custom deep links shown in application, project, and
	// resource views (argocd-cm: application.links / project.links / resource.links).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	DeepLinks *DeepLinksConfig `json:"deepLinks,omitempty" protobuf:"bytes,30,opt,name=deepLinks"`
	// Help configures the UI help/chat panel and CLI download links.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Help *HelpConfig `json:"help,omitempty" protobuf:"bytes,31,opt,name=help"`
	// Users holds anonymous access, session duration, and password policy.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Users *UsersConfig `json:"users,omitempty" protobuf:"bytes,32,opt,name=users"`
	// GoogleAnalytics holds optional Google Analytics tracking settings.
	// Migration: if non-nil, takes precedence over argocd-cm ga.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	GoogleAnalytics *GoogleAnalyticsConfig `json:"googleAnalytics,omitempty" protobuf:"bytes,33,opt,name=googleAnalytics"`
	// Webhook holds SCM webhook payload, jitter, and server webhook worker settings.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Webhook *WebhookConfig `json:"webhook,omitempty" protobuf:"bytes,34,opt,name=webhook"`
	// Exec configures the UI pod terminal (exec) feature.
	// Migration: if non-nil, takes precedence over argocd-cm exec.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	Exec *ExecConfig `json:"exec,omitempty" protobuf:"bytes,35,opt,name=exec"`
	// StatusBadge configures application/project status badge URLs.
	// Migration: if non-nil, takes precedence over argocd-cm statusbadge.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	StatusBadge *StatusBadgeConfig `json:"statusBadge,omitempty" protobuf:"bytes,36,opt,name=statusBadge"`
}

// --- Dex ---

// DexConfig is the Dex SSO configuration (argocd-cm: dex.config).
// Connectors use a typed envelope; per-connector config stays opaque and may
// contain $string secret interpolation (Dex-owned schema).
type DexConfig struct {
	// Connectors is the list of Dex identity connectors (GitHub, OIDC, SAML, LDAP, …).
	// See https://dexidp.io/docs/connectors/. Migration: composite child of dex.config.
	// +optional
	// +listType=map
	// +listMapKey=id
	Connectors []DexConnector `json:"connectors,omitempty" protobuf:"bytes,1,rep,name=connectors"`
	// StaticClients are additional Dex static OAuth clients for reuse with other
	// services (e.g. Argo Workflows). Argo CD also injects its own static clients
	// when Dex is enabled. Migration: composite child of dex.config (staticClients).
	// +optional
	// +listType=atomic
	// +kubebuilder:pruning:PreserveUnknownFields
	StaticClients []runtime.RawExtension `json:"staticClients,omitempty" protobuf:"bytes,2,rep,name=staticClients"`
	// Extra holds any additional top-level Dex config keys not modeled above
	// (e.g. issuer, storage). Opaque; preserved as-is from dex.config.
	// Migration: composite child of dex.config (unmodeled root keys).
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Extra *runtime.RawExtension `json:"extra,omitempty" protobuf:"bytes,3,opt,name=extra"`
}

// DexConnector is one Dex identity connector entry (dex.config connectors[]).
type DexConnector struct {
	// Type is the Dex connector type (e.g. "github", "oidc", "saml", "ldap").
	// Migration: composite child of dex.config (connectors[].type).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// ID is the connector's unique identifier within Dex (login routing / internal use).
	// Migration: composite child of dex.config (connectors[].id).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id" protobuf:"bytes,2,opt,name=id"`
	// Name is the human-readable connector name shown on the login page.
	// Migration: composite child of dex.config (connectors[].name).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name" protobuf:"bytes,3,opt,name=name"`
	// Config is opaque Dex connector configuration (client IDs, org filters, etc.).
	// $string / $secret:key refs are allowed. Schema is owned by Dex, not Argo CD.
	// When set from Go, Raw must be JSON bytes.
	// Migration: composite child of dex.config (connectors[].config).
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Config runtime.RawExtension `json:"config,omitempty" protobuf:"bytes,4,opt,name=config"`
}

// --- OIDC ---

// OIDCConfig is direct OIDC SSO configuration (argocd-cm: oidc.config).
// When non-nil it replaces oidc.config wholesale.
// See docs/operator-manual/user-management/ in argo-cd.
type OIDCConfig struct {
	// Name is the provider display name shown on the SSO login button
	// (oidc.config name). Migration: composite child of oidc.config.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// IssuerURL is the OIDC issuer URL that must expose
	// .well-known/openid-configuration (oidc.config issuer).
	// Migration: composite child of oidc.config.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^https?://.+`
	IssuerURL string `json:"issuerURL,omitempty" protobuf:"bytes,2,opt,name=issuerURL"`
	// ClientID is the OAuth 2.0 client ID registered with the IdP for the Argo CD
	// server callback (/auth/callback) (oidc.config clientID).
	// Migration: composite child of oidc.config.
	// +optional
	ClientID string `json:"clientID,omitempty" protobuf:"bytes,3,opt,name=clientID"`
	// ClientSecretRef references the OIDC client secret in a Kubernetes Secret.
	// Raw secrets and $string refs are not accepted here — use a SecretKeySelector.
	// Legacy oidc.config clientSecret ($secret:key in argocd-cm) must become an
	// explicit secret ref. Migration: composite child of oidc.config.
	// +optional
	ClientSecretRef *corev1.SecretKeySelector `json:"clientSecretRef,omitempty" protobuf:"bytes,4,opt,name=clientSecretRef"`
	// CLIClientID is a separate OAuth client ID for the Argo CD CLI (localhost
	// callback). When omitted, the CLI reuses ClientID (oidc.config cliClientID).
	// Migration: composite child of oidc.config.
	// +optional
	CLIClientID string `json:"cliClientID,omitempty" protobuf:"bytes,5,opt,name=cliClientID"`
	// UserInfo holds UserInfo endpoint settings for group membership lookup when
	// groups are absent from the ID token. Migration: composite child of oidc.config.
	// +optional
	UserInfo *OIDCUserInfoConfig `json:"userInfo,omitempty" protobuf:"bytes,6,opt,name=userInfo"`
	// RequestedScopes are OIDC scopes requested at login (oidc.config requestedScopes).
	// When omitted, Argo CD defaults to ["openid", "profile", "email", "groups"].
	// Migration: composite child of oidc.config.
	// +optional
	RequestedScopes []string `json:"requestedScopes,omitempty" protobuf:"bytes,7,rep,name=requestedScopes"`
	// RequestedIDTokenClaims requests additional claims via the OIDC Claims
	// Parameter on the ID token (e.g. groups) (oidc.config requestedIDTokenClaims).
	// Migration: composite child of oidc.config.
	// +optional
	RequestedIDTokenClaims map[string]OIDCClaim `json:"requestedIDTokenClaims,omitempty" protobuf:"bytes,8,rep,name=requestedIDTokenClaims"`
	// LogoutURL is an optional IdP logout URL. May include {{token}} and
	// {{logoutRedirectURL}} placeholders (oidc.config logoutURL).
	// Migration: composite child of oidc.config.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^[a-zA-Z][a-zA-Z0-9+.-]*:.+`
	LogoutURL string `json:"logoutURL,omitempty" protobuf:"bytes,9,opt,name=logoutURL"`
	// RootCA is a PEM-encoded root CA used to verify the OIDC provider's TLS
	// certificate when it is not signed by a well-known CA (oidc.config rootCA).
	// Migration: composite child of oidc.config.
	// +optional
	RootCA string `json:"rootCA,omitempty" protobuf:"bytes,10,opt,name=rootCA"`
	// PKCEAuthenticationEnabled enables PKCE for the authorization code flow
	// (oidc.config enablePKCEAuthentication). Default false.
	// Migration: composite child of oidc.config.
	// +optional
	PKCEAuthenticationEnabled bool `json:"pkceAuthenticationEnabled,omitempty" protobuf:"varint,11,opt,name=pkceAuthenticationEnabled"`
	// DomainHint is sent to the IdP as the domain_hint authorization parameter
	// (common with Microsoft Entra ID) (oidc.config domainHint). Empty = not sent.
	// Migration: composite child of oidc.config.
	// +optional
	DomainHint string `json:"domainHint,omitempty" protobuf:"bytes,12,opt,name=domainHint"`
	// Azure holds Azure AD / Entra ID–specific OIDC options (workload identity,
	// group overage). Migration: composite child of oidc.config (azure).
	// +optional
	Azure *AzureOIDCConfig `json:"azure,omitempty" protobuf:"bytes,13,opt,name=azure"`
	// RefreshTokenThreshold refreshes the ID token this long before expiry using
	// the refresh token (oidc.config refreshTokenThreshold). Default 0s refreshes
	// only after expiry; must be shorter than token lifetime.
	// Migration: composite child of oidc.config.
	// +optional
	RefreshTokenThreshold *metav1.Duration `json:"refreshTokenThreshold,omitempty" protobuf:"bytes,14,opt,name=refreshTokenThreshold"`
	// AllowedAudiences are accepted JWT aud values for tokens presented to Argo CD
	// (oidc.config allowedAudiences). When omitted/empty, defaults to
	// [clientID, cliClientID].
	// Migration: composite child of oidc.config.
	// +optional
	AllowedAudiences []string `json:"allowedAudiences,omitempty" protobuf:"bytes,15,rep,name=allowedAudiences"`
	// SkipAudienceCheckWhenTokenHasNoAudience accepts tokens that omit the aud
	// claim when true (oidc.config skipAudienceCheckWhenTokenHasNoAudience).
	// Default false for direct OIDC (Argo CD ≥ 2.6).
	// Migration: composite child of oidc.config.
	// +optional
	SkipAudienceCheckWhenTokenHasNoAudience bool `json:"skipAudienceCheckWhenTokenHasNoAudience,omitempty" protobuf:"varint,16,opt,name=skipAudienceCheckWhenTokenHasNoAudience"`
	// InsecureSkipVerify skips TLS certificate verification when talking to the
	// OIDC provider or bundled Dex (argocd-cm: oidc.tls.insecure.skip.verify).
	// This is a separate legacy key from oidc.config; nested under oidc for
	// discoverability. Pointer so it can migrate independently of the composite blob.
	// Default false. Migration: if set, takes precedence over argocd-cm oidc.tls.insecure.skip.verify.
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty" protobuf:"varint,17,opt,name=insecureSkipVerify"`
}

// OIDCUserInfoConfig holds UserInfo endpoint settings for OIDC group lookup.
type OIDCUserInfoConfig struct {
	// GroupsEnabled fetches group membership from the UserInfo endpoint when groups
	// are absent from the ID token (oidc.config enableUserInfoGroups). Default false.
	// Migration: composite child of oidc.config.
	// +optional
	GroupsEnabled bool `json:"groupsEnabled,omitempty" protobuf:"varint,1,opt,name=groupsEnabled"`
	// BaseURL overrides the UserInfo endpoint base URL when it differs from the issuer
	// (oidc.config userInfoBaseURL). Empty uses the issuer URL.
	// Migration: composite child of oidc.config.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^https?://.+`
	BaseURL string `json:"baseURL,omitempty" protobuf:"bytes,2,opt,name=baseURL"`
	// Path is the UserInfo path appended to BaseURL or the issuer URL
	// (oidc.config userInfoPath). Must be set (with GroupsEnabled) to enable
	// UserInfo lookup; there is no hardcoded default in Argo CD.
	// Migration: composite child of oidc.config.
	// +optional
	Path string `json:"path,omitempty" protobuf:"bytes,3,opt,name=path"`
	// CacheExpiration is how long cached UserInfo group results are kept
	// (oidc.config userInfoCacheExpiration). Zero falls back to ID token lifetime.
	// Migration: composite child of oidc.config.
	// +optional
	CacheExpiration *metav1.Duration `json:"cacheExpiration,omitempty" protobuf:"bytes,4,opt,name=cacheExpiration"`
}

// OIDCClaim describes a requested ID token claim (OIDC Claims Parameter).
type OIDCClaim struct {
	// Essential marks the claim as essential in the OIDC claims request
	// (oidc.config requestedIDTokenClaims.<claim>.essential).
	// Migration: composite child of oidc.config.
	// +optional
	Essential bool `json:"essential,omitempty" protobuf:"varint,1,opt,name=essential"`
	// Value requests a single specific claim value
	// (oidc.config requestedIDTokenClaims.<claim>.value).
	// Migration: composite child of oidc.config.
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// Values requests that the claim match one of several allowed values
	// (oidc.config requestedIDTokenClaims.<claim>.values).
	// Migration: composite child of oidc.config.
	// +optional
	Values []string `json:"values,omitempty" protobuf:"bytes,3,rep,name=values"`
}

// AzureOIDCConfig holds Azure Active Directory / Entra ID–specific OIDC settings.
type AzureOIDCConfig struct {
	// UseWorkloadIdentity authenticates to Azure AD using Kubernetes workload
	// identity instead of a client secret (oidc.config azure.useWorkloadIdentity).
	// Default false. Migration: composite child of oidc.config.
	// +optional
	UseWorkloadIdentity bool `json:"useWorkloadIdentity,omitempty" protobuf:"varint,1,opt,name=useWorkloadIdentity"`
	// UserGroupOverageClaim resolves Azure AD group overage (>200 groups) via
	// Microsoft Graph when the token indicates groups were truncated.
	// Migration: composite child of oidc.config.
	// +optional
	UserGroupOverageClaim *AzureUserGroupOverageClaimConfig `json:"userGroupOverageClaim,omitempty" protobuf:"bytes,2,opt,name=userGroupOverageClaim"`
	// GraphAPIEndpointURL overrides the Microsoft Graph API base URL for sovereign
	// clouds (oidc.config azure.graphApiEndpoint).
	// Default https://graph.microsoft.com/v1.0.
	// Migration: composite child of oidc.config.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^https?://.+`
	GraphAPIEndpointURL string `json:"graphAPIEndpointURL,omitempty" protobuf:"bytes,3,opt,name=graphAPIEndpointURL"`
}

// AzureUserGroupOverageClaimConfig holds Azure AD group overage resolution settings.
type AzureUserGroupOverageClaimConfig struct {
	// Enabled resolves group overage via Microsoft Graph
	// (oidc.config azure.enableUserGroupOverageClaim). Default false.
	// Migration: composite child of oidc.config.
	// +optional
	Enabled bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
	// CacheExpiration is the cache TTL for Graph-resolved group IDs
	// (oidc.config azure.userGroupOverageClaimCacheExpiration).
	// When unset (0), falls back to token expiry.
	// Migration: composite child of oidc.config.
	// +optional
	CacheExpiration *metav1.Duration `json:"cacheExpiration,omitempty" protobuf:"bytes,2,opt,name=cacheExpiration"`
}

// --- RBAC ---

// RBACConfig holds authorization policy (argocd-rbac-cm).
// Organizational subgroup: children migrate independently.
type RBACConfig struct {
	// Default is the default role assigned when no policy matches
	// (argocd-rbac-cm: policy.default), e.g. "role:readonly".
	// Migration: if set, takes precedence over argocd-rbac-cm policy.default.
	// +optional
	Default string `json:"default,omitempty" protobuf:"bytes,1,opt,name=default"`
	// Scopes are OIDC scopes/claims examined for group membership
	// (argocd-rbac-cm: scopes). Default is typically [groups].
	// Migration: if present, takes precedence over argocd-rbac-cm scopes; replaces the whole collection.
	// +optional
	Scopes []string `json:"scopes,omitempty" protobuf:"bytes,2,rep,name=scopes"`
	// MatchMode selects the Casbin matcher: "glob" (default) or "regex"
	// (argocd-rbac-cm: policy.matchMode).
	// Migration: if set, takes precedence over argocd-rbac-cm policy.matchMode.
	// +optional
	// +kubebuilder:validation:Enum=glob;regex
	MatchMode string `json:"matchMode,omitempty" protobuf:"bytes,3,opt,name=matchMode"`
	// PolicyCSV is the main Casbin policy.csv contents (roles and bindings).
	// Migration: if set, takes precedence over argocd-rbac-cm policy.csv.
	// +optional
	PolicyCSV string `json:"policyCSV,omitempty" protobuf:"bytes,4,opt,name=policyCSV"`
	// PolicyOverlays are additional named policy.csv fragments concatenated
	// after the main policy (argocd-rbac-cm: policy.<name>.csv).
	// Migration: if present, takes precedence over argocd-rbac-cm policy.<name>.csv; replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=name
	PolicyOverlays []RBACPolicyOverlay `json:"policyOverlays,omitempty" protobuf:"bytes,5,rep,name=policyOverlays"`
	// ApplicationFineGrainedInheritanceEnabled controls inheriting project roles
	// for fine-grained application RBAC
	// (argocd-cm: server.rbac.disableApplicationFineGrainedRBACInheritance —
	// inverted: disable...=true means enabled=false). Inheritance is on by default.
	// Migration: if set, takes precedence over argocd-cm server.rbac.disableApplicationFineGrainedRBACInheritance (inverted).
	// +optional
	ApplicationFineGrainedInheritanceEnabled *bool `json:"applicationFineGrainedInheritanceEnabled,omitempty" protobuf:"varint,6,opt,name=applicationFineGrainedInheritanceEnabled"`
}

// RBACPolicyOverlay is a named extra policy.csv fragment concatenated after the
// main policy (argocd-rbac-cm: policy.<name>.csv).
type RBACPolicyOverlay struct {
	// Name is the overlay identifier (maps to policy.<name>.csv in argocd-rbac-cm).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// CSV is the Casbin CSV policy content for this overlay (roles, bindings,
	// and resource rules). Maps to argocd-rbac-cm policy.<name>.csv.
	// +kubebuilder:validation:Required
	CSV string `json:"csv" protobuf:"bytes,2,opt,name=csv"`
}

// --- Resource ---

// ResourceConfig holds resource watch filters, customizations, and UI display options.
// Organizational subgroup: children migrate independently.
type ResourceConfig struct {
	// Exclusions lists group/kind/cluster filters for resources excluded from
	// the cluster watch cache (argocd-cm: resource.exclusions).
	// Migration: if present, takes precedence over argocd-cm resource.exclusions; replaces the whole collection.
	// +optional
	// +listType=atomic
	Exclusions []FilteredResource `json:"exclusions,omitempty" protobuf:"bytes,1,rep,name=exclusions"`
	// Inclusions, when set, limits the watch cache to only these group/kind/cluster
	// filters (argocd-cm: resource.inclusions).
	// Migration: if present, takes precedence over argocd-cm resource.inclusions; replaces the whole collection.
	// +optional
	// +listType=atomic
	Inclusions []FilteredResource `json:"inclusions,omitempty" protobuf:"bytes,2,rep,name=inclusions"`
	// Health is per-GVK health Lua overrides (argocd-cm: resource.customizations.health.*
	// and useOpenLibs.*). Listed by concern so you edit health without scanning mixed GVK rows.
	// Migration: if present, takes precedence over argocd-cm resource.customizations.health.*
	// and resource.customizations.useOpenLibs.*; replaces those collections.
	// +optional
	// +listType=map
	// +listMapKey=group
	// +listMapKey=kind
	Health []ResourceHealthCustomization `json:"health,omitempty" protobuf:"bytes,3,rep,name=health"`
	// Actions is per-GVK custom resource actions: discovery script plus named definitions
	// (argocd-cm: resource.customizations.actions.*). Discovery and definitions stay grouped
	// because they form one actions blob per GVK in Argo CD.
	// Migration: if present, takes precedence over argocd-cm resource.customizations.actions.*;
	// replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=group
	// +listMapKey=kind
	Actions []ResourceActionsCustomization `json:"actions,omitempty" protobuf:"bytes,4,rep,name=actions"`
	// IgnoreDifferences is per-GVK sync-diff ignore rules
	// (argocd-cm: resource.customizations.ignoreDifferences.*).
	// Migration: if present, takes precedence over argocd-cm resource.customizations.ignoreDifferences.*;
	// replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=group
	// +listMapKey=kind
	IgnoreDifferences []ResourceIgnoreCustomization `json:"ignoreDifferences,omitempty" protobuf:"bytes,5,rep,name=ignoreDifferences"`
	// IgnoreResourceUpdates is per-GVK rules for fields whose changes should not trigger
	// reconcile (argocd-cm: resource.customizations.ignoreResourceUpdates.*).
	// Migration: if present, takes precedence over argocd-cm resource.customizations.ignoreResourceUpdates.*;
	// replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=group
	// +listMapKey=kind
	IgnoreResourceUpdates []ResourceIgnoreCustomization `json:"ignoreResourceUpdates,omitempty" protobuf:"bytes,6,rep,name=ignoreResourceUpdates"`
	// KnownTypeFields is per-GVK typed-field declarations for structured diff
	// (argocd-cm: resource.customizations.knownTypeFields.*).
	// Migration: if present, takes precedence over argocd-cm resource.customizations.knownTypeFields.*;
	// replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=group
	// +listMapKey=kind
	KnownTypeFields []ResourceKnownTypesCustomization `json:"knownTypeFields,omitempty" protobuf:"bytes,7,rep,name=knownTypeFields"`
	// RespectRBAC limits watched resources to those the controller can list:
	// empty (off), "normal", or "strict" (argocd-cm: resource.respectRBAC).
	// Migration: if set, takes precedence over argocd-cm resource.respectRBAC.
	// +optional
	// +kubebuilder:validation:Enum=;normal;strict
	RespectRBAC string `json:"respectRBAC,omitempty" protobuf:"bytes,8,opt,name=respectRBAC"`
	// SensitiveMaskAnnotationKeys are Secret annotation keys masked in the UI/CLI
	// (argocd-cm: resource.sensitive.mask.annotations).
	// Migration: if present, takes precedence over argocd-cm resource.sensitive.mask.annotations; replaces the whole collection.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	SensitiveMaskAnnotationKeys []string `json:"sensitiveMaskAnnotationKeys,omitempty" protobuf:"bytes,9,rep,name=sensitiveMaskAnnotationKeys"`
	// CustomLabelKeys are resource label keys shown in the resource node info panel
	// (argocd-cm: resource.customLabels).
	// Migration: if present, takes precedence over argocd-cm resource.customLabels; replaces the whole collection.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	CustomLabelKeys []string `json:"customLabelKeys,omitempty" protobuf:"bytes,10,rep,name=customLabelKeys"`
	// EventLabels controls which Application/AppProject label keys are copied onto
	// emitted Kubernetes events (argocd-cm: resource.includeEventLabelKeys /
	// resource.excludeEventLabelKeys).
	// Migration: if non-nil, takes precedence over argocd-cm resource.*EventLabelKeys as a group.
	// +optional
	EventLabels *EventLabelsConfig `json:"eventLabels,omitempty" protobuf:"bytes,11,opt,name=eventLabels"`
}

// EventLabelsConfig holds label key globs for event propagation.
type EventLabelsConfig struct {
	// IncludeKeyGlobs are label key globs copied onto emitted Kubernetes events
	// (argocd-cm: resource.includeEventLabelKeys).
	// Migration: if present, takes precedence over argocd-cm resource.includeEventLabelKeys; replaces the whole collection.
	// +optional
	IncludeKeyGlobs []string `json:"includeKeyGlobs,omitempty" protobuf:"bytes,1,rep,name=includeKeyGlobs"`
	// ExcludeKeyGlobs are label key globs excluded from event propagation
	// (argocd-cm: resource.excludeEventLabelKeys).
	// Migration: if present, takes precedence over argocd-cm resource.excludeEventLabelKeys; replaces the whole collection.
	// +optional
	ExcludeKeyGlobs []string `json:"excludeKeyGlobs,omitempty" protobuf:"bytes,2,rep,name=excludeKeyGlobs"`
}

// FilteredResource selects resources by API group, kind, and/or cluster for
// resource.exclusions / resource.inclusions. Omitting a field matches all values
// for that dimension. events.k8s.io and metrics.k8s.io are excluded by default
// even when this list is empty.
type FilteredResource struct {
	// APIGroups are API groups to match ("" for core, "*" for any).
	// Omitted = match all groups. Legacy: resource.exclusions/inclusions apiGroups.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*|[*])?$`
	APIGroups []string `json:"apiGroups,omitempty" protobuf:"bytes,1,rep,name=apiGroups"`
	// Kinds are Kubernetes Kind names to match ("*" for any).
	// Omitted = match all kinds. Legacy: resource.exclusions/inclusions kinds.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Z][A-Za-z0-9]*|[*])$`
	Kinds []string `json:"kinds,omitempty" protobuf:"bytes,2,rep,name=kinds"`
	// Clusters are cluster server URLs or names this filter applies to
	// (supports globs such as "*.local"). Omitted = all clusters.
	// Legacy: resource.exclusions/inclusions clusters.
	// +optional
	Clusters []string `json:"clusters,omitempty" protobuf:"bytes,3,rep,name=clusters"`
}

// CompareOptions controls how live vs desired state are compared
// (argocd-cm: resource.compareoptions).
type CompareOptions struct {
	// IgnoreAggregatedRoles ignores RBAC differences caused by aggregated
	// ClusterRoles (resource.compareoptions ignoreAggregatedRoles). Default false.
	// Migration: composite child of resource.compareoptions.
	// +optional
	IgnoreAggregatedRoles bool `json:"ignoreAggregatedRoles,omitempty" protobuf:"varint,1,opt,name=ignoreAggregatedRoles"`
	// IgnoreResourceStatusField controls ignoring .status during diff:
	// "all" (default in Argo CD), "crd" (CustomResourceDefinitions only), or "none".
	// Legacy: resource.compareoptions ignoreResourceStatusField.
	// Migration: composite child of resource.compareoptions.
	// +optional
	// +kubebuilder:validation:Enum=all;crd;none
	IgnoreResourceStatusField string `json:"ignoreResourceStatusField,omitempty" protobuf:"bytes,2,opt,name=ignoreResourceStatusField"`
	// IgnoreDifferencesOnResourceUpdates applies ignoreDifferences rules when
	// deciding whether a watched resource update should trigger reconcile
	// (resource.compareoptions ignoreDifferencesOnResourceUpdates). Default true.
	// Migration: composite child of resource.compareoptions.
	// +optional
	IgnoreDifferencesOnResourceUpdates bool `json:"ignoreDifferencesOnResourceUpdates,omitempty" protobuf:"varint,3,opt,name=ignoreDifferencesOnResourceUpdates"`
}

// ResourceHealthCustomization is a per-GVK health Lua override
// (argocd-cm: resource.customizations.health.<group_kind> /
// resource.customizations.useOpenLibs.<group_kind>).
type ResourceHealthCustomization struct {
	// Group is the API group ("" for core, "*" for wildcard).
	// Default empty string is required because group is a listMapKey.
	// +optional
	// +kubebuilder:default=""
	// +kubebuilder:validation:Pattern=`^$|^([a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*|[*])$`
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	// Kind is the Kubernetes Kind ("*" for wildcard). Required.
	// Wildcards are only valid in the unified resource.customizations YAML blob,
	// not in split ConfigMap keys.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([A-Z][A-Za-z0-9]*|[*])$`
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	// HealthLua is a Lua script that overrides built-in health assessment for this GVK.
	// Must return a table with status (Healthy/Progressing/Degraded/…) and optional message.
	// When absent, default health is Progressing until a status is produced.
	// Legacy: resource.customizations.health.<group_kind> or health.lua in the unified blob.
	// +optional
	HealthLua string `json:"healthLua,omitempty" protobuf:"bytes,3,opt,name=healthLua"`
	// UseOpenLibs enables standard Lua libraries for health and action scripts for this GVK
	// (disabled by default for security). Legacy: resource.customizations.useOpenLibs.<group_kind>
	// or health.lua.useOpenLibs. Default false. Lives on the health entry because Argo CD's
	// monolithic blob nests it under health.lua; the flag still applies to action scripts too.
	// +optional
	UseOpenLibs bool `json:"useOpenLibs,omitempty" protobuf:"varint,4,opt,name=useOpenLibs"`
}

// ResourceActionsCustomization is a per-GVK custom-actions override: discovery plus
// named action definitions (argocd-cm: resource.customizations.actions.<group_kind>).
type ResourceActionsCustomization struct {
	// Group is the API group ("" for core, "*" for wildcard).
	// Default empty string is required because group is a listMapKey.
	// +optional
	// +kubebuilder:default=""
	// +kubebuilder:validation:Pattern=`^$|^([a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*|[*])$`
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	// Kind is the Kubernetes Kind ("*" for wildcard). Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([A-Z][A-Za-z0-9]*|[*])$`
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	// DiscoveryLua is a Lua script that returns which custom actions are available
	// (and optional disabled flags) for the resource (legacy: discovery.lua).
	// +optional
	DiscoveryLua string `json:"discoveryLua,omitempty" protobuf:"bytes,3,opt,name=discoveryLua"`
	// MergeBuiltinActions merges custom actions with built-in ones instead of
	// replacing them (Argo CD ≥ 2.13; legacy: mergeBuiltinActions). Default false.
	// +optional
	MergeBuiltinActions bool `json:"mergeBuiltinActions,omitempty" protobuf:"varint,4,opt,name=mergeBuiltinActions"`
	// Definitions are named custom resource actions (legacy: definitions[] in the
	// resource.customizations.actions.<group_kind> YAML blob).
	// +optional
	// +listType=map
	// +listMapKey=name
	Definitions []ResourceActionDefinition `json:"definitions,omitempty" protobuf:"bytes,5,rep,name=definitions"`
}

// ResourceIgnoreCustomization is a per-GVK ignoreDifferences or ignoreResourceUpdates
// override (argocd-cm: resource.customizations.ignoreDifferences.<group_kind> /
// ignoreResourceUpdates.<group_kind>).
type ResourceIgnoreCustomization struct {
	// Group is the API group ("" for core, "*" for wildcard).
	// Default empty string is required because group is a listMapKey.
	// +optional
	// +kubebuilder:default=""
	// +kubebuilder:validation:Pattern=`^$|^([a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*|[*])$`
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	// Kind is the Kubernetes Kind ("*" for wildcard). Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([A-Z][A-Za-z0-9]*|[*])$`
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	// JSONPointers are RFC 6901 JSON Pointers to ignore (e.g. /spec/replicas).
	// +optional
	JSONPointers []string `json:"jsonPointers,omitempty" protobuf:"bytes,3,rep,name=jsonPointers"`
	// JQPathExpressions are jq expressions selecting fields to ignore.
	// +optional
	JQPathExpressions []string `json:"jqPathExpressions,omitempty" protobuf:"bytes,4,rep,name=jqPathExpressions"`
	// ManagedFieldsManagers ignores fields owned by these managedFields managers.
	// +optional
	ManagedFieldsManagers []string `json:"managedFieldsManagers,omitempty" protobuf:"bytes,5,rep,name=managedFieldsManagers"`
}

// ResourceKnownTypesCustomization is a per-GVK knownTypeFields override
// (argocd-cm: resource.customizations.knownTypeFields.<group_kind>).
type ResourceKnownTypesCustomization struct {
	// Group is the API group ("" for core, "*" for wildcard).
	// Default empty string is required because group is a listMapKey.
	// +optional
	// +kubebuilder:default=""
	// +kubebuilder:validation:Pattern=`^$|^([a-z0-9]([-a-z0-9]*[a-z0-9])?([.][a-z0-9]([-a-z0-9]*[a-z0-9])?)*|[*])$`
	Group string `json:"group,omitempty" protobuf:"bytes,1,opt,name=group"`
	// Kind is the Kubernetes Kind ("*" for wildcard). Required.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^([A-Z][A-Za-z0-9]*|[*])$`
	Kind string `json:"kind" protobuf:"bytes,2,opt,name=kind"`
	// Fields declares typed fields used for structured diff normalization
	// (e.g. treating a nested object as core/v1/PodSpec).
	// +optional
	// +listType=map
	// +listMapKey=field
	Fields []KnownTypeField `json:"fields,omitempty" protobuf:"bytes,3,rep,name=fields"`
}

// ClusterRegistrationConfig holds cluster-address policy settings (argocd-cm: cluster.*).
type ClusterRegistrationConfig struct {
	// InClusterEnabled controls whether the Kubernetes in-cluster API server
	// address (kubernetes.default.svc) may be registered as a cluster
	// (argocd-cm: cluster.inClusterEnabled).
	// Migration: if set, takes precedence over argocd-cm cluster.inClusterEnabled.
	// +optional
	InClusterEnabled *bool `json:"inClusterEnabled,omitempty" protobuf:"varint,1,opt,name=inClusterEnabled"`
}

// ControllerMetricsConfig holds controller Prometheus metric label/condition lists.
type ControllerMetricsConfig struct {
	// CacheExpiration is the Prometheus metrics cache TTL
	// (cmd-params: controller.metrics.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.metrics.cache.expiration.
	// +optional
	CacheExpiration *metav1.Duration `json:"cacheExpiration,omitempty" protobuf:"bytes,1,opt,name=cacheExpiration"`
	// Application configures Application-scoped Prometheus metrics export.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Application *ControllerMetricsApplicationConfig `json:"application,omitempty" protobuf:"bytes,2,opt,name=application"`
	// Cluster configures Cluster-scoped Prometheus metrics export.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Cluster *ControllerMetricsClusterConfig `json:"cluster,omitempty" protobuf:"bytes,3,opt,name=cluster"`
}

// ControllerMetricsApplicationConfig holds Application metric label/condition export settings.
type ControllerMetricsApplicationConfig struct {
	// LabelKeys are Application label keys exported on the argocd_app_labels metric
	// (cmd-params: controller.metrics.application.labels).
	// High-cardinality labels can degrade Prometheus performance.
	// Migration: if present, takes precedence over argocd-cmd-params-cm controller.metrics.application.labels; replaces the whole collection.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	LabelKeys []string `json:"labelKeys,omitempty" protobuf:"bytes,1,rep,name=labelKeys"`
	// Conditions are Application condition types exported on the argocd_app_conditions metric
	// (cmd-params: controller.metrics.application.conditions).
	// Migration: if present, takes precedence over argocd-cmd-params-cm controller.metrics.application.conditions; replaces the whole collection.
	// +optional
	Conditions []string `json:"conditions,omitempty" protobuf:"bytes,2,rep,name=conditions"`
}

// ControllerMetricsClusterConfig holds Cluster metric label export settings.
type ControllerMetricsClusterConfig struct {
	// LabelKeys are Cluster Secret label keys exported on the argocd_cluster_labels metric
	// (cmd-params: controller.metrics.cluster.labels).
	// Migration: if present, takes precedence over argocd-cmd-params-cm controller.metrics.cluster.labels; replaces the whole collection.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	LabelKeys []string `json:"labelKeys,omitempty" protobuf:"bytes,1,rep,name=labelKeys"`
}

// SelfHealConfig holds controller self-heal timing (cmd-params: controller.self.heal.*).
type SelfHealConfig struct {
	// Timeout is the minimum interval between self-heal attempts
	// (cmd-params: controller.self.heal.timeout.seconds). Zero means no minimum
	// (Argo CD default).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.self.heal.timeout.seconds.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty" protobuf:"bytes,1,opt,name=timeout"`
	// Cooldown maps to controller.self.heal.backoff.cooldown.seconds.
	// Deprecated in Argo CD: the CLI flag is marked deprecated and has no effect.
	// Kept for ConfigMap round-trip compatibility only.
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.self.heal.backoff.cooldown.seconds.
	// +optional
	Cooldown *metav1.Duration `json:"cooldown,omitempty" protobuf:"bytes,2,opt,name=cooldown"`
	// Backoff configures exponential backoff between self-heal attempts
	// (cmd-params: controller.self.heal.backoff.*). Defaults in Argo CD:
	// duration 2s, factor 3, maxDuration 300s.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.self.heal.backoff.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	Backoff *BackoffConfig `json:"backoff,omitempty" protobuf:"bytes,3,opt,name=backoff"`
}

// BackoffConfig is exponential backoff used for retries (mirrors Application sync
// retry backoff naming: duration / factor / maxDuration).
// Use this shape for any retry/backoff settings group rather than ad-hoc
// timeout/cap/baseBackoff field names.
type BackoffConfig struct {
	// Duration is the initial / base wait before the first retry.
	// Self-heal: controller.self.heal.backoff.timeout.seconds (default 2s).
	// K8s client: *.k8sclient.retry.base.backoff (default 100ms; doubles each
	// retry up to an internal 10s cap — only Duration is used for k8s client).
	// Application sync: backoff.duration.
	// +optional
	Duration *metav1.Duration `json:"duration,omitempty" protobuf:"bytes,1,opt,name=duration"`
	// Factor multiplies the wait after each failed attempt.
	// Self-heal: controller.self.heal.backoff.factor (default 3).
	// Unused for current k8s client retry (reserved for shape consistency).
	// +optional
	Factor *int32 `json:"factor,omitempty" protobuf:"varint,2,opt,name=factor"`
	// MaxDuration is the maximum wait between retries.
	// Self-heal: controller.self.heal.backoff.cap.seconds (default 300s).
	// Unused for current k8s client retry (hard-capped at 10s in code).
	// +optional
	MaxDuration *metav1.Duration `json:"maxDuration,omitempty" protobuf:"bytes,3,opt,name=maxDuration"`
}

// ApplicationSyncConfig holds application sync policy and controller sync timing.
type ApplicationSyncConfig struct {
	// Impersonation holds service-account impersonation policy during sync.
	// Migration: if non-nil, takes precedence over argocd-cm application.sync.impersonation.* as a group.
	// +optional
	Impersonation *SyncImpersonationConfig `json:"impersonation,omitempty" protobuf:"bytes,1,opt,name=impersonation"`
	// RequireOverridePrivilegeForRevisionSync requires the "override" RBAC privilege
	// to sync a revision other than the one declared in the Application
	// (argocd-cm: application.sync.requireOverridePrivilegeForRevisionSync).
	// Migration: if set, takes precedence over argocd-cm application.sync.requireOverridePrivilegeForRevisionSync.
	// +optional
	RequireOverridePrivilegeForRevisionSync *bool `json:"requireOverridePrivilegeForRevisionSync,omitempty" protobuf:"varint,2,opt,name=requireOverridePrivilegeForRevisionSync"`
	// Timeout is the maximum duration of a sync operation
	// (cmd-params: controller.sync.timeout.seconds). Zero means no timeout.
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.sync.timeout.seconds.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty" protobuf:"bytes,3,opt,name=timeout"`
	// Wave holds sync-wave settings (cmd-params: controller.sync.wave.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.sync.wave.* as a group.
	// +optional
	Wave *SyncWaveConfig `json:"wave,omitempty" protobuf:"bytes,4,opt,name=wave"`
}

// SyncWaveConfig holds sync-wave timing and related settings.
type SyncWaveConfig struct {
	// Delay is the pause between sync waves so other controllers can react
	// (cmd-params: controller.sync.wave.delay.seconds).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.sync.wave.delay.seconds.
	// +optional
	Delay *metav1.Duration `json:"delay,omitempty" protobuf:"bytes,1,opt,name=delay"`
}

// SyncImpersonationConfig holds sync impersonation policy.
type SyncImpersonationConfig struct {
	// Mode controls sync service-account impersonation
	// (argocd-cm: application.sync.impersonation.enabled / .enforced):
	// disabled — impersonation off;
	// optional — enabled with enforced=false (fall back to controller SA when no destination SA);
	// required — enabled with enforced=true or enforced omitted (Argo CD default when enabled).
	// Migration: if set, takes precedence over argocd-cm application.sync.impersonation.enabled and .enforced as a pair.
	// +optional
	// +kubebuilder:validation:Enum=disabled;optional;required
	Mode string `json:"mode,omitempty" protobuf:"bytes,1,opt,name=mode"`
}

// --- Controller ---

// ControllerConfig holds application-controller runtime and product settings.
type ControllerConfig struct {
	// Reconciliation holds periodic app reconciliation / git poll timing
	// (argocd-cm: timeout.reconciliation, timeout.reconciliation.jitter).
	// Migration: if non-nil, takes precedence over argocd-cm timeout.reconciliation* as a group.
	// +optional
	Reconciliation *ReconciliationConfig `json:"reconciliation,omitempty" protobuf:"bytes,1,opt,name=reconciliation"`
	// Sharding holds controller shard balancing settings
	// (cmd-params: controller.sharding.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.sharding.* as a group.
	// +optional
	Sharding *ControllerShardingConfig `json:"sharding,omitempty" protobuf:"bytes,2,opt,name=sharding"`
	// Processors holds parallel worker counts for status, operations, and hydration.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.*.processors as a group.
	// +optional
	Processors *ControllerProcessorsConfig `json:"processors,omitempty" protobuf:"bytes,3,opt,name=processors"`
	// Log holds controller log format and level (cmd-params: controller.log.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.log.* as a group.
	// +optional
	Log *LogConfig `json:"log,omitempty" protobuf:"bytes,4,opt,name=log"`
	// Metrics configures which labels/conditions are exported as Prometheus metrics.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Metrics *ControllerMetricsConfig `json:"metrics,omitempty" protobuf:"bytes,5,opt,name=metrics"`
	// SelfHeal configures automated self-heal attempt timing and backoff.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.self.heal.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	SelfHeal *SelfHealConfig `json:"selfHeal,omitempty" protobuf:"bytes,6,opt,name=selfHeal"`
	// Cache holds controller Redis cache TTLs (cmd-params: controller.app.state.cache.expiration,
	// controller.default.cache.expiration).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.*.cache.expiration as a group.
	// +optional
	Cache *ControllerCacheConfig `json:"cache,omitempty" protobuf:"bytes,7,opt,name=cache"`
	// ResourceHealthPersist stores per-resource health in the Application CR
	// (cmd-params: controller.resource.health.persist). Increases CR update load.
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.resource.health.persist.
	// +optional
	ResourceHealthPersist *bool `json:"resourceHealthPersist,omitempty" protobuf:"varint,8,opt,name=resourceHealthPersist"`
	// KubectlParallelismLimit caps concurrent kubectl fork/execs
	// (cmd-params: controller.kubectl.parallelism.limit). Zero means unlimited.
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.kubectl.parallelism.limit.
	// +optional
	// +kubebuilder:validation:Minimum=0
	KubectlParallelismLimit *int32 `json:"kubectlParallelismLimit,omitempty" protobuf:"varint,9,opt,name=kubectlParallelismLimit"`
	// Diff holds server-side diff and global ignore-differences settings.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Diff *ControllerDiffConfig `json:"diff,omitempty" protobuf:"bytes,10,opt,name=diff"`
	// ProfileEnabled enables pprof profiling endpoints
	// (cmd-params: controller.profile.enabled).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.profile.enabled.
	// +optional
	ProfileEnabled *bool `json:"profileEnabled,omitempty" protobuf:"varint,11,opt,name=profileEnabled"`
	// ProfilerFilePath enables the Go profiler and writes profiles to this path
	// (env: ARGOCD_ENABLE_PROFILER_FILE_PATH).
	// Migration: if set, takes precedence over env ARGOCD_ENABLE_PROFILER_FILE_PATH. Does not round-trip through ConfigMaps.
	// +optional
	ProfilerFilePath string `json:"profilerFilePath,omitempty" protobuf:"bytes,12,opt,name=profilerFilePath"`
	// GRPCTXTServiceConfigEnabled enables gRPC TXT service config
	// (cmd-params: controller.grpc.enable.txt.service.config).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.grpc.enable.txt.service.config.
	// +optional
	GRPCTXTServiceConfigEnabled *bool `json:"grpcTXTServiceConfigEnabled,omitempty" protobuf:"varint,13,opt,name=grpcTXTServiceConfigEnabled"`
	// RepoErrorGracePeriod is how long the controller tolerates repo-server errors
	// before failing an operation (cmd-params: controller.repo.error.grace.period.seconds).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.repo.error.grace.period.seconds.
	// +optional
	RepoErrorGracePeriod *metav1.Duration `json:"repoErrorGracePeriod,omitempty" protobuf:"bytes,14,opt,name=repoErrorGracePeriod"`
	// ClusterCache tunes cluster cache event batching
	// (cmd-params: controller.cluster.cache.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.cluster.cache.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	ClusterCache *ControllerClusterCacheConfig `json:"clusterCache,omitempty" protobuf:"bytes,15,opt,name=clusterCache"`
	// K8sClient holds Kubernetes client tuning for the application controller
	// (cmd-params: controller.k8s.* / controller.k8sclient.*).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	K8sClient *K8sClientConfig `json:"k8sClient,omitempty" protobuf:"bytes,16,opt,name=k8sClient"`
	// Resource holds watch filters and per-GVK customizations.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Resource *ResourceConfig `json:"resource,omitempty" protobuf:"bytes,17,opt,name=resource"`
	// InstanceLabelKey is the label key used to track managed resources
	// (argocd-cm: application.instanceLabelKey). Default is app.kubernetes.io/instance.
	// Migration: if set, takes precedence over argocd-cm application.instanceLabelKey.
	// +optional
	// +kubebuilder:validation:Pattern=`^$|^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	InstanceLabelKey string `json:"instanceLabelKey,omitempty" protobuf:"bytes,18,opt,name=instanceLabelKey"`
	// ResourceTrackingMethod selects how ownership is recorded on managed resources:
	// "label", "annotation", or "annotation+label"
	// (argocd-cm: application.resourceTrackingMethod).
	// Migration: if set, takes precedence over argocd-cm application.resourceTrackingMethod.
	// +optional
	// +kubebuilder:validation:Enum=annotation;label;annotation+label
	ResourceTrackingMethod string `json:"resourceTrackingMethod,omitempty" protobuf:"bytes,19,opt,name=resourceTrackingMethod"`
	// AllowedNodeLabelKeys are node label keys shown in the application pod view
	// (argocd-cm: application.allowedNodeLabels).
	// Migration: if present, takes precedence over argocd-cm application.allowedNodeLabels; replaces the whole collection.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	AllowedNodeLabelKeys []string `json:"allowedNodeLabelKeys,omitempty" protobuf:"bytes,20,rep,name=allowedNodeLabelKeys"`
	// Sync holds sync impersonation policy and sync timing.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Sync *ApplicationSyncConfig `json:"sync,omitempty" protobuf:"bytes,21,opt,name=sync"`
	// GlobalProjects defines global AppProjects applied to matching projects by
	// label selector (argocd-cm: globalProjects).
	// Migration: if present, takes precedence over argocd-cm globalProjects; replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=projectName
	GlobalProjects []GlobalProjectConfig `json:"globalProjects,omitempty" protobuf:"bytes,22,rep,name=globalProjects"`
	// SourceHydrator configures the beta manifest source hydrator.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	SourceHydrator *SourceHydratorConfig `json:"sourceHydrator,omitempty" protobuf:"bytes,23,opt,name=sourceHydrator"`
}

// ControllerDiffConfig holds server-side diff and global ignore-differences settings.
type ControllerDiffConfig struct {
	// ServerSide holds server-side diff via server-side apply dry-run when the
	// diff cache is unavailable (cmd-params: controller.diff.server.side).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm controller.diff.server.side as a group.
	// +optional
	ServerSide *DiffServerSideConfig `json:"serverSide,omitempty" protobuf:"bytes,1,opt,name=serverSide"`
	// CompareOptions controls default diff behavior (argocd-cm: resource.compareoptions).
	// Migration: if non-nil, takes precedence over argocd-cm resource.compareoptions as a whole, including all child fields.
	// +optional
	CompareOptions *CompareOptions `json:"compareOptions,omitempty" protobuf:"bytes,2,opt,name=compareOptions"`
	// IgnoreResourceUpdatesEnabled is the master switch for ignoreResourceUpdates
	// rules that skip reconciles on watched updates
	// (argocd-cm: resource.ignoreResourceUpdatesEnabled). Default true in Argo CD —
	// when false, all resource updates are applied to the cluster cache.
	// Migration: if set, takes precedence over argocd-cm resource.ignoreResourceUpdatesEnabled.
	// +optional
	IgnoreResourceUpdatesEnabled *bool `json:"ignoreResourceUpdatesEnabled,omitempty" protobuf:"varint,3,opt,name=ignoreResourceUpdatesEnabled"`
	// IgnoreNormalizerJQTimeout is the max time to evaluate a JQPathExpression
	// ignore-differences rule (cmd-params: controller.ignore.normalizer.jq.timeout).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.ignore.normalizer.jq.timeout.
	// +optional
	IgnoreNormalizerJQTimeout *metav1.Duration `json:"ignoreNormalizerJQTimeout,omitempty" protobuf:"bytes,4,opt,name=ignoreNormalizerJQTimeout"`
}

// DiffServerSideConfig holds server-side diff enablement.
type DiffServerSideConfig struct {
	// Enabled enables server-side diff (cmd-params: controller.diff.server.side).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.diff.server.side.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
}

// ReconciliationConfig holds controller reconciliation timing.
type ReconciliationConfig struct {
	// Timeout is the periodic app reconciliation / git poll interval
	// (argocd-cm: timeout.reconciliation). Also used as the repo-server git revision cache TTL.
	// Zero disables polling.
	// Migration: if set, takes precedence over argocd-cm timeout.reconciliation.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty" protobuf:"bytes,1,opt,name=timeout"`
	// HardTimeout is the hard resync interval that forces reconciliation even when
	// the repo has not changed (argocd-cm: timeout.hard.reconciliation).
	// Migration: if set, takes precedence over argocd-cm timeout.hard.reconciliation.
	// +optional
	HardTimeout *metav1.Duration `json:"hardTimeout,omitempty" protobuf:"bytes,2,opt,name=hardTimeout"`
	// Jitter is random additional delay added to reconciliation (argocd-cm: timeout.reconciliation.jitter).
	// Migration: if set, takes precedence over argocd-cm timeout.reconciliation.jitter.
	// +optional
	Jitter *metav1.Duration `json:"jitter,omitempty" protobuf:"bytes,3,opt,name=jitter"`
}

// ControllerShardingConfig holds how clusters are balanced across controller shards.
type ControllerShardingConfig struct {
	// Algorithm selects how clusters are balanced across controller shards
	// (cmd-params: controller.sharding.algorithm).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.sharding.algorithm.
	// +optional
	// +kubebuilder:validation:Enum=legacy;round-robin;consistent-hashing
	Algorithm string `json:"algorithm,omitempty" protobuf:"bytes,1,opt,name=algorithm"`
}

// ControllerProcessorsConfig holds parallel worker counts for the application controller.
type ControllerProcessorsConfig struct {
	// Status is the number of parallel application status processors
	// (cmd-params: controller.status.processors).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.status.processors.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Status *int32 `json:"status,omitempty" protobuf:"varint,1,opt,name=status"`
	// Operation is the number of parallel sync/operation processors
	// (cmd-params: controller.operation.processors).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.operation.processors.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Operation *int32 `json:"operation,omitempty" protobuf:"varint,2,opt,name=operation"`
	// Hydration is the number of manifest hydration workers when the source hydrator is enabled
	// (cmd-params: controller.hydration.processors).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.hydration.processors.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Hydration *int32 `json:"hydration,omitempty" protobuf:"varint,3,opt,name=hydration"`
}

// ControllerCacheConfig holds controller Redis cache TTLs.
type ControllerCacheConfig struct {
	// AppStateExpiration is the app state (tree / managed resources) cache TTL
	// (cmd-params: controller.app.state.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.app.state.cache.expiration.
	// +optional
	AppStateExpiration *metav1.Duration `json:"appStateExpiration,omitempty" protobuf:"bytes,1,opt,name=appStateExpiration"`
	// DefaultExpiration is the default Redis cache TTL for the controller
	// (cmd-params: controller.default.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.default.cache.expiration.
	// +optional
	DefaultExpiration *metav1.Duration `json:"defaultExpiration,omitempty" protobuf:"bytes,2,opt,name=defaultExpiration"`
}

// RepoServerConfig holds repo-server address, client, and manifest-tool settings.
type RepoServerConfig struct {
	// Address is the repo-server host:port (cmd-params: repo.server). Not a full URL.
	// Migration: if set, takes precedence over argocd-cmd-params-cm repo.server.
	// +optional
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address"`
	// Client holds shared TLS/timeout settings used by components connecting to
	// the repo-server (controller.repo.server.* / server.repo.server.*).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Client *RepoServerClientConfig `json:"client,omitempty" protobuf:"bytes,2,opt,name=client"`
	// ParallelismLimit caps concurrent manifest generation requests
	// (cmd-params: reposerver.parallelism.limit). Zero means unlimited.
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.parallelism.limit.
	// +optional
	// +kubebuilder:validation:Minimum=0
	ParallelismLimit *int32 `json:"parallelismLimit,omitempty" protobuf:"varint,3,opt,name=parallelismLimit"`
	// Listen holds repo-server and metrics listen addresses
	// (cmd-params: reposerver.listen.address, reposerver.metrics.listen.address).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.listen.address /
	// reposerver.metrics.listen.address as a group.
	// +optional
	Listen *ListenConfig `json:"listen,omitempty" protobuf:"bytes,4,opt,name=listen"`
	// TLSEnabled controls whether the repo-server serves TLS (cmd-params: reposerver.disable.tls —
	// inverted: disable.tls=true means tlsEnabled=false). TLS is on by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.disable.tls (inverted).
	// +optional
	TLSEnabled *bool `json:"tlsEnabled,omitempty" protobuf:"varint,5,opt,name=tlsEnabled"`
	// Git holds git submodule, timeout, and built-in config settings.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.enable.git.submodule /
	// reposerver.git.* / reposerver.enable.builtin.git.config as a group.
	// +optional
	Git *RepoServerGitConfig `json:"git,omitempty" protobuf:"bytes,6,opt,name=git"`
	// AllowOOBSymlinks allows out-of-bounds symlinks in repos
	// (cmd-params: reposerver.allow.oob.symlinks).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.allow.oob.symlinks.
	// +optional
	AllowOOBSymlinks *bool `json:"allowOOBSymlinks,omitempty" protobuf:"varint,7,opt,name=allowOOBSymlinks"`
	// IncludeHiddenDirectories includes hidden directories when generating manifests
	// (cmd-params: reposerver.include.hidden.directories).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.include.hidden.directories.
	// +optional
	IncludeHiddenDirectories *bool `json:"includeHiddenDirectories,omitempty" protobuf:"varint,8,opt,name=includeHiddenDirectories"`
	// Cache holds repo-server Redis cache TTLs (cmd-params: reposerver.default.cache.expiration,
	// reposerver.repo.cache.expiration).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.*.cache.expiration as a group.
	// +optional
	Cache *RepoServerCacheConfig `json:"cache,omitempty" protobuf:"bytes,9,opt,name=cache"`
	// MaxCombinedDirectoryManifestsSize caps combined directory manifests size
	// (cmd-params: reposerver.max.combined.directory.manifests.size).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.max.combined.directory.manifests.size.
	// +optional
	MaxCombinedDirectoryManifestsSize *resource.Quantity `json:"maxCombinedDirectoryManifestsSize,omitempty" protobuf:"bytes,10,opt,name=maxCombinedDirectoryManifestsSize"`
	// StreamedManifest caps streamed manifest tar and extracted sizes
	// (cmd-params: reposerver.streamed.manifest.max.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.streamed.manifest.max.* as a group.
	// +optional
	StreamedManifest *StreamedManifestConfig `json:"streamedManifest,omitempty" protobuf:"bytes,11,opt,name=streamedManifest"`
	// Plugin holds config management plugin settings.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.plugin.* as a group.
	// +optional
	Plugin *RepoServerPluginConfig `json:"plugin,omitempty" protobuf:"bytes,12,opt,name=plugin"`
	// ProfileEnabled enables pprof profiling endpoints
	// (cmd-params: reposerver.profile.enabled).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.profile.enabled.
	// +optional
	ProfileEnabled *bool `json:"profileEnabled,omitempty" protobuf:"varint,13,opt,name=profileEnabled"`
	// GRPCTXTServiceConfigEnabled enables gRPC TXT service config
	// (cmd-params: reposerver.grpc.enable.txt.service.config).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.grpc.enable.txt.service.config.
	// +optional
	GRPCTXTServiceConfigEnabled *bool `json:"grpcTXTServiceConfigEnabled,omitempty" protobuf:"varint,14,opt,name=grpcTXTServiceConfigEnabled"`
	// ClientCAPath is the path to a client CA for mTLS
	// (cmd-params: reposerver.client.ca.path).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.client.ca.path.
	// +optional
	ClientCAPath string `json:"clientCAPath,omitempty" protobuf:"bytes,15,opt,name=clientCAPath"`
	// TLS holds repo-server TLS cipher/version settings (cmd-params: reposerver.tls.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.tls.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	TLS *TLSVersionConfig `json:"tls,omitempty" protobuf:"bytes,16,opt,name=tls"`
	// OCI holds OCI registry size and media-type limits (cmd-params: reposerver.oci.*).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	OCI *RepoServerOCIConfig `json:"oci,omitempty" protobuf:"bytes,17,opt,name=oci"`
	// GRPCMaxSize caps gRPC response message size for the repo-server
	// (cmd-params: reposerver.grpc.max.size; historically an integer megabyte count).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.grpc.max.size.
	// +optional
	GRPCMaxSize *resource.Quantity `json:"grpcMaxSize,omitempty" protobuf:"bytes,18,opt,name=grpcMaxSize"`
	// RevisionCacheLockTimeout is how long to wait for the revision-cache lock
	// (cmd-params: reposerver.revision.cache.lock.timeout).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.revision.cache.lock.timeout.
	// +optional
	RevisionCacheLockTimeout *metav1.Duration `json:"revisionCacheLockTimeout,omitempty" protobuf:"bytes,19,opt,name=revisionCacheLockTimeout"`
	// Jsonnet holds Jsonnet tool enablement (argocd-cm: jsonnet.enable).
	// Migration: if non-nil, takes precedence over argocd-cm jsonnet.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	Jsonnet *JsonnetConfig `json:"jsonnet,omitempty" protobuf:"bytes,20,opt,name=jsonnet"`
	// Log holds repo-server log format and level (cmd-params: reposerver.log.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.log.* as a group.
	// +optional
	Log *LogConfig `json:"log,omitempty" protobuf:"bytes,21,opt,name=log"`
	// Kustomize holds Kustomize enablement, build options, and version binaries.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Kustomize *KustomizeConfig `json:"kustomize,omitempty" protobuf:"bytes,22,opt,name=kustomize"`
	// Helm holds Helm enablement, values-file schemes, and repo-server Helm limits.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	Helm *HelmConfig `json:"helm,omitempty" protobuf:"bytes,23,opt,name=helm"`
}

// RepoServerGitConfig holds repo-server git tool settings.
type RepoServerGitConfig struct {
	// SubmoduleEnabled controls git submodule support
	// (cmd-params: reposerver.enable.git.submodule). Submodules are on by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.enable.git.submodule.
	// +optional
	SubmoduleEnabled *bool `json:"submoduleEnabled,omitempty" protobuf:"varint,1,opt,name=submoduleEnabled"`
	// RequestTimeout is the timeout for git network operations
	// (cmd-params: reposerver.git.request.timeout; also env ARGOCD_GIT_REQUEST_TIMEOUT).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.git.request.timeout.
	// +optional
	RequestTimeout *metav1.Duration `json:"requestTimeout,omitempty" protobuf:"bytes,2,opt,name=requestTimeout"`
	// LSRemoteParallelismLimit caps concurrent git ls-remote calls
	// (cmd-params: reposerver.git.lsremote.parallelism.limit; also env ARGOCD_GIT_LS_REMOTE_PARALLELISM_LIMIT).
	// Zero means unlimited.
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.git.lsremote.parallelism.limit.
	// +optional
	// +kubebuilder:validation:Minimum=0
	LSRemoteParallelismLimit *int32 `json:"lsRemoteParallelismLimit,omitempty" protobuf:"varint,3,opt,name=lsRemoteParallelismLimit"`
	// BuiltinConfigEnabled controls Argo CD's built-in git config
	// (cmd-params: reposerver.enable.builtin.git.config). On by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.enable.builtin.git.config.
	// +optional
	BuiltinConfigEnabled *bool `json:"builtinConfigEnabled,omitempty" protobuf:"varint,4,opt,name=builtinConfigEnabled"`
}

// RepoServerCacheConfig holds repo-server Redis cache TTLs.
type RepoServerCacheConfig struct {
	// DefaultExpiration is the default Redis cache TTL for the repo-server
	// (cmd-params: reposerver.default.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.default.cache.expiration.
	// +optional
	DefaultExpiration *metav1.Duration `json:"defaultExpiration,omitempty" protobuf:"bytes,1,opt,name=defaultExpiration"`
	// RepoExpiration is the repository cache TTL
	// (cmd-params: reposerver.repo.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.repo.cache.expiration.
	// +optional
	RepoExpiration *metav1.Duration `json:"repoExpiration,omitempty" protobuf:"bytes,2,opt,name=repoExpiration"`
}

// StreamedManifestConfig caps streamed manifest tar and extracted sizes.
type StreamedManifestConfig struct {
	// MaxTarSize caps streamed manifest tar size
	// (cmd-params: reposerver.streamed.manifest.max.tar.size).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.streamed.manifest.max.tar.size.
	// +optional
	MaxTarSize *resource.Quantity `json:"maxTarSize,omitempty" protobuf:"bytes,1,opt,name=maxTarSize"`
	// MaxExtractedSize caps extracted streamed manifest size
	// (cmd-params: reposerver.streamed.manifest.max.extracted.size).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.streamed.manifest.max.extracted.size.
	// +optional
	MaxExtractedSize *resource.Quantity `json:"maxExtractedSize,omitempty" protobuf:"bytes,2,opt,name=maxExtractedSize"`
}

// RepoServerPluginConfig holds config management plugin settings.
type RepoServerPluginConfig struct {
	// UseManifestGeneratePaths enables CMP manifest-generate-paths support
	// (cmd-params: reposerver.plugin.use.manifest.generate.paths).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.plugin.use.manifest.generate.paths.
	// +optional
	UseManifestGeneratePaths *bool `json:"useManifestGeneratePaths,omitempty" protobuf:"varint,1,opt,name=useManifestGeneratePaths"`
	// TarExclusionGlobs are path globs excluded from tarballs streamed to config management plugins
	// (cmd-params: reposerver.plugin.tar.exclusions; legacy separator is ';').
	// Migration: if present, takes precedence over argocd-cmd-params-cm reposerver.plugin.tar.exclusions; replaces the whole collection.
	// +optional
	TarExclusionGlobs []string `json:"tarExclusionGlobs,omitempty" protobuf:"bytes,2,rep,name=tarExclusionGlobs"`
}

// RepoServerClientConfig is how other components connect to the repo-server.
type RepoServerClientConfig struct {
	// Timeout is the RPC timeout when calling the repo-server
	// (cmd-params: *.repo.server.timeout.seconds).
	// Migration: if set, takes precedence over argocd-cmd-params-cm *.repo.server.timeout.seconds.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty" protobuf:"bytes,1,opt,name=timeout"`
	// TLSEnabled controls TLS when connecting to the repo-server
	// (cmd-params: *.repo.server.plaintext — inverted: plaintext=true means tlsEnabled=false).
	// Migration: if set, takes precedence over argocd-cmd-params-cm *.repo.server.plaintext (inverted).
	// +optional
	TLSEnabled *bool `json:"tlsEnabled,omitempty" protobuf:"varint,2,opt,name=tlsEnabled"`
	// InsecureSkipVerify skips verification of the repo-server TLS certificate
	// (cmd-params: *.repo.server.strict.tls — inverted: strict.tls=true means skip=false).
	// Migration: if set, takes precedence over argocd-cmd-params-cm *.repo.server.strict.tls (inverted).
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty" protobuf:"varint,3,opt,name=insecureSkipVerify"`
	// MTLS holds mTLS certificate paths for connecting to the repo-server.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm *.repo.server.*.cert.path as a group.
	// +optional
	MTLS *MTLSCertConfig `json:"mtls,omitempty" protobuf:"bytes,4,opt,name=mtls"`
}

// MTLSCertConfig holds mTLS certificate paths.
type MTLSCertConfig struct {
	// CACertPath is the path to a CA certificate for verifying the peer
	// (cmd-params: *.repo.server.ca.cert.path).
	// Migration: if set, takes precedence over the component's argocd-cmd-params-cm *.repo.server.ca.cert.path.
	// +optional
	CACertPath string `json:"caCertPath,omitempty" protobuf:"bytes,1,opt,name=caCertPath"`
	// ClientCertPath is the path to the client certificate for mTLS
	// (cmd-params: *.repo.server.client.cert.path).
	// Migration: if set, takes precedence over the component's argocd-cmd-params-cm *.repo.server.client.cert.path.
	// +optional
	ClientCertPath string `json:"clientCertPath,omitempty" protobuf:"bytes,2,opt,name=clientCertPath"`
	// ClientCertKeyPath is the path to the client certificate key for mTLS
	// (cmd-params: *.repo.server.client.cert.key.path).
	// Migration: if set, takes precedence over the component's argocd-cmd-params-cm *.repo.server.client.cert.key.path.
	// +optional
	ClientCertKeyPath string `json:"clientCertKeyPath,omitempty" protobuf:"bytes,3,opt,name=clientCertKeyPath"`
}

// CommitServerConfig holds commit-server runtime settings and commit identity.
type CommitServerConfig struct {
	// Address is the commit-server host:port (cmd-params: commit.server).
	// Migration: if set, takes precedence over argocd-cmd-params-cm commit.server.
	// +optional
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address"`
	// Listen holds commit-server and metrics listen addresses
	// (cmd-params: commitserver.listen.address, commitserver.metrics.listen.address).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm commitserver.listen.address /
	// commitserver.metrics.listen.address as a group.
	// +optional
	Listen *ListenConfig `json:"listen,omitempty" protobuf:"bytes,2,opt,name=listen"`
	// GRPCTXTServiceConfigEnabled enables gRPC TXT service config
	// (cmd-params: commitserver.grpc.enable.txt.service.config).
	// Migration: if set, takes precedence over argocd-cmd-params-cm commitserver.grpc.enable.txt.service.config.
	// +optional
	GRPCTXTServiceConfigEnabled *bool `json:"grpcTXTServiceConfigEnabled,omitempty" protobuf:"varint,3,opt,name=grpcTXTServiceConfigEnabled"`
	// Commit holds author identity used for hydrator-created commits.
	// Migration: if non-nil, takes precedence over argocd-cm commit.author.* as a group (children apply from the CR; no merge with legacy siblings under that family).
	// +optional
	Commit *CommitConfig `json:"commit,omitempty" protobuf:"bytes,4,opt,name=commit"`
	// Log holds commit-server log format and level (cmd-params: commitserver.log.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm commitserver.log.* as a group.
	// +optional
	Log *LogConfig `json:"log,omitempty" protobuf:"bytes,5,opt,name=log"`
}

// ApplicationSetConfig holds ApplicationSet controller settings.
type ApplicationSetConfig struct {
	// NamespaceGlobs are additional namespaces where ApplicationSets may live
	// (cmd-params: applicationsetcontroller.namespaces). Supports globs.
	// Migration: if present, takes precedence over argocd-cmd-params-cm applicationsetcontroller.namespaces; replaces the whole collection.
	// +optional
	NamespaceGlobs []string `json:"namespaceGlobs,omitempty" protobuf:"bytes,1,rep,name=namespaceGlobs"`
	// Policy is the default ApplicationSet sync policy when not overridden:
	// create-only, create-update, create-delete, or sync (create+update+delete).
	// Empty means sync (cmd-params: applicationsetcontroller.policy).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.policy.
	// +optional
	// +kubebuilder:validation:Enum=;create-only;create-update;create-delete;sync
	Policy string `json:"policy,omitempty" protobuf:"bytes,2,opt,name=policy"`
	// AllowedSCMProviderURLs allowlists SCM provider base URLs for SCM/PR generators
	// (cmd-params: applicationsetcontroller.allowed.scm.providers).
	// Migration: if present, takes precedence over argocd-cmd-params-cm applicationsetcontroller.allowed.scm.providers; replaces the whole collection.
	// +optional
	// +listType=atomic
	// +kubebuilder:validation:MaxItems=64
	// +kubebuilder:validation:items:MaxLength=2048
	// +kubebuilder:validation:items:Pattern=`^https?://.+`
	AllowedSCMProviderURLs []string `json:"allowedSCMProviderURLs,omitempty" protobuf:"bytes,3,rep,name=allowedSCMProviderURLs"`
	// GlobalPreserved holds annotation and label keys preserved on generated Applications.
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm applicationsetcontroller.global.preserved.* as a group.
	// +optional
	GlobalPreserved *GlobalPreservedKeysConfig `json:"globalPreserved,omitempty" protobuf:"bytes,4,opt,name=globalPreserved"`
	// SCMProvidersEnabled enables SCM provider generators
	// (cmd-params: applicationsetcontroller.enable.scm.providers).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.scm.providers.
	// +optional
	SCMProvidersEnabled *bool `json:"scmProvidersEnabled,omitempty" protobuf:"varint,5,opt,name=scmProvidersEnabled"`
	// PolicyOverrideEnabled allows ApplicationSets to override the controller policy
	// (cmd-params: applicationsetcontroller.enable.policy.override).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.policy.override.
	// +optional
	PolicyOverrideEnabled *bool `json:"policyOverrideEnabled,omitempty" protobuf:"varint,6,opt,name=policyOverrideEnabled"`
	// ProgressiveSyncs holds progressive syncs settings
	// (cmd-params: applicationsetcontroller.enable.progressive.syncs).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.progressive.syncs as a group.
	// +optional
	ProgressiveSyncs *ProgressiveSyncsConfig `json:"progressiveSyncs,omitempty" protobuf:"bytes,7,opt,name=progressiveSyncs"`
	// GitSubmoduleEnabled controls git submodule support
	// (cmd-params: applicationsetcontroller.enable.git.submodule). On by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.git.submodule.
	// +optional
	GitSubmoduleEnabled *bool `json:"gitSubmoduleEnabled,omitempty" protobuf:"varint,8,opt,name=gitSubmoduleEnabled"`
	// NewGitFileGlobbingEnabled enables the new git file globbing behavior
	// (cmd-params: applicationsetcontroller.enable.new.git.file.globbing).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.new.git.file.globbing.
	// +optional
	NewGitFileGlobbingEnabled *bool `json:"newGitFileGlobbingEnabled,omitempty" protobuf:"varint,9,opt,name=newGitFileGlobbingEnabled"`
	// TokenRefStrictModeEnabled enables strict token ref validation
	// (cmd-params: applicationsetcontroller.enable.tokenref.strict.mode).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.tokenref.strict.mode.
	// +optional
	TokenRefStrictModeEnabled *bool `json:"tokenRefStrictModeEnabled,omitempty" protobuf:"varint,10,opt,name=tokenRefStrictModeEnabled"`
	// GitHubAPIMetricsEnabled enables GitHub API Prometheus metrics
	// (cmd-params: applicationsetcontroller.enable.github.api.metrics).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.github.api.metrics.
	// +optional
	GitHubAPIMetricsEnabled *bool `json:"gitHubAPIMetricsEnabled,omitempty" protobuf:"varint,11,opt,name=gitHubAPIMetricsEnabled"`
	// DryRun runs ApplicationSet reconciliation without writing Applications
	// (cmd-params: applicationsetcontroller.dryrun).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.dryrun.
	// +optional
	DryRun *bool `json:"dryRun,omitempty" protobuf:"varint,12,opt,name=dryRun"`
	// LeaderElectionEnabled enables leader election
	// (cmd-params: applicationsetcontroller.enable.leader.election).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.leader.election.
	// +optional
	LeaderElectionEnabled *bool `json:"leaderElectionEnabled,omitempty" protobuf:"varint,13,opt,name=leaderElectionEnabled"`
	// ReconciliationsParallelismLimit caps concurrent ApplicationSet reconciles
	// (cmd-params: applicationsetcontroller.concurrent.reconciliations.max).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.concurrent.reconciliations.max.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ReconciliationsParallelismLimit *int32 `json:"reconciliationsParallelismLimit,omitempty" protobuf:"varint,14,opt,name=reconciliationsParallelismLimit"`
	// RequeueAfter is the default requeue interval
	// (cmd-params: applicationsetcontroller.requeue.after).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.requeue.after.
	// +optional
	RequeueAfter *metav1.Duration `json:"requeueAfter,omitempty" protobuf:"bytes,15,opt,name=requeueAfter"`
	// WebhookParallelismLimit caps concurrent webhook processing
	// (cmd-params: applicationsetcontroller.webhook.parallelism.limit).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.webhook.parallelism.limit.
	// +optional
	// +kubebuilder:validation:Minimum=1
	WebhookParallelismLimit *int32 `json:"webhookParallelismLimit,omitempty" protobuf:"varint,16,opt,name=webhookParallelismLimit"`
	// StatusMaxResourcesCount caps resources reported in ApplicationSet status
	// (cmd-params: applicationsetcontroller.status.max.resources.count).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.status.max.resources.count.
	// +optional
	// +kubebuilder:validation:Minimum=0
	StatusMaxResourcesCount *int32 `json:"statusMaxResourcesCount,omitempty" protobuf:"varint,17,opt,name=statusMaxResourcesCount"`
	// SCMRootCAPath is the path to a root CA for SCM TLS
	// (cmd-params: applicationsetcontroller.scm.root.ca.path).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.scm.root.ca.path.
	// +optional
	SCMRootCAPath string `json:"scmRootCAPath,omitempty" protobuf:"bytes,18,opt,name=scmRootCAPath"`
	// Log holds ApplicationSet controller log format and level
	// (cmd-params: applicationsetcontroller.log.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm applicationsetcontroller.log.* as a group.
	// +optional
	Log *LogConfig `json:"log,omitempty" protobuf:"bytes,19,opt,name=log"`
	// ProfileEnabled enables pprof profiling endpoints
	// (cmd-params: applicationsetcontroller.profile.enabled).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.profile.enabled.
	// +optional
	ProfileEnabled *bool `json:"profileEnabled,omitempty" protobuf:"varint,20,opt,name=profileEnabled"`
	// GRPCTXTServiceConfigEnabled enables gRPC TXT service config
	// (cmd-params: applicationsetcontroller.grpc.enable.txt.service.config).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.grpc.enable.txt.service.config.
	// +optional
	GRPCTXTServiceConfigEnabled *bool `json:"grpcTXTServiceConfigEnabled,omitempty" protobuf:"varint,21,opt,name=grpcTXTServiceConfigEnabled"`
	// K8sClient holds Kubernetes client tuning for the ApplicationSet controller
	// (cmd-params: applicationsetcontroller.k8s.*).
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	K8sClient *K8sClientConfig `json:"k8sClient,omitempty" protobuf:"bytes,22,opt,name=k8sClient"`
	// RepoServerClient holds mTLS cert paths when connecting to the repo-server
	// (cmd-params: applicationsetcontroller.repo.server.*). Named distinctly from
	// spec.repoServer (the repo-server itself) to avoid overloaded keys.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	RepoServerClient *MTLSCertConfig `json:"repoServerClient,omitempty" protobuf:"bytes,23,opt,name=repoServerClient"`
	// Metrics holds ApplicationSet Prometheus metrics listen settings
	// (flags: --metrics-addr, --metrics-applicationset-labels).
	// Migration: if non-nil, takes precedence over ApplicationSet --metrics-addr /
	// --metrics-applicationset-labels as a group. Does not round-trip through ConfigMaps.
	// +optional
	Metrics *ApplicationSetMetricsConfig `json:"metrics,omitempty" protobuf:"bytes,24,opt,name=metrics"`
	// ProbeAddr is the health/readiness probe listen address (flag: --probe-addr).
	// Migration: if set, takes precedence over ApplicationSet --probe-addr. Does not round-trip through ConfigMaps.
	// +optional
	ProbeAddr string `json:"probeAddr,omitempty" protobuf:"bytes,25,opt,name=probeAddr"`
	// WebhookAddr is the webhook listen address (flag: --webhook-addr).
	// Migration: if set, takes precedence over ApplicationSet --webhook-addr. Does not round-trip through ConfigMaps.
	// +optional
	WebhookAddr string `json:"webhookAddr,omitempty" protobuf:"bytes,26,opt,name=webhookAddr"`
}

// ApplicationSetMetricsConfig holds ApplicationSet metrics listen settings (flag-only).
type ApplicationSetMetricsConfig struct {
	// Address is the metrics listen address (flag: --metrics-addr).
	// Migration: if set, takes precedence over ApplicationSet --metrics-addr. Does not round-trip through ConfigMaps.
	// +optional
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address"`
	// ApplicationSetLabels are ApplicationSet label keys exported as Prometheus labels
	// (flag: --metrics-applicationset-labels).
	// Migration: if present, takes precedence over ApplicationSet --metrics-applicationset-labels; replaces the whole collection. Does not round-trip through ConfigMaps.
	// +optional
	ApplicationSetLabels []string `json:"applicationSetLabels,omitempty" protobuf:"bytes,2,rep,name=applicationSetLabels"`
}

// ProgressiveSyncsConfig holds ApplicationSet progressive syncs settings.
type ProgressiveSyncsConfig struct {
	// Enabled enables progressive syncs
	// (cmd-params: applicationsetcontroller.enable.progressive.syncs).
	// Migration: if set, takes precedence over argocd-cmd-params-cm applicationsetcontroller.enable.progressive.syncs.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
}

// GlobalPreservedKeysConfig holds keys preserved on ApplicationSet-generated Applications.
type GlobalPreservedKeysConfig struct {
	// AnnotationKeys are annotation keys preserved on generated Applications
	// (cmd-params: applicationsetcontroller.global.preserved.annotations).
	// Migration: if present, takes precedence over argocd-cmd-params-cm applicationsetcontroller.global.preserved.annotations; replaces the whole collection.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	AnnotationKeys []string `json:"annotationKeys,omitempty" protobuf:"bytes,1,rep,name=annotationKeys"`
	// LabelKeys are label keys preserved on generated Applications
	// (cmd-params: applicationsetcontroller.global.preserved.labels).
	// Migration: if present, takes precedence over argocd-cmd-params-cm applicationsetcontroller.global.preserved.labels; replaces the whole collection.
	// +optional
	// +kubebuilder:validation:items:Pattern=`^([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?/)?([A-Za-z0-9]([-A-Za-z0-9_.]*[A-Za-z0-9])?)$`
	LabelKeys []string `json:"labelKeys,omitempty" protobuf:"bytes,2,rep,name=labelKeys"`
}

// RedisConfig holds shared Redis connection settings.
type RedisConfig struct {
	// Server is the Redis host:port (cmd-params: redis.server). Not a full URL.
	// Migration: if set, takes precedence over argocd-cmd-params-cm redis.server.
	// +optional
	Server string `json:"server,omitempty" protobuf:"bytes,1,opt,name=server"`
	// Compression is the algorithm used for values written to Redis
	// (cmd-params: redis.compression): "gzip" or "none".
	// Migration: if set, takes precedence over argocd-cmd-params-cm redis.compression.
	// +optional
	// +kubebuilder:validation:Enum=gzip;none
	Compression string `json:"compression,omitempty" protobuf:"bytes,2,opt,name=compression"`
	// Sentinel holds Redis Sentinel connection settings (cmd-params: redis.sentinel.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm redis.sentinel.* as a group.
	// +optional
	Sentinel *RedisSentinelConfig `json:"sentinel,omitempty" protobuf:"bytes,3,opt,name=sentinel"`
	// DB is the Redis database number (cmd-params: redis.db).
	// Migration: if set, takes precedence over argocd-cmd-params-cm redis.db.
	// +optional
	// +kubebuilder:validation:Minimum=0
	DB *int32 `json:"db,omitempty" protobuf:"varint,4,opt,name=db"`
	// KeyPrefix is prepended to Redis keys (cmd-params: redis.key.prefix).
	// Migration: if set, takes precedence over argocd-cmd-params-cm redis.key.prefix.
	// +optional
	KeyPrefix string `json:"keyPrefix,omitempty" protobuf:"bytes,5,opt,name=keyPrefix"`
}

// RedisSentinelConfig holds Redis Sentinel connection settings.
type RedisSentinelConfig struct {
	// Hosts are Redis Sentinel addresses (cmd-params: redis.sentinel.hosts).
	// Migration: if present, takes precedence over argocd-cmd-params-cm redis.sentinel.hosts; replaces the whole collection.
	// +optional
	Hosts []string `json:"hosts,omitempty" protobuf:"bytes,1,rep,name=hosts"`
	// Master is the Sentinel master group name (cmd-params: redis.sentinel.master).
	// Migration: if set, takes precedence over argocd-cmd-params-cm redis.sentinel.master.
	// +optional
	Master string `json:"master,omitempty" protobuf:"bytes,2,opt,name=master"`
}

// OTLPConfig holds OpenTelemetry collector settings shared across components.
type OTLPConfig struct {
	// Address is the collector host:port (cmd-params: otlp.address), e.g. "otel:4317".
	// Migration: if set, takes precedence over argocd-cmd-params-cm otlp.address.
	// +optional
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address"`
	// TLSEnabled enables TLS when exporting to the collector (cmd-params: otlp.insecure —
	// inverted: insecure=true means TLSEnabled=false). Default legacy behavior is insecure.
	// Migration: if set, takes precedence over argocd-cmd-params-cm otlp.insecure (inverted).
	// +optional
	TLSEnabled *bool `json:"tlsEnabled,omitempty" protobuf:"varint,2,opt,name=tlsEnabled"`
	// Headers are extra headers sent to the collector (cmd-params: otlp.headers),
	// historically "key=value,key2=value2".
	// Migration: if present, takes precedence over argocd-cmd-params-cm otlp.headers; replaces the whole collection.
	// +optional
	Headers map[string]string `json:"headers,omitempty" protobuf:"bytes,3,rep,name=headers"`
	// Attrs are resource attributes attached to spans (cmd-params: otlp.attrs),
	// historically "key:value,key2:value2".
	// Migration: if present, takes precedence over argocd-cmd-params-cm otlp.attrs; replaces the whole collection.
	// +optional
	Attrs map[string]string `json:"attrs,omitempty" protobuf:"bytes,4,rep,name=attrs"`
	// SampleRatio is the trace sampling ratio between 0.0 and 1.0
	// (cmd-params: otlp.sample.ratio). Stored as a decimal string because CRDs
	// discourage float types (cross-language JSON number issues).
	// Migration: if set, takes precedence over argocd-cmd-params-cm otlp.sample.ratio.
	// +optional
	// +kubebuilder:validation:Pattern=`^(0(\.[0-9]+)?|1(\.0+)?)$`
	SampleRatio string `json:"sampleRatio,omitempty" protobuf:"bytes,5,opt,name=sampleRatio"`
}

// LoggingConfig holds cross-component logging options.
type LoggingConfig struct {
	// FormatTimestamp is the shared log timestamp format for all components
	// (cmd-params: log.format.timestamp). Empty means RFC3339.
	// See https://pkg.go.dev/time#pkg-constants.
	// Migration: if set, takes precedence over argocd-cmd-params-cm log.format.timestamp.
	// +optional
	FormatTimestamp string `json:"formatTimestamp,omitempty" protobuf:"bytes,1,opt,name=formatTimestamp"`
}

// LogConfig holds per-component log format and level (cmd-params: *.log.format / *.log.level).
type LogConfig struct {
	// Format is the log format (cmd-params: *.log.format).
	// Migration: if set, takes precedence over the corresponding argocd-cmd-params-cm *.log.format key.
	// +optional
	// +kubebuilder:validation:Enum=json;text
	Format string `json:"format,omitempty" protobuf:"bytes,1,opt,name=format"`
	// Level is the log level (cmd-params: *.log.level).
	// Migration: if set, takes precedence over the corresponding argocd-cmd-params-cm *.log.level key.
	// +optional
	// +kubebuilder:validation:Enum=debug;info;warn;error
	Level string `json:"level,omitempty" protobuf:"bytes,2,opt,name=level"`
}

// KustomizeConfig holds Kustomize tool settings (argocd-cm: kustomize.*).
type KustomizeConfig struct {
	// Enabled enables Kustomize as a manifest source tool (argocd-cm: kustomize.enable).
	// Migration: if set, takes precedence over argocd-cm kustomize.enable.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
	// BuildOptions are default options passed to `kustomize build`
	// (argocd-cm: kustomize.buildOptions).
	// Migration: if set, takes precedence over argocd-cm kustomize.buildOptions.
	// +optional
	BuildOptions string `json:"buildOptions,omitempty" protobuf:"bytes,2,opt,name=buildOptions"`
	// Versions lists alternate Kustomize binaries and per-version build options
	// (argocd-cm: kustomize.path.* / kustomize.buildOptions.*).
	// Applications select a version via source.kustomize.version.
	// +optional
	// +listType=map
	// +listMapKey=name
	Versions []KustomizeVersion `json:"versions,omitempty" protobuf:"bytes,3,rep,name=versions"`
}

// HelmConfig holds Helm tool settings (argocd-cm: helm.* and cmd-params: reposerver.helm.*).
// Organizational subgroup: children migrate independently.
type HelmConfig struct {
	// Enabled enables Helm as a manifest source tool (argocd-cm: helm.enable).
	// Migration: if set, takes precedence over argocd-cm helm.enable.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
	// ValuesFileSchemes are URI schemes allowed for remote Helm values files
	// (argocd-cm: helm.valuesFileSchemes), e.g. "https", "http".
	// Migration: if present, takes precedence over argocd-cm helm.valuesFileSchemes; replaces the whole collection.
	// +optional
	ValuesFileSchemes []string `json:"valuesFileSchemes,omitempty" protobuf:"bytes,2,rep,name=valuesFileSchemes"`
	// UserAgent is the User-Agent header for Helm registry/HTTP requests
	// (cmd-params: reposerver.helm.user.agent).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.helm.user.agent.
	// +optional
	UserAgent string `json:"userAgent,omitempty" protobuf:"bytes,3,opt,name=userAgent"`
	// Manifest holds Helm chart extracted-size limits (cmd-params: reposerver.helm.manifest.max.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.helm.manifest.max.* as a group.
	// +optional
	Manifest *HelmManifestConfig `json:"manifest,omitempty" protobuf:"bytes,4,opt,name=manifest"`
}

// HelmManifestConfig holds Helm chart extracted-size limits.
type HelmManifestConfig struct {
	// MaxExtractedSize caps extracted Helm chart size
	// (cmd-params: reposerver.helm.manifest.max.extracted.size).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.helm.manifest.max.extracted.size.
	// +optional
	MaxExtractedSize *resource.Quantity `json:"maxExtractedSize,omitempty" protobuf:"bytes,1,opt,name=maxExtractedSize"`
	// MaxExtractedSizeEnabled controls the extracted size limit
	// (cmd-params: reposerver.disable.helm.manifest.max.extracted.size —
	// inverted: disable...=true means enabled=false). Limit is on by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.disable.helm.manifest.max.extracted.size (inverted).
	// +optional
	MaxExtractedSizeEnabled *bool `json:"maxExtractedSizeEnabled,omitempty" protobuf:"varint,2,opt,name=maxExtractedSizeEnabled"`
}

// UIConfig holds web UI customization (argocd-cm: ui.*).
// Organizational subgroup: children migrate independently.
type UIConfig struct {
	// CSSURL is a local path (relative to /shared/app) or remote http(s) URL for custom CSS
	// (argocd-cm: ui.cssurl). Not CEL-validated as an absolute URL — relative paths are valid.
	// Migration: if set, takes precedence over argocd-cm ui.cssurl.
	// +optional
	CSSURL string `json:"cssURL,omitempty" protobuf:"bytes,1,opt,name=cssURL"`
	// Banner holds the UI banner message, link, and display options (argocd-cm: ui.banner*).
	// Migration: if non-nil, takes precedence over argocd-cm ui.banner* as a group.
	// +optional
	Banner *UIBannerConfig `json:"banner,omitempty" protobuf:"bytes,2,opt,name=banner"`
	// LoginButtonText overrides the SSO login button label
	// (argocd-cm: ui.loginButtonText).
	// Migration: if set, takes precedence over argocd-cm ui.loginButtonText.
	// +optional
	LoginButtonText string `json:"loginButtonText,omitempty" protobuf:"bytes,3,opt,name=loginButtonText"`
}

// UIBannerConfig holds UI banner display settings.
type UIBannerConfig struct {
	// Content is the banner message shown in the UI (argocd-cm: ui.bannercontent).
	// Migration: if set, takes precedence over argocd-cm ui.bannercontent.
	// +optional
	Content string `json:"content,omitempty" protobuf:"bytes,1,opt,name=content"`
	// URL is an optional link target for the banner text (argocd-cm: ui.bannerurl).
	// Migration: if set, takes precedence over argocd-cm ui.bannerurl.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^https?://.+`
	URL string `json:"url,omitempty" protobuf:"bytes,2,opt,name=url"`
	// Permanent makes the banner non-dismissible (argocd-cm: ui.bannerpermanent).
	// Migration: if set, takes precedence over argocd-cm ui.bannerpermanent.
	// +optional
	Permanent *bool `json:"permanent,omitempty" protobuf:"varint,3,opt,name=permanent"`
	// Position is where the banner appears: top, bottom, or both (argocd-cm: ui.bannerposition).
	// Migration: if set, takes precedence over argocd-cm ui.bannerposition.
	// +optional
	// +kubebuilder:validation:Enum=top;bottom;both
	Position string `json:"position,omitempty" protobuf:"bytes,4,opt,name=position"`
}

// AccountConfig configures a local (non-SSO) user account (argocd-cm: accounts.*).
// The built-in admin user is also represented here (legacy admin.enabled).
type AccountConfig struct {
	// Name is the local account username (max 32 characters in Argo CD),
	// e.g. "admin", "ci-bot". Legacy: suffix of accounts.<name>.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Enabled enables or disables the account (argocd-cm: accounts.<name>.enabled
	// or admin.enabled for the built-in admin user). Accounts are enabled by
	// default when the accounts.<name> entry exists.
	// Migration: if set, takes precedence over argocd-cm accounts.
	// +optional
	Enabled bool `json:"enabled,omitempty" protobuf:"varint,2,opt,name=enabled"`
	// Capabilities is the set of actions the account may perform: "login" (UI)
	// and/or "apiKey" (generate tokens). Legacy: comma-separated value of
	// accounts.<name> (e.g. "apiKey, login"). Entries must be unique.
	// Migration: if present, takes precedence over argocd-cm accounts.<name> capabilities; replaces the whole collection.
	// +optional
	// +listType=set
	// +kubebuilder:validation:items:Enum=login;apiKey
	Capabilities []string `json:"capabilities,omitempty" protobuf:"bytes,3,rep,name=capabilities"`
}

// ExtensionConfig configures one UI proxy extension (argocd-cm: extension.config).
// Requires server.proxyExtensionEnabled (cmd-params: server.enable.proxy.extension).
// Transport fields are hoisted onto the extension (legacy CM still nests them under backend).
type ExtensionConfig struct {
	// Name is the extension identifier; registers the proxy route at
	// <argocd-host>/api/v1/extensions/<name> and is used in RBAC.
	// Legacy: extension.config extensions[].name.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Services are upstream backends the proxy may forward to (optionally
	// filtered by Application destination cluster).
	// Legacy: extension.config extensions[].backend.services.
	// +optional
	// +listType=map
	// +listMapKey=url
	Services []ExtensionService `json:"services,omitempty" protobuf:"bytes,2,rep,name=services"`
	// ConnectionTimeout is the max dial time to the upstream extension server
	// (legacy: backend.connectionTimeout / backend.transport.connectionTimeout; default 2s).
	// +optional
	ConnectionTimeout *metav1.Duration `json:"connectionTimeout,omitempty" protobuf:"bytes,3,opt,name=connectionTimeout"`
	// KeepAlive is the HTTP keep-alive probe interval for upstream connections
	// (legacy: backend.keepAlive; default 15s).
	// +optional
	KeepAlive *metav1.Duration `json:"keepAlive,omitempty" protobuf:"bytes,4,opt,name=keepAlive"`
	// IdleConnectionTimeout is how long idle upstream connections are kept
	// (legacy: backend.idleConnectionTimeout; default 60s).
	// +optional
	IdleConnectionTimeout *metav1.Duration `json:"idleConnectionTimeout,omitempty" protobuf:"bytes,5,opt,name=idleConnectionTimeout"`
	// MaxIdleConnections is the max idle connections in the upstream pool
	// (legacy: backend.maxIdleConnections; default 30).
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxIdleConnections int32 `json:"maxIdleConnections,omitempty" protobuf:"varint,6,opt,name=maxIdleConnections"`
}

// ExtensionService is one upstream URL for an extension.
type ExtensionService struct {
	// URL is the upstream extension service base URL (mandatory).
	// Legacy: extension.config ...services[].url.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^https?://.+`
	URL string `json:"url" protobuf:"bytes,1,opt,name=url"`
	// Cluster optionally restricts this backend to Applications whose destination
	// matches. Required when multiple services are configured; ignored when only
	// one service exists. Legacy: ...services[].cluster.
	// +optional
	Cluster *ExtensionCluster `json:"cluster,omitempty" protobuf:"bytes,2,opt,name=cluster"`
	// Headers are extra HTTP headers injected on outgoing proxy requests
	// (override same-named incoming headers; reserved names are ignored).
	// Values may use $secret.key for argocd-secret lookup.
	// Legacy: ...services[].headers.
	// +optional
	// +listType=map
	// +listMapKey=name
	Headers []ExtensionHeader `json:"headers,omitempty" protobuf:"bytes,3,rep,name=headers"`
}

// ExtensionCluster selects a destination cluster for an extension backend.
type ExtensionCluster struct {
	// ServerURL is the cluster API server URL to match against Application
	// destination.server (legacy: cluster.server).
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^[a-zA-Z][a-zA-Z0-9+.-]*:.+`
	ServerURL string `json:"serverURL,omitempty" protobuf:"bytes,1,opt,name=serverURL"`
	// Name is the Argo CD cluster name to match against Application
	// destination.name (legacy: cluster.name).
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,2,opt,name=name"`
}

// ExtensionHeader is an HTTP header injected into extension proxy requests.
type ExtensionHeader struct {
	// Name is the HTTP header name (legacy: headers[].name).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Value is the header value; may include $secret.key interpolation
	// (legacy: headers[].value).
	// +kubebuilder:validation:Required
	Value string `json:"value" protobuf:"bytes,2,opt,name=value"`
}

// GlobalProjectConfig applies a global AppProject's defaults to matching projects
// (argocd-cm: globalProjects). Inherited settings include namespace/cluster resource
// black/whitelists, syncWindows, sourceRepos, destinations, and
// destinationServiceAccounts.
type GlobalProjectConfig struct {
	// ProjectName is the name of the global AppProject to apply
	// (legacy: globalProjects[].projectName). Required as a listMapKey.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	ProjectName string `json:"projectName" protobuf:"bytes,1,opt,name=projectName"`
	// LabelSelector selects which AppProjects receive this global project's settings
	// (In, NotIn, Exists, DoesNotExist). Legacy: globalProjects[].labelSelector.
	// +optional
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty" protobuf:"bytes,2,opt,name=labelSelector"`
}

// DeepLinksConfig holds custom deep links for application, project, and resource views
// (argocd-cm: application.links / project.links / resource.links).
// See docs/operator-manual/deep_links.md in argo-cd.
// Organizational subgroup: each link collection migrates independently.
type DeepLinksConfig struct {
	// Application links appear in the application summary tab
	// (argocd-cm: application.links). Template context: app/application, cluster.
	// Migration: if present, takes precedence over argocd-cm application.links; replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=title
	Application []DeepLink `json:"application,omitempty" protobuf:"bytes,1,rep,name=application"`
	// Project links appear in the project tab (argocd-cm: project.links).
	// Template context: project.
	// Migration: if present, takes precedence over argocd-cm project.links; replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=title
	Project []DeepLink `json:"project,omitempty" protobuf:"bytes,2,rep,name=project"`
	// Resource links appear in the resource summary tab (argocd-cm: resource.links).
	// Template context: resource, application, cluster, project. Secret data fields
	// are redacted but other fields remain accessible for templating.
	// Migration: if present, takes precedence over argocd-cm resource.links; replaces the whole collection.
	// +optional
	// +listType=map
	// +listMapKey=title
	Resource []DeepLink `json:"resource,omitempty" protobuf:"bytes,3,rep,name=resource"`
}

// DeepLink is one custom UI deep link. URLTemplate and ConditionExpr may use
// Go templates / expr-lang expressions evaluated against application/project/resource context.
type DeepLink struct {
	// URLTemplate is the link target and may include Go text/template expressions
	// (e.g. {{.app.spec.destination.namespace}}); not CEL-validated as a plain URL.
	// Legacy: <location>.links[].url.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	URLTemplate string `json:"urlTemplate" protobuf:"bytes,1,opt,name=urlTemplate"`
	// Title is the link text / tag shown in the UI (legacy: title).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Title string `json:"title" protobuf:"bytes,2,opt,name=title"`
	// Description is optional explanatory text for the link (legacy: description).
	// +optional
	Description string `json:"description,omitempty" protobuf:"bytes,3,opt,name=description"`
	// IconClass is an optional Font Awesome CSS class for dropdown display
	// (legacy: icon.class).
	// +optional
	IconClass string `json:"iconClass,omitempty" protobuf:"bytes,4,opt,name=iconClass"`
	// ConditionExpr is an expr-lang expression; the link is shown only when true
	// (legacy: if). Omitted = always show.
	// +optional
	ConditionExpr string `json:"conditionExpr,omitempty" protobuf:"bytes,5,opt,name=conditionExpr"`
}

// HelpConfig configures the UI help panel and CLI download links.
// Organizational subgroup: children migrate independently.
type HelpConfig struct {
	// Chat holds support chat / community link settings (argocd-cm: help.chatUrl, help.chatText).
	// Migration: if non-nil, takes precedence over argocd-cm help.chat* as a group.
	// +optional
	Chat *HelpChatConfig `json:"chat,omitempty" protobuf:"bytes,1,opt,name=chat"`
	// BinaryURLs maps architecture name to a CLI binary download path or URL
	// (argocd-cm: help.download.<arch>). Values may be absolute http(s) URLs or paths —
	// not CEL-validated as absolute URLs.
	// Migration: if present, takes precedence over argocd-cm help.download.*; replaces the whole collection.
	// +optional
	BinaryURLs map[string]string `json:"binaryURLs,omitempty" protobuf:"bytes,2,rep,name=binaryURLs"`
}

// HelpChatConfig holds support chat link settings.
type HelpChatConfig struct {
	// URL is the support chat / community link (argocd-cm: help.chatUrl).
	// Migration: if set, takes precedence over argocd-cm help.chatUrl.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^https?://.+`
	URL string `json:"url,omitempty" protobuf:"bytes,1,opt,name=url"`
	// Text is the display text for URL (argocd-cm: help.chatText).
	// Migration: if set, takes precedence over argocd-cm help.chatText.
	// +optional
	Text string `json:"text,omitempty" protobuf:"bytes,2,opt,name=text"`
}

// UsersConfig holds anonymous access, session, and password policy settings.
// Organizational subgroup: children migrate independently.
type UsersConfig struct {
	// AnonymousEnabled allows unauthenticated access with the default RBAC role
	// (argocd-cm: users.anonymous.enabled).
	// Migration: if set, takes precedence over argocd-cm users.anonymous.enabled.
	// +optional
	AnonymousEnabled *bool `json:"anonymousEnabled,omitempty" protobuf:"varint,1,opt,name=anonymousEnabled"`
	// SessionDuration is how long user sessions / tokens remain valid
	// (argocd-cm: users.session.duration). Default is typically 24h.
	// Migration: if set, takes precedence over argocd-cm users.session.duration.
	// +optional
	SessionDuration *metav1.Duration `json:"sessionDuration,omitempty" protobuf:"bytes,2,opt,name=sessionDuration"`
	// PasswordRegex is a Go regexp that local account passwords must match
	// (argocd-cm: passwordPattern).
	// Migration: if set, takes precedence over argocd-cm passwordPattern.
	// +optional
	PasswordRegex string `json:"passwordRegex,omitempty" protobuf:"bytes,3,opt,name=passwordRegex"`
}

// GoogleAnalyticsConfig holds optional Google Analytics settings.
type GoogleAnalyticsConfig struct {
	// TrackingID is the Google Analytics tracking / measurement ID
	// (argocd-cm: ga.trackingid).
	// Migration: if set, takes precedence over argocd-cm ga.trackingid.
	// +optional
	TrackingID string `json:"trackingID,omitempty" protobuf:"bytes,1,opt,name=trackingID"`
	// AnonymizeUsers anonymizes user identifiers sent to Google Analytics
	// (argocd-cm: ga.anonymizeusers).
	// Migration: if set, takes precedence over argocd-cm ga.anonymizeusers.
	// +optional
	AnonymizeUsers bool `json:"anonymizeUsers,omitempty" protobuf:"varint,2,opt,name=anonymizeUsers"`
}

// SourceHydratorConfig configures the beta manifest source hydrator.
type SourceHydratorConfig struct {
	// Enabled turns on the manifest hydrator feature (cmd-params: hydrator.enabled).
	// Migration: if set, takes precedence over argocd-cmd-params-cm hydrator.enabled.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
	// CommitMessageTemplate is a Go template for commits created by the hydrator
	// (argocd-cm: sourceHydrator.commitMessageTemplate).
	// Migration: if set, takes precedence over argocd-cm sourceHydrator.commitMessageTemplate.
	// +optional
	CommitMessageTemplate string `json:"commitMessageTemplate,omitempty" protobuf:"bytes,2,opt,name=commitMessageTemplate"`
	// ReadmeMessageTemplate is a Go template for the hydrator-generated README.md
	// (argocd-cm: sourceHydrator.readmeMessageTemplate).
	// Migration: if set, takes precedence over argocd-cm sourceHydrator.readmeMessageTemplate.
	// +optional
	ReadmeMessageTemplate string `json:"readmeMessageTemplate,omitempty" protobuf:"bytes,3,opt,name=readmeMessageTemplate"`
}

// CommitConfig holds commit identity settings for the hydrator/commit-server
// (argocd-cm: commit.author.*).
type CommitConfig struct {
	// Author is the git author used for hydrator-created commits
	// (argocd-cm: commit.author.name / commit.author.email).
	// +optional
	Author *CommitAuthor `json:"author,omitempty" protobuf:"bytes,1,opt,name=author"`
}

// CommitAuthor is the commit author identity (argocd-cm: commit.author.*).
type CommitAuthor struct {
	// Name is the git author name (argocd-cm: commit.author.name).
	// Migration: if set, takes precedence over argocd-cm commit.author.name.
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,1,opt,name=name"`
	// Email is the git author email (argocd-cm: commit.author.email).
	// Migration: if set, takes precedence over argocd-cm commit.author.email.
	// +optional
	// +kubebuilder:validation:Format=email
	Email string `json:"email,omitempty" protobuf:"bytes,2,opt,name=email"`
}

// WebhookConfig holds SCM webhook payload limits, refresh jitter, and server workers.
// Organizational subgroup: children migrate independently (argocd-cm + cmd-params).
type WebhookConfig struct {
	// MaxPayloadSize is the maximum webhook HTTP body size (argocd-cm:
	// webhook.maxPayloadSizeMB). Use a Kubernetes resource.Quantity (e.g. "50M").
	// Legacy CM values in megabytes map to/from decimal megabytes.
	// Migration: if set, takes precedence over argocd-cm webhook.maxPayloadSizeMB.
	// +optional
	MaxPayloadSize *resource.Quantity `json:"maxPayloadSize,omitempty" protobuf:"bytes,1,opt,name=maxPayloadSize"`
	// Refresh holds webhook-triggered refresh jitter and worker settings.
	// Migration: if non-nil, takes precedence over argocd-cm webhook.refresh.* and
	// argocd-cmd-params-cm server.webhook.refresh.workers as a group.
	// +optional
	Refresh *WebhookRefreshConfig `json:"refresh,omitempty" protobuf:"bytes,2,opt,name=refresh"`
	// ParallelismLimit caps concurrent webhook processing on the API server
	// (cmd-params: server.webhook.parallelism.limit).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.webhook.parallelism.limit.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ParallelismLimit *int32 `json:"parallelismLimit,omitempty" protobuf:"varint,3,opt,name=parallelismLimit"`
}

// WebhookRefreshConfig holds webhook refresh jitter and worker settings.
type WebhookRefreshConfig struct {
	// Jitter is the maximum random delay before webhook-triggered app refreshes
	// (argocd-cm: webhook.refresh.jitter), spreading load.
	// Migration: if set, takes precedence over argocd-cm webhook.refresh.jitter.
	// +optional
	Jitter *metav1.Duration `json:"jitter,omitempty" protobuf:"bytes,1,opt,name=jitter"`
	// JitterThreshold is the minimum number of affected apps before jitter is applied
	// (argocd-cm: webhook.refresh.jitter.threshold).
	// Migration: if set, takes precedence over argocd-cm webhook.refresh.jitter.threshold.
	// +optional
	// +kubebuilder:validation:Minimum=0
	JitterThreshold *int32 `json:"jitterThreshold,omitempty" protobuf:"varint,2,opt,name=jitterThreshold"`
	// Workers is the number of workers handling webhook refreshes
	// (cmd-params: server.webhook.refresh.workers).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.webhook.refresh.workers.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Workers *int32 `json:"workers,omitempty" protobuf:"varint,3,opt,name=workers"`
}

// ExecConfig configures the UI pod terminal (exec) feature.
type ExecConfig struct {
	// Enabled turns on pod exec from the UI (argocd-cm: exec.enabled).
	// Migration: if set, takes precedence over argocd-cm exec.enabled.
	// +optional
	Enabled bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
	// Shells are preferred shells tried in order for exec sessions
	// (argocd-cm: exec.shells), e.g. bash, sh, powershell, cmd.
	// Migration: if present, takes precedence over argocd-cm exec.shells; replaces the whole collection.
	// +optional
	Shells []string `json:"shells,omitempty" protobuf:"bytes,2,rep,name=shells"`
}

// ServerLogsConfig holds UI pod-log viewing settings.
type ServerLogsConfig struct {
	// MaxPodsToRender is the maximum number of pod log lines the UI renders
	// before truncation (argocd-cm: server.maxPodLogsToRender).
	// Migration: if set, takes precedence over argocd-cm server.maxPodLogsToRender.
	// +optional
	// +kubebuilder:validation:Minimum=0
	MaxPodsToRender *int64 `json:"maxPodsToRender,omitempty" protobuf:"varint,1,opt,name=maxPodsToRender"`
}

// StatusBadgeConfig configures application/project status badges.
type StatusBadgeConfig struct {
	// Enabled turns on status badge endpoints (argocd-cm: statusbadge.enabled).
	// Migration: if set, takes precedence over argocd-cm statusbadge.enabled.
	// +optional
	Enabled bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
	// URL is an optional custom root URL for status badge links
	// (argocd-cm: statusbadge.url).
	// Migration: if set, takes precedence over argocd-cm statusbadge.url.
	// +optional
	// +kubebuilder:validation:MaxLength=2048
	// +kubebuilder:validation:Pattern=`^$|^https?://.+`
	URL string `json:"url,omitempty" protobuf:"bytes,2,opt,name=url"`
}

// DexConnectionConfig is how the API server reaches Dex (cmd-params: server.dex.server*).
type DexConnectionConfig struct {
	// Address is the Dex server host:port (cmd-params: server.dex.server).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.dex.server.
	// +optional
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address"`
	// TLSEnabled controls TLS when connecting to Dex (cmd-params: server.dex.server.plaintext —
	// inverted: plaintext=true means tlsEnabled=false).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.dex.server.plaintext (inverted).
	// +optional
	TLSEnabled *bool `json:"tlsEnabled,omitempty" protobuf:"varint,2,opt,name=tlsEnabled"`
	// InsecureSkipVerify skips verification of the Dex TLS certificate
	// (cmd-params: server.dex.server.strict.tls — inverted: strict.tls=true means skip=false).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.dex.server.strict.tls (inverted).
	// +optional
	InsecureSkipVerify *bool `json:"insecureSkipVerify,omitempty" protobuf:"varint,3,opt,name=insecureSkipVerify"`
}

// ServerCacheConfig holds API server cache TTLs and sizes.
// Organizational subgroup: children migrate independently.
type ServerCacheConfig struct {
	// AppStateExpiration is the app state cache TTL (cmd-params: server.app.state.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.app.state.cache.expiration.
	// +optional
	AppStateExpiration *metav1.Duration `json:"appStateExpiration,omitempty" protobuf:"bytes,1,opt,name=appStateExpiration"`
	// ConnectionStatusExpiration is the connection status cache TTL
	// (cmd-params: server.connection.status.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.connection.status.cache.expiration.
	// +optional
	ConnectionStatusExpiration *metav1.Duration `json:"connectionStatusExpiration,omitempty" protobuf:"bytes,2,opt,name=connectionStatusExpiration"`
	// DefaultExpiration is the default Redis cache TTL (cmd-params: server.default.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.default.cache.expiration.
	// +optional
	DefaultExpiration *metav1.Duration `json:"defaultExpiration,omitempty" protobuf:"bytes,3,opt,name=defaultExpiration"`
	// OIDCExpiration is the OIDC cache TTL (cmd-params: server.oidc.cache.expiration).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.oidc.cache.expiration.
	// +optional
	OIDCExpiration *metav1.Duration `json:"oidcExpiration,omitempty" protobuf:"bytes,4,opt,name=oidcExpiration"`
	// GlobCacheSize is the glob matcher cache size (cmd-params: server.glob.cache.size).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.glob.cache.size.
	// +optional
	// +kubebuilder:validation:Minimum=0
	GlobCacheSize *int32 `json:"globCacheSize,omitempty" protobuf:"varint,5,opt,name=globCacheSize"`
}

// TLSVersionConfig holds TLS min/max version and cipher suites for a component
// (cmd-params: server.tls.* / reposerver.tls.*).
type TLSVersionConfig struct {
	// MinVersion is the minimum accepted TLS version: "1.0", "1.1", "1.2", or "1.3"
	// (cmd-params: *.tls.minversion). Default "1.2".
	// +optional
	// +kubebuilder:validation:Enum="1.0";"1.1";"1.2";"1.3"
	MinVersion string `json:"minVersion,omitempty" protobuf:"bytes,1,opt,name=minVersion"`
	// MaxVersion is the maximum accepted TLS version: "1.0", "1.1", "1.2", or "1.3"
	// (cmd-params: *.tls.maxversion). Default "1.3".
	// +optional
	// +kubebuilder:validation:Enum="1.0";"1.1";"1.2";"1.3"
	MaxVersion string `json:"maxVersion,omitempty" protobuf:"bytes,2,opt,name=maxVersion"`
	// Ciphers lists allowed TLS cipher suite names
	// (cmd-params: *.tls.ciphers). Default typically
	// TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384; use "list" with the Argo CD CLI
	// to enumerate available ciphers.
	// +optional
	Ciphers []string `json:"ciphers,omitempty" protobuf:"bytes,3,rep,name=ciphers"`
}

// ListenConfig holds HTTP listen and metrics listen host addresses
// (cmd-params: *.listen.address / *.metrics.listen.address). Bind host only —
// not host:port. Server listen ports live on ServerListenConfig.
type ListenConfig struct {
	// Address is the primary listen host (cmd-params: *.listen.address), e.g. "0.0.0.0".
	// Migration: if set, takes precedence over the component's argocd-cmd-params-cm *.listen.address key.
	// +optional
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address"`
	// MetricsAddress is the metrics listen host (cmd-params: *.metrics.listen.address).
	// Migration: if set, takes precedence over the component's argocd-cmd-params-cm *.metrics.listen.address key.
	// +optional
	MetricsAddress string `json:"metricsAddress,omitempty" protobuf:"bytes,2,opt,name=metricsAddress"`
}

// ServerListenConfig holds argocd-server listen host and port settings.
// Organizational subgroup: children migrate independently (cmd-params hosts vs flag ports).
type ServerListenConfig struct {
	// Address is the API server listen host (cmd-params: server.listen.address), e.g. "0.0.0.0".
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.listen.address.
	// +optional
	Address string `json:"address,omitempty" protobuf:"bytes,1,opt,name=address"`
	// Port is the API server listen port (flag: --port).
	// Migration: if set, takes precedence over server --port. Does not round-trip through ConfigMaps.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port *int32 `json:"port,omitempty" protobuf:"varint,2,opt,name=port"`
	// MetricsAddress is the metrics listen host (cmd-params: server.metrics.listen.address).
	// Migration: if set, takes precedence over argocd-cmd-params-cm server.metrics.listen.address.
	// +optional
	MetricsAddress string `json:"metricsAddress,omitempty" protobuf:"bytes,3,opt,name=metricsAddress"`
	// MetricsPort is the metrics listen port (flag: --metrics-port).
	// Migration: if set, takes precedence over server --metrics-port. Does not round-trip through ConfigMaps.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	MetricsPort *int32 `json:"metricsPort,omitempty" protobuf:"varint,4,opt,name=metricsPort"`
}

// K8sClientConfig is an alias for the protobuf-generated K8SClientConfig type.
type K8sClientConfig = K8SClientConfig

// K8sClientTCPConfig is an alias for the protobuf-generated K8SClientTCPConfig type.
type K8sClientTCPConfig = K8SClientTCPConfig

// ClientRetryConfig is retry policy for a Kubernetes API client.
// Organizational subgroup: max attempts and backoff migrate independently.
type ClientRetryConfig struct {
	// Max is the maximum number of client retries
	// (cmd-params: *.k8sclient.retry.max). Default 0 — only the initial attempt.
	// Migration: if set, takes precedence over the component *.k8sclient.retry.max key.
	// +optional
	// +kubebuilder:validation:Minimum=0
	Max *int32 `json:"max,omitempty" protobuf:"varint,1,opt,name=max"`
	// Backoff is the wait between retries. Only Duration is used by current
	// Argo CD cmd-params (*.k8sclient.retry.base.backoff; default 100ms);
	// Factor/MaxDuration are reserved for a consistent BackoffConfig shape.
	// Delay doubles each retry and is hard-capped at 10s in Argo CD.
	// Migration: if non-nil, takes precedence over the component *.k8sclient.retry.base.backoff key via backoff.duration.
	// +optional
	Backoff *BackoffConfig `json:"backoff,omitempty" protobuf:"bytes,2,opt,name=backoff"`
}

// ControllerClusterCacheConfig tunes cluster informer event batching.
type ControllerClusterCacheConfig struct {
	// BatchEventsProcessing enables batching of cluster cache events
	// (cmd-params: controller.cluster.cache.batch.events.processing).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.cluster.cache.batch.events.processing.
	// +optional
	BatchEventsProcessing *bool `json:"batchEventsProcessing,omitempty" protobuf:"varint,1,opt,name=batchEventsProcessing"`
	// EventsProcessingInterval is how often batched events are processed
	// (cmd-params: controller.cluster.cache.events.processing.interval).
	// Migration: if set, takes precedence over argocd-cmd-params-cm controller.cluster.cache.events.processing.interval.
	// +optional
	EventsProcessingInterval *metav1.Duration `json:"eventsProcessingInterval,omitempty" protobuf:"bytes,2,opt,name=eventsProcessingInterval"`
}

// RepoServerOCIConfig holds OCI pull size and media-type limits.
// Organizational subgroup: children migrate independently.
type RepoServerOCIConfig struct {
	// Manifest holds OCI manifest size limits (cmd-params: reposerver.oci.manifest.max.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm reposerver.oci.manifest.max.* as a group.
	// +optional
	Manifest *OCIManifestConfig `json:"manifest,omitempty" protobuf:"bytes,1,opt,name=manifest"`
	// LayerMediaTypes lists allowed OCI layer media types
	// (cmd-params: reposerver.oci.layer.media.types).
	// Migration: if present, takes precedence over argocd-cmd-params-cm reposerver.oci.layer.media.types; replaces the whole collection.
	// +optional
	LayerMediaTypes []string `json:"layerMediaTypes,omitempty" protobuf:"bytes,2,rep,name=layerMediaTypes"`
}

// OCIManifestConfig holds OCI manifest size limits.
type OCIManifestConfig struct {
	// MaxExtractedSize caps extracted OCI manifest size
	// (cmd-params: reposerver.oci.manifest.max.extracted.size).
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.oci.manifest.max.extracted.size.
	// +optional
	MaxExtractedSize *resource.Quantity `json:"maxExtractedSize,omitempty" protobuf:"bytes,1,opt,name=maxExtractedSize"`
	// MaxExtractedSizeEnabled controls the extracted size limit
	// (cmd-params: reposerver.disable.oci.manifest.max.extracted.size —
	// inverted: disable...=true means enabled=false). Limit is on by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm reposerver.disable.oci.manifest.max.extracted.size (inverted).
	// +optional
	MaxExtractedSizeEnabled *bool `json:"maxExtractedSizeEnabled,omitempty" protobuf:"varint,2,opt,name=maxExtractedSizeEnabled"`
}

// JsonnetConfig holds Jsonnet tool settings (argocd-cm: jsonnet.*).
type JsonnetConfig struct {
	// Enabled enables Jsonnet as a manifest source tool (argocd-cm: jsonnet.enable).
	// Migration: if set, takes precedence over argocd-cm jsonnet.enable.
	// +optional
	Enabled *bool `json:"enabled,omitempty" protobuf:"varint,1,opt,name=enabled"`
}

// DexServerConfig holds Dex server process runtime settings (cmd-params: dexserver.*).
type DexServerConfig struct {
	// Log holds Dex server log format and level (cmd-params: dexserver.log.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm dexserver.log.* as a group.
	// +optional
	Log *LogConfig `json:"log,omitempty" protobuf:"bytes,1,opt,name=log"`
	// TLSEnabled controls whether the Dex server serves TLS (cmd-params: dexserver.disable.tls —
	// inverted: disable.tls=true means tlsEnabled=false). TLS is on by default.
	// Migration: if set, takes precedence over argocd-cmd-params-cm dexserver.disable.tls (inverted).
	// +optional
	TLSEnabled *bool `json:"tlsEnabled,omitempty" protobuf:"varint,2,opt,name=tlsEnabled"`
	// ConnectorFailureContinue continues serving when a connector fails to initialize
	// (cmd-params: dexserver.connector.failure.continue).
	// Migration: if set, takes precedence over argocd-cmd-params-cm dexserver.connector.failure.continue.
	// +optional
	ConnectorFailureContinue *bool `json:"connectorFailureContinue,omitempty" protobuf:"varint,3,opt,name=connectorFailureContinue"`
}

// NotificationsConfig holds notifications-controller settings.
type NotificationsConfig struct {
	// Log holds notifications-controller log format and level
	// (cmd-params: notificationscontroller.log.*).
	// Migration: if non-nil, takes precedence over argocd-cmd-params-cm notificationscontroller.log.* as a group.
	// +optional
	Log *LogConfig `json:"log,omitempty" protobuf:"bytes,1,opt,name=log"`
	// ProcessorsCount is the number of notification processors
	// (cmd-params: notificationscontroller.processors.count).
	// Migration: if set, takes precedence over argocd-cmd-params-cm notificationscontroller.processors.count.
	// +optional
	// +kubebuilder:validation:Minimum=1
	ProcessorsCount *int32 `json:"processorsCount,omitempty" protobuf:"varint,2,opt,name=processorsCount"`
	// SelfServiceEnabled enables notifications self-service
	// (cmd-params: notificationscontroller.selfservice.enabled).
	// Migration: if set, takes precedence over argocd-cmd-params-cm notificationscontroller.selfservice.enabled.
	// +optional
	SelfServiceEnabled *bool `json:"selfServiceEnabled,omitempty" protobuf:"varint,3,opt,name=selfServiceEnabled"`
	// TLSEnabled controls TLS when connecting to the repo-server
	// (cmd-params: notificationscontroller.repo.server.plaintext —
	// inverted: plaintext=true means tlsEnabled=false).
	// Migration: if set, takes precedence over argocd-cmd-params-cm notificationscontroller.repo.server.plaintext (inverted).
	// +optional
	TLSEnabled *bool `json:"tlsEnabled,omitempty" protobuf:"varint,4,opt,name=tlsEnabled"`
	// RepoServerClient holds mTLS cert paths when connecting to the repo-server
	// (cmd-params: notificationscontroller.repo.server.*). Named distinctly from
	// spec.repoServer (the repo-server itself) to avoid overloaded keys.
	// Migration: organizational subgroup; no legacy key — see child fields.
	// +optional
	RepoServerClient *MTLSCertConfig `json:"repoServerClient,omitempty" protobuf:"bytes,5,opt,name=repoServerClient"`
	// AppLabelSelector restricts which Applications the notifications controller watches
	// (flag: --app-label-selector).
	// Migration: if set, takes precedence over notifications --app-label-selector. Does not round-trip through ConfigMaps.
	// +optional
	AppLabelSelector string `json:"appLabelSelector,omitempty" protobuf:"bytes,6,opt,name=appLabelSelector"`
	// RepoServerAddress is the repo-server host:port
	// (flag: --argocd-repo-server).
	// Migration: if set, takes precedence over notifications --argocd-repo-server. Does not round-trip through ConfigMaps.
	// +optional
	RepoServerAddress string `json:"repoServerAddress,omitempty" protobuf:"bytes,7,opt,name=repoServerAddress"`
	// RepoServerStrictTLS requires a verified TLS connection to the repo-server
	// (flag: --argocd-repo-server-strict-tls).
	// Migration: if set, takes precedence over notifications --argocd-repo-server-strict-tls. Does not round-trip through ConfigMaps.
	// +optional
	RepoServerStrictTLS *bool `json:"repoServerStrictTLS,omitempty" protobuf:"varint,8,opt,name=repoServerStrictTLS"`
	// ConfigMapName is the notifications ConfigMap name (flag: --config-map-name).
	// Migration: if set, takes precedence over notifications --config-map-name. Does not round-trip through ConfigMaps.
	// +optional
	ConfigMapName string `json:"configMapName,omitempty" protobuf:"bytes,9,opt,name=configMapName"`
	// SecretName is the notifications Secret name (flag: --secret-name).
	// Migration: if set, takes precedence over notifications --secret-name. Does not round-trip through ConfigMaps.
	// +optional
	SecretName string `json:"secretName,omitempty" protobuf:"bytes,10,opt,name=secretName"`
	// MetricsPort is the metrics listen port (flag: --metrics-port).
	// Migration: if set, takes precedence over notifications --metrics-port. Does not round-trip through ConfigMaps.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	MetricsPort *int32 `json:"metricsPort,omitempty" protobuf:"varint,11,opt,name=metricsPort"`
}
