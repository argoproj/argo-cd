package settings

import (
	"github.com/ghodss/yaml"
	"golang.org/x/net/context"

	sessionmgr "github.com/argoproj/argo-cd/util/session"

	settingspkg "github.com/argoproj/argo-cd/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Settings service
type Server struct {
	mgr           *settings.SettingsManager
	authenticator Authenticator
	disableAuth   bool
}

type Authenticator interface {
	Authenticate(ctx context.Context) (context.Context, error)
}

// NewServer returns a new instance of the Settings service
func NewServer(mgr *settings.SettingsManager, authenticator Authenticator, disableAuth bool) *Server {
	return &Server{mgr: mgr, authenticator: authenticator, disableAuth: disableAuth}
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
	plugins, err := s.plugins()
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

	set := settingspkg.Settings{
		URL:                argoCDSettings.URL,
		AppLabelKey:        appInstanceLabelKey,
		ResourceOverrides:  overrides,
		StatusBadgeEnabled: argoCDSettings.StatusBadgeEnabled,
		KustomizeOptions: &v1alpha1.KustomizeOptions{
			BuildOptions: argoCDSettings.KustomizeBuildOptions,
		},
		GoogleAnalytics: &settingspkg.GoogleAnalyticsConfig{
			TrackingID:     gaSettings.TrackingID,
			AnonymizeUsers: gaSettings.AnonymizeUsers,
		},
		Help: &settingspkg.Help{
			ChatUrl:  help.ChatURL,
			ChatText: help.ChatText,
		},
		Plugins:            plugins,
		UserLoginsDisabled: userLoginsDisabled,
		KustomizeVersions:  kustomizeVersions,
		UiCssURL:           argoCDSettings.UiCssURL,
	}

	if sessionmgr.LoggedIn(ctx) || s.disableAuth {
		configManagementPlugins, err := s.mgr.GetConfigManagementPlugins()
		if err != nil {
			return nil, err
		}
		tools := make([]*v1alpha1.ConfigManagementPlugin, len(configManagementPlugins))
		for i := range configManagementPlugins {
			tools[i] = &configManagementPlugins[i]
		}
		set.ConfigManagementPlugins = tools
		set.UiBannerContent = argoCDSettings.UiBannerContent
		set.UiBannerURL = argoCDSettings.UiBannerURL
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
			Name:        oidcConfig.Name,
			Issuer:      oidcConfig.Issuer,
			ClientID:    oidcConfig.ClientID,
			CLIClientID: oidcConfig.CLIClientID,
			Scopes:      oidcConfig.RequestedScopes,
		}
		if len(argoCDSettings.OIDCConfig().RequestedIDTokenClaims) > 0 {
			set.OIDCConfig.IDTokenClaims = argoCDSettings.OIDCConfig().RequestedIDTokenClaims
		}
	}
	return &set, nil
}

func (s *Server) plugins() ([]*settingspkg.Plugin, error) {
	in, err := s.mgr.GetConfigManagementPlugins()
	if err != nil {
		return nil, err
	}
	out := make([]*settingspkg.Plugin, len(in))
	for i, p := range in {
		out[i] = &settingspkg.Plugin{Name: p.Name}

	}
	return out, nil
}

// AuthFuncOverride disables authentication for settings service
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	// this authenticates the user, but ignores any error, so that we have claims populated
	ctx, _ = s.authenticator.Authenticate(ctx)
	return ctx, nil
}
