package settings

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	ioutil "github.com/argoproj/argo-cd/v2/util/io"

	sessionmgr "github.com/argoproj/argo-cd/v2/util/session"

	settingspkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

// Server provides a Settings service
type Server struct {
	mgr                       *settings.SettingsManager
	repoClient                apiclient.Clientset
	authenticator             Authenticator
	disableAuth               bool
	appsInAnyNamespaceEnabled bool
}

type Authenticator interface {
	Authenticate(ctx context.Context) (context.Context, error)
}

// NewServer returns a new instance of the Settings service
func NewServer(mgr *settings.SettingsManager, repoClient apiclient.Clientset, authenticator Authenticator, disableAuth, appsInAnyNamespaceEnabled bool) *Server {
	return &Server{mgr: mgr, repoClient: repoClient, authenticator: authenticator, disableAuth: disableAuth, appsInAnyNamespaceEnabled: appsInAnyNamespaceEnabled}
}

// Get returns Argo CD settings
func (s *Server) Get(ctx context.Context, q *settingspkg.SettingsQuery) (*settingspkg.Settings, error) {
	resourceOverrides, err := s.mgr.GetResourceOverrides()
	if err != nil {
		return nil, err
	}
	overrides := make(map[string]*v1alpha1.ResourceOverride)
	for k := range resourceOverrides {
		val := resourceOverrides[k]
		overrides[k] = &val
	}
	appInstanceLabelKey, err := s.mgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, err
	}
	argoCDSettings, err := s.mgr.GetSettings()
	if err != nil {
		return nil, err
	}
	gaSettings, err := s.mgr.GetGoogleAnalytics()
	if err != nil {
		return nil, err
	}
	help, err := s.mgr.GetHelp()
	if err != nil {
		return nil, err
	}
	userLoginsDisabled := true
	accounts, err := s.mgr.GetAccounts()
	if err != nil {
		return nil, err
	}
	for _, account := range accounts {
		if account.Enabled && account.HasCapability(settings.AccountCapabilityLogin) {
			userLoginsDisabled = false
			break
		}
	}

	kustomizeSettings, err := s.mgr.GetKustomizeSettings()
	if err != nil {
		return nil, err
	}
	var kustomizeVersions []string
	for i := range kustomizeSettings.Versions {
		kustomizeVersions = append(kustomizeVersions, kustomizeSettings.Versions[i].Name)
	}

	trackingMethod, err := s.mgr.GetTrackingMethod()
	if err != nil {
		return nil, err
	}

	set := settingspkg.Settings{
		URL:                argoCDSettings.URL,
		AppLabelKey:        appInstanceLabelKey,
		ResourceOverrides:  overrides,
		StatusBadgeEnabled: argoCDSettings.StatusBadgeEnabled,
		StatusBadgeRootUrl: argoCDSettings.StatusBadgeRootUrl,
		KustomizeOptions: &v1alpha1.KustomizeOptions{
			BuildOptions: argoCDSettings.KustomizeBuildOptions,
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
		TrackingMethod:            trackingMethod,
		ExecEnabled:               argoCDSettings.ExecEnabled,
		AppsInAnyNamespaceEnabled: s.appsInAnyNamespaceEnabled,
		ImpersonationEnabled:      argoCDSettings.ImpersonationEnabled,
	}

	if sessionmgr.LoggedIn(ctx) || s.disableAuth {
		set.UiBannerContent = argoCDSettings.UiBannerContent
		set.UiBannerURL = argoCDSettings.UiBannerURL
		set.UiBannerPermanent = argoCDSettings.UiBannerPermanent
		set.UiBannerPosition = argoCDSettings.UiBannerPosition
		set.ControllerNamespace = s.mgr.GetNamespace()
	}
	if sessionmgr.LoggedIn(ctx) {
		set.PasswordPattern = argoCDSettings.PasswordPattern
	}
	if argoCDSettings.DexConfig != "" {
		var cfg settingspkg.DexConfig
		err = yaml.Unmarshal([]byte(argoCDSettings.DexConfig), &cfg)
		if err == nil {
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
func (s *Server) GetPlugins(ctx context.Context, q *settingspkg.SettingsQuery) (*settingspkg.SettingsPluginsResponse, error) {
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
	defer ioutil.Close(closer)

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
