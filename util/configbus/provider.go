package configbus

import (
	"context"
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// ErrNotConfigured is returned by a leaf Provider when it does not own / does
// not have a value for a field. ChainProvider skips links that return this
// sentinel and continues to the next link.
var ErrNotConfigured = errors.New("config: not configured")

// SelfHealRetry is the self-heal retry policy. A nil Backoff means use a flat
// SelfHealTimeout rather than exponential backoff.
type SelfHealRetry struct {
	Backoff *wait.Backoff
}

// Provider is the typed config API for one Argo CD process.
//
// Construction rules (for reviewers and contributors):
//
//   - Each method is the smallest migrateable unit: when its backing CRD field
//     is set, every nested value under that field is considered migrated.
//   - Method names are alphabetical so each component layer can insert receivers
//     in a predictable place and PRs stay skimmable.
//   - Every config getter returns (T, error) and accepts context.Context for
//     future Kubernetes/informer-backed reads and logging. Implementations must
//     never omit the error return.
//
// Production processes compose leaf providers with ChainProvider (Static /
// SettingsManagerProvider / Env; CRD is inserted once wired). Tests typically
// inject mocks.Provider from mockery, or a StaticProvider literal.
type Provider interface {
	// Subscribe registers for argocd-cm/secret change notifications when the
	// backing implementation supports it (SettingsManagerProvider / ChainProvider).
	Subscribe(subCh chan<- *settings.ArgoCDSettings)
	// Unsubscribe unregisters a settings change subscriber.
	Unsubscribe(subCh chan<- *settings.ArgoCDSettings)

	Accounts(ctx context.Context) (map[string]settings.Account, error)
	AdditionalURLs(ctx context.Context) ([]string, error)
	AllowedNodeLabels(ctx context.Context) ([]string, error)
	AllowedScmProviders(ctx context.Context) ([]string, error)
	AnonymousUserEnabled(ctx context.Context) (bool, error)
	AppInstanceLabelKey(ctx context.Context) (string, error)
	ApplicationDeepLinks(ctx context.Context) ([]settings.DeepLink, error)
	ApplicationFineGrainedRBACInheritanceDisabled(ctx context.Context) (bool, error)
	ApplicationNamespaces(ctx context.Context) ([]string, error)
	BaseHRef(ctx context.Context) (string, error)
	CommitAuthorEmail(ctx context.Context) (string, error)
	CommitAuthorName(ctx context.Context) (string, error)
	ContentSecurityPolicy(ctx context.Context) (string, error)
	ContentTypes(ctx context.Context) ([]string, error)
	DexServerAddr(ctx context.Context) (string, error)
	DexServerPlaintext(ctx context.Context) (bool, error)
	DexServerStrictTLS(ctx context.Context) (bool, error)
	DisableAuth(ctx context.Context) (bool, error)
	EnableGZip(ctx context.Context) (bool, error)
	EnableGitHubAPIMetrics(ctx context.Context) (bool, error)
	EnableK8sEvent(ctx context.Context) ([]string, error)
	EnableNewGitFileGlobbing(ctx context.Context) (bool, error)
	EnableProxyExtension(ctx context.Context) (bool, error)
	EnableScmProviders(ctx context.Context) (bool, error)
	EnabledSourceTypes(ctx context.Context) (map[string]bool, error)
	ExcludeEventLabelKeys(ctx context.Context) ([]string, error)
	ExecEnabled(ctx context.Context) (bool, error)
	ExecShells(ctx context.Context) ([]string, error)
	ExtensionConfig(ctx context.Context) (map[string]string, error)
	GitRequestTimeout(ctx context.Context) (time.Duration, error)
	GitSubmoduleEnabled(ctx context.Context) (bool, error)
	GlobalProjectsSettings(ctx context.Context) ([]settings.GlobalProjectSettings, error)
	GoogleAnalytics(ctx context.Context) (*settings.GoogleAnalytics, error)
	HardReconciliationTimeout(ctx context.Context) (time.Duration, error)
	HelmSettings(ctx context.Context) (v1alpha1.HelmOptions, error)
	Help(ctx context.Context) (*settings.Help, error)
	HydratorEnabled(ctx context.Context) (bool, error)
	HydratorReadmeTemplate(ctx context.Context) (string, error)
	IgnoreNormalizerJQTimeout(ctx context.Context) (time.Duration, error)
	IgnoreResourceUpdatesOverrides(ctx context.Context) (map[string]v1alpha1.ResourceOverride, error)
	InClusterEnabled(ctx context.Context) (bool, error)
	IncludeEventLabelKeys(ctx context.Context) ([]string, error)
	Insecure(ctx context.Context) (bool, error)
	InstallationID(ctx context.Context) (string, error)
	IsIgnoreResourceUpdatesEnabled(ctx context.Context) (bool, error)
	IsImpersonationEnabled(ctx context.Context) (bool, error)
	IsImpersonationEnforced(ctx context.Context) (bool, error)
	KustomizeSettings(ctx context.Context) (v1alpha1.KustomizeOptions, error)
	ListenHost(ctx context.Context) (string, error)
	ListenPort(ctx context.Context) (int, error)
	MaxPodLogsToRender(ctx context.Context) (int64, error)
	MaxWebhookPayloadSize(ctx context.Context) (int64, error)
	MetricsClusterLabels(ctx context.Context) ([]string, error)
	MetricsHost(ctx context.Context) (string, error)
	MetricsPort(ctx context.Context) (int, error)
	OIDCLogoutURL(ctx context.Context) (string, error)
	PasswordPattern(ctx context.Context) (string, error)
	PersistResourceHealth(ctx context.Context) (bool, error)
	ProjectDeepLinks(ctx context.Context) ([]settings.DeepLink, error)
	ReconciliationJitter(ctx context.Context) (time.Duration, error)
	ReconciliationTimeout(ctx context.Context) (time.Duration, error)
	RepoErrorGracePeriod(ctx context.Context) (time.Duration, error)
	RequireOverridePrivilegeForRevisionSync(ctx context.Context) (bool, error)
	ResourceCompareOptions(ctx context.Context) (settings.ArgoCDDiffOptions, error)
	ResourceCustomLabels(ctx context.Context) ([]string, error)
	ResourceDeepLinks(ctx context.Context) ([]settings.DeepLink, error)
	ResourceOverrides(ctx context.Context) (map[string]v1alpha1.ResourceOverride, error)
	ResourcesFilter(ctx context.Context) (settings.ResourcesFilter, error)
	RespectRBAC(ctx context.Context) (int, error)
	RootPath(ctx context.Context) (string, error)
	ScmRootCAPath(ctx context.Context) (string, error)
	SelfHealRetry(ctx context.Context) (SelfHealRetry, error)
	SelfHealTimeout(ctx context.Context) (time.Duration, error)
	SensitiveAnnotations(ctx context.Context) (map[string]bool, error)
	ServerSideDiff(ctx context.Context) (bool, error)
	ServerURL(ctx context.Context) (string, error)
	SourceHydratorCommitMessageTemplate(ctx context.Context) (string, error)
	StaticAssetsDir(ctx context.Context) (string, error)
	StatusBadgeEnabled(ctx context.Context) (bool, error)
	SyncTimeout(ctx context.Context) (time.Duration, error)
	SyncWithReplaceAllowed(ctx context.Context) (bool, error)
	TrackingMethod(ctx context.Context) (string, error)
	UserSessionDuration(ctx context.Context) (time.Duration, error)
	WebhookParallelism(ctx context.Context) (int, error)
	WebhookRefreshJitter(ctx context.Context) (time.Duration, error)
	WebhookRefreshJitterThreshold(ctx context.Context) (int, error)
	WebhookRefreshWorkers(ctx context.Context) (int, error)
	XFrameOptions(ctx context.Context) (string, error)
}

// firstConfigured tries each link in order and returns the first result that is
// not ErrNotConfigured. Other errors propagate immediately. If every link
// returns ErrNotConfigured, that sentinel is returned.
func firstConfigured[T any](fn func(Provider) (T, error), links []Provider) (T, error) {
	var zero T
	var lastNotConfigured error
	for _, link := range links {
		v, err := fn(link)
		if err == nil {
			return v, nil
		}
		if errors.Is(err, ErrNotConfigured) {
			lastNotConfigured = err
			continue
		}
		return zero, err
	}
	if lastNotConfigured != nil {
		return zero, lastNotConfigured
	}
	return zero, ErrNotConfigured
}
