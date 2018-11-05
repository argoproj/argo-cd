package settings

import (
	"github.com/argoproj/argo-cd/util/rbac"
	"github.com/argoproj/argo-cd/util/settings"
	"github.com/ghodss/yaml"
	"golang.org/x/net/context"
)

// Server provides a Settings service
type Server struct {
	mgr *settings.SettingsManager
}

// NewServer returns a new instance of the Repository service
func NewServer(mgr *settings.SettingsManager) *Server {
	return &Server{
		mgr: mgr,
	}
}

// Get returns ArgoCD settings
func (s *Server) Get(ctx context.Context, q *SettingsQuery) (*Settings, error) {
	argoCDSettings, err := s.mgr.GetSettings()
	if err != nil {
		return nil, err
	}

	set := Settings{
		URL: argoCDSettings.URL,
		RBAC: []ListOfClaimActions{
			ListOfClaimActions{
				Key: rbac.ClaimsResourceApplications,
				Values: []string{
					rbac.ClaimsActionCreate,
					rbac.ClaimsActionDelete,
					rbac.ClaimsActionGet,
					rbac.ClaimsActionSync,
					rbac.ClaimsActionUpdate,
				},
			},
			ListOfClaimActions{
				Key: rbac.ClaimsResourceClusters,
				Values: []string{
					rbac.ClaimsActionCreate,
					rbac.ClaimsActionDelete,
					rbac.ClaimsActionGet,
					rbac.ClaimsActionUpdate,
				},
			},
			ListOfClaimActions{
				Key: rbac.ClaimsResourceProjects,
				Values: []string{
					rbac.ClaimsActionDelete,
					rbac.ClaimsActionGet,
					rbac.ClaimsActionUpdate,
				},
			},
			ListOfClaimActions{
				Key: rbac.ClaimsResourceRepositories,
				Values: []string{
					rbac.ClaimsActionCreate,
					rbac.ClaimsActionDelete,
					rbac.ClaimsActionGet,
					rbac.ClaimsActionUpdate,
				},
			},
		},
	}
	if argoCDSettings.DexConfig != "" {
		var cfg DexConfig
		err = yaml.Unmarshal([]byte(argoCDSettings.DexConfig), &cfg)
		if err == nil {
			set.DexConfig = &cfg
		}
	}
	if oidcConfig := argoCDSettings.OIDCConfig(); oidcConfig != nil {
		set.OIDCConfig = &OIDCConfig{
			Name:     oidcConfig.Name,
			Issuer:   oidcConfig.Issuer,
			ClientID: oidcConfig.ClientID,
		}
	}
	return &set, nil
}

// AuthFuncOverride disables authentication for settings service
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
