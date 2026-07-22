package version

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/go-jsonnet"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/v3/server/settings"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/helm"
	"github.com/argoproj/argo-cd/v3/util/kustomize"
	sessionmgr "github.com/argoproj/argo-cd/v3/util/session"
	utilsettings "github.com/argoproj/argo-cd/v3/util/settings"
)

type Server struct {
	kustomizeVersion string
	helmVersion      string
	jsonnetVersion   string
	authenticator    settings.Authenticator
	configProvider   configbus.Provider
	settingsMgr      *utilsettings.SettingsManager
}

func NewServer(authenticator settings.Authenticator, configProvider configbus.Provider, settingsMgr *utilsettings.SettingsManager) *Server {
	return &Server{authenticator: authenticator, configProvider: configProvider, settingsMgr: settingsMgr}
}

func (s *Server) allowUnauthenticated(ctx context.Context) (bool, error) {
	disableAuth, err := s.configProvider.DisableAuth(ctx)
	if err != nil {
		return false, err
	}
	if disableAuth {
		return true, nil
	}
	sett, err := s.settingsMgr.GetSettings()
	if err != nil {
		return false, err
	}
	return sett.AnonymousUserEnabled, nil
}

// Version returns the version of the API server
func (s *Server) Version(ctx context.Context, _ *empty.Empty) (*version.VersionMessage, error) {
	vers := common.GetVersion()
	allowUnauthenticated, err := s.allowUnauthenticated(ctx)
	if err != nil {
		return nil, err
	}

	if !sessionmgr.LoggedIn(ctx) && !allowUnauthenticated {
		return &version.VersionMessage{Version: vers.Version}, nil
	}

	if s.kustomizeVersion == "" {
		kustomizeVersion, err := kustomize.Version()
		if err == nil {
			s.kustomizeVersion = kustomizeVersion
		} else {
			s.kustomizeVersion = err.Error()
		}
	}
	if s.helmVersion == "" {
		helmVersion, err := helm.Version()
		if err == nil {
			s.helmVersion = helmVersion
		} else {
			s.helmVersion = err.Error()
		}
	}
	s.jsonnetVersion = jsonnet.Version()
	return &version.VersionMessage{
		Version:          vers.Version,
		BuildDate:        vers.BuildDate,
		GitCommit:        vers.GitCommit,
		GitTag:           vers.GitTag,
		GitTreeState:     vers.GitTreeState,
		GoVersion:        vers.GoVersion,
		Compiler:         vers.Compiler,
		Platform:         vers.Platform,
		KustomizeVersion: s.kustomizeVersion,
		HelmVersion:      s.helmVersion,
		JsonnetVersion:   s.jsonnetVersion,
		KubectlVersion:   vers.KubectlVersion,
		ExtraBuildInfo:   vers.ExtraBuildInfo,
	}, nil
}

// AuthFuncOverride allows the version to be returned without auth
func (s *Server) AuthFuncOverride(ctx context.Context, _ string) (context.Context, error) {
	if s.authenticator != nil {
		// this authenticates the user, but ignores any error, so that we have claims populated
		ctx, _ = s.authenticator.Authenticate(ctx)
	}
	return ctx, nil
}
