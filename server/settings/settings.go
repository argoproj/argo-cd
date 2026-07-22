package settings

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	utilio "github.com/argoproj/argo-cd/v3/util/io"

	sessionmgr "github.com/argoproj/argo-cd/v3/util/session"

	settingspkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

// Server provides a Settings service
type Server struct {
	mgr            *settings.SettingsManager
	repoClient     apiclient.Clientset
	authenticator  Authenticator
	configProvider configbus.Provider
}

type Authenticator interface {
	Authenticate(ctx context.Context) (context.Context, error)
}

// NewServer returns a new instance of the Settings service
func NewServer(mgr *settings.SettingsManager, repoClient apiclient.Clientset, authenticator Authenticator, configProvider configbus.Provider) *Server {
	return &Server{mgr: mgr, repoClient: repoClient, authenticator: authenticator, configProvider: configProvider}
}

// Get returns Argo CD settings
func (s *Server) Get(ctx context.Context, _ *settingspkg.SettingsQuery) (*settingspkg.Settings, error) {
	resourceOverrides, err := s.configProvider.ResourceOverrides(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ResourceOverrides: %w", err)
	}
	overrides := make(map[string]*v1alpha1.ResourceOverride)
	for k := range resourceOverrides {
		val := resourceOverrides[k]
		overrides[k] = &val
	}
	appInstanceLabelKey, err := s.configProvider.AppInstanceLabelKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve AppInstanceLabelKey: %w", err)
	}
	argoCDSettings, err := s.mgr.GetSettings()
	if err != nil {
		return nil, err
	}
	gaSettings, err := s.configProvider.GoogleAnalytics(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GoogleAnalytics: %w", err)
	}
	help, err := s.configProvider.Help(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Help: %w", err)
	}
	userLoginsDisabled := true
	accounts, err := s.configProvider.Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve Accounts: %w", err)
	}
	for _, account := range accounts {
		if account.Enabled && account.HasCapability(settings.AccountCapabilityLogin) {
			userLoginsDisabled = false
			break
		}
	}

	kustomizeSettings, err := s.configProvider.KustomizeSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve KustomizeSettings: %w", err)
	}
	var kustomizeVersions []string
	for i := range kustomizeSettings.Versions {
		kustomizeVersions = append(kustomizeVersions, kustomizeSettings.Versions[i].Name)
	}

	trackingMethod, err := s.configProvider.TrackingMethod(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve TrackingMethod: %w", err)
	}

	installationID, err := s.configProvider.InstallationID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve InstallationID: %w", err)
	}

	statusBadgeEnabled, err := s.configProvider.StatusBadgeEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve StatusBadgeEnabled: %w", err)
	}
	execEnabled, err := s.configProvider.ExecEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ExecEnabled: %w", err)
	}
	impersonationEnabled, err := s.configProvider.IsImpersonationEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve IsImpersonationEnabled: %w", err)
	}

	applicationNamespaces, err := s.configProvider.ApplicationNamespaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ApplicationNamespaces: %w", err)
	}
	hydratorEnabled, err := s.configProvider.HydratorEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve HydratorEnabled: %w", err)
	}
	syncWithReplaceAllowed, err := s.configProvider.SyncWithReplaceAllowed(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve SyncWithReplaceAllowed: %w", err)
	}
	disableAuth, err := s.configProvider.DisableAuth(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve DisableAuth: %w", err)
	}

	serverURL, err := s.configProvider.ServerURL(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ServerURL: %w", err)
	}
	additionalURLs, err := s.configProvider.AdditionalURLs(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve AdditionalURLs: %w", err)
	}

	set := settingspkg.Settings{
		URL:                serverURL,
		AdditionalURLs:     additionalURLs,
		AppLabelKey:        appInstanceLabelKey,
		StatusBadgeEnabled: statusBadgeEnabled,
		StatusBadgeRootUrl: argoCDSettings.StatusBadgeRootUrl,
		KustomizeOptions: &v1alpha1.KustomizeOptions{
			BuildOptions: kustomizeSettings.BuildOptions,
		},
		GoogleAnalytics: &settingspkg.GoogleAnalyticsConfig{
			TrackingID:     gaSettings.TrackingID,
			AnonymizeUsers: gaSettings.AnonymizeUsers,
		},
		Help: &settingspkg.Help{
			ChatUrl:    help.ChatURL,
			ChatText:   help.ChatText,
			BinaryUrls: help.BinaryURLs,
		},
		UserLoginsDisabled:        userLoginsDisabled,
		KustomizeVersions:         kustomizeVersions,
		UiCssURL:                  argoCDSettings.UiCssURL,
		UiLoginButtonText:         argoCDSettings.UiLoginButtonText,
		TrackingMethod:            trackingMethod,
		InstallationID:            installationID,
		ExecEnabled:               execEnabled,
		AppsInAnyNamespaceEnabled: len(applicationNamespaces) > 0,
		ImpersonationEnabled:      impersonationEnabled,
		HydratorEnabled:           hydratorEnabled,
		SyncWithReplaceAllowed:    syncWithReplaceAllowed,
		ResourceViewEnabled:       argoCDSettings.ResourceViewEnabled,
	}

	if sessionmgr.LoggedIn(ctx) || disableAuth {
		set.UiBannerContent = argoCDSettings.UiBannerContent
		set.UiBannerURL = argoCDSettings.UiBannerURL
		set.UiBannerPermanent = argoCDSettings.UiBannerPermanent
		set.UiBannerPosition = argoCDSettings.UiBannerPosition
		set.ControllerNamespace = s.mgr.GetNamespace()
		set.ResourceOverrides = overrides
	}
	if sessionmgr.LoggedIn(ctx) {
		passwordPattern, err := s.configProvider.PasswordPattern(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve PasswordPattern: %w", err)
		}
		set.PasswordPattern = passwordPattern
	}
	if argoCDSettings.DexConfig != "" {
		var cfg settingspkg.DexConfig
		err = yaml.Unmarshal([]byte(argoCDSettings.DexConfig), &cfg)
		if err == nil {
			// DexAuthConnectorID, when set, tells the login screen to redirect directly to the
			// given connector, bypassing Dex's connector selection screen. It is only populated
			// with a connector ID that exists in dex.config (validated in the settings manager).
			cfg.DexAuthConnectorID = argoCDSettings.DexAuthConnectorID
			set.DexConfig = &cfg
		}
	}
	if oidcConfig := argoCDSettings.OIDCConfig(); oidcConfig != nil {
		set.OIDCConfig = &settingspkg.OIDCConfig{
			Name:                     oidcConfig.Name,
			Issuer:                   oidcConfig.Issuer,
			ClientID:                 oidcConfig.ClientID,
			CLIClientID:              oidcConfig.CLIClientID,
			Scopes:                   oidcConfig.RequestedScopes,
			EnablePKCEAuthentication: oidcConfig.EnablePKCEAuthentication,
		}
		if len(argoCDSettings.OIDCConfig().RequestedIDTokenClaims) > 0 {
			set.OIDCConfig.IDTokenClaims = argoCDSettings.OIDCConfig().RequestedIDTokenClaims
		}
	}
	return &set, nil
}

