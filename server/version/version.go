package version

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/util/helm"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/plugins"
)

type Server struct {
	ksonnetVersion string
	helmVersion    string
	kubectlVersion string
	pluginVersions map[string]string
}

// Version returns the version of the API server
func (s *Server) Version(context.Context, *empty.Empty) (*version.VersionMessage, error) {
	vers := common.GetVersion()
	if s.ksonnetVersion == "" {
		ksonnetVersion, err := ksutil.Version()
		if err != nil {
			return nil, err
		}
		s.ksonnetVersion = ksonnetVersion
	}
	if s.helmVersion == "" {
		helmVersion, err := helm.Version()
		if err != nil {
			return nil, err
		}
		s.helmVersion = helmVersion
	}
	if s.kubectlVersion == "" {
		kubectlVersion, err := kube.Version()
		if err != nil {
			return nil, err
		}
		s.kubectlVersion = kubectlVersion
	}
	if len(s.pluginVersions) == 0 {
		for name, plugin := range plugins.Plugins() {
			v, err := plugin.Version()
			if err != nil {
				return nil, err
			}
			s.pluginVersions[name] = v
		}
	}
	return &version.VersionMessage{
		Version:        vers.Version,
		BuildDate:      vers.BuildDate,
		GitCommit:      vers.GitCommit,
		GitTag:         vers.GitTag,
		GitTreeState:   vers.GitTreeState,
		GoVersion:      vers.GoVersion,
		Compiler:       vers.Compiler,
		Platform:       vers.Platform,
		KsonnetVersion: s.ksonnetVersion,
		HelmVersion:    s.helmVersion,
		KubectlVersion: s.kubectlVersion,
		PluginVersions: s.pluginVersions,
	}, nil
}

// AuthFuncOverride allows the version to be returned without auth
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
