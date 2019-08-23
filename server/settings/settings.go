package settings

import (
	"github.com/ghodss/yaml"
	"golang.org/x/net/context"

	settingspkg "github.com/argoproj/argo-cd/pkg/apiclient/settings"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
)

// Server provides a Settings service
type Server struct {
	mgr *settings.SettingsManager
}

// NewServer returns a new instance of the Settings service
func NewServer(mgr *settings.SettingsManager) *Server {
	return &Server{
		mgr: mgr,
	}
}

// Get returns Argo CD settings
func (s *Server) Get(ctx context.Context, q *settingspkg.SettingsQuery) (*settingspkg.Settings, error) {
	resourceOverrides, err := s.mgr.GetResourceOverrides()
	if err != nil {
		return nil, err
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

	overrides := make(map[string]*v1alpha1.ResourceOverride)
	for k := range resourceOverrides {
		val := resourceOverrides[k]
		overrides[k] = &val
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
	}
	return &set, nil
}

// AuthFuncOverride disables authentication for settings service
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