// GetPlugins returns a list of plugins
func (s *Server) GetPlugins(ctx context.Context, _ *settingspkg.SettingsQuery) (*settingspkg.SettingsPluginsResponse, error) {
	plugins, err := s.plugins(ctx)
	if err != nil {
		return nil, err
	}
	return &settingspkg.SettingsPluginsResponse{Plugins: plugins}, nil
}

func (s *Server) plugins(ctx context.Context) ([]*settingspkg.Plugin, error) {
	closer, client, err := s.repoClient.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error creating repo server client: %w", err)
	}
	defer utilio.Close(closer)

	pluginList, err := client.ListPlugins(ctx, &empty.Empty{})
	if err != nil {
		return nil, fmt.Errorf("failed to list sidecar plugins from reposerver: %w", err)
	}

	var out []*settingspkg.Plugin
	if pluginList != nil && len(pluginList.Items) > 0 {
		for _, p := range pluginList.Items {
			out = append(out, &settingspkg.Plugin{Name: p.Name})
		}
	}

	return out, nil
}

// AuthFuncOverride disables authentication for settings service
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	ctx, err := s.authenticator.Authenticate(ctx)
	if fullMethodName == "/cluster.SettingsService/Get" {
		// SettingsService/Get API is used by login page.
		// This authenticates the user, but ignores any error, so that we have claims populated
		err = nil
	}
	return ctx, err
}
