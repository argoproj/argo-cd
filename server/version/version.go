package version

import (
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"

	"github.com/google/go-jsonnet"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/version"
	"github.com/argoproj/argo-cd/util/helm"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
	"github.com/argoproj/argo-cd/util/kustomize"
)

type Server struct {
	ksonnetVersion   string
	kustomizeVersion string
	helmVersion      string
	kubectlVersion   string
	jsonnetVersion   string
}

// Version returns the version of the API server
func (s *Server) Version(context.Context, *empty.Empty) (*version.VersionMessage, error) {
	vers := common.GetVersion()
	if s.ksonnetVersion == "" {
		ksonnetVersion, err := ksutil.Version()
		if err == nil {
			s.ksonnetVersion = ksonnetVersion
		} else {
			s.ksonnetVersion = err.Error()
		}
	}
	if s.kustomizeVersion == "" {
		kustomizeVersion, err := kustomize.Version(true)
		if err == nil {
			s.kustomizeVersion = kustomizeVersion
		} else {
			s.kustomizeVersion = err.Error()
		}
	}
	if s.helmVersion == "" {
		helmVersion, err := helm.Version(true)
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
		KsonnetVersion:   s.ksonnetVersion,
		KustomizeVersion: s.kustomizeVersion,
		HelmVersion:      s.helmVersion,
		KubectlVersion:   s.kubectlVersion,
		JsonnetVersion:   s.jsonnetVersion,
	}, nil
}

// AuthFuncOverride allows the version to be returned without auth
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
