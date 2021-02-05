package version

import (
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/server/settings"
	"github.com/argoproj/argo-cd/util/helm"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kustomize"
	sessionmgr "github.com/argoproj/argo-cd/util/session"
)

type server struct {
	ksonnetVersion   string
	kustomizeVersion string
	helmVersion      string
	kubectlVersion   string
	jsonnetVersion   string
	authenticator    settings.Authenticator
	disableAuth      bool
}

func NewServer(authenticator settings.Authenticator, disableAuth bool) *server {
	return &server{authenticator: authenticator, disableAuth: disableAuth}
}

// Version returns the version of the API server
func (s *server) Version(ctx context.Context, _ *empty.Empty) (*version.VersionMessage, error) {
	vers := common.GetVersion()

	if !sessionmgr.LoggedIn(ctx) && !s.disableAuth {
		return &version.VersionMessage{Version: vers.Version}, nil
	}

	if s.ksonnetVersion == "" {
		ksonnetVersion, err := ksutil.Version()
		if err == nil {
			s.ksonnetVersion = ksonnetVersion
		} else {
			s.ksonnetVersion = err.Error()
		}
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
	if s.kubectlVersion == "" {
		kubectlVersion, err := kube.Version()
		if err == nil {
			s.kubectlVersion = kubectlVersion
		} else {
			s.kubectlVersion = err.Error()
		}
	}
	return &version.VersionMessage{
		Version:          vers.Version,
		BuildDate:        vers.BuildDate,
		GitCommit:        vers.GitCommit,
		GitTag:           vers.GitTag,
		GitTreeState:     vers.GitTreeState,
		GoVersion:        vers.GoVersion,
		Compiler:         vers.Compiler,
		Platform:         vers.Platform,
		KsonnetVersion:   s.ksonnetVersion,
		KustomizeVersion: s.kustomizeVersion,
		HelmVersion:      s.helmVersion,
		KubectlVersion:   s.kubectlVersion,
	}, nil
}

// AuthFuncOverride allows the version to be returned without auth
func (s *server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	if s.authenticator != nil {
		// this authenticates the user, but ignores any error, so that we have claims populated
		ctx, _ = s.authenticator.Authenticate(ctx)
	}
	return ctx, nil
}
