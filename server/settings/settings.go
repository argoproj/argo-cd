package settings

import (
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
	}
	var cfg DexConfig
	err = yaml.Unmarshal([]byte(argoCDSettings.DexConfig), &cfg)
	if err == nil {
		set.DexConfig = &cfg
	}
	return &set, nil
}
