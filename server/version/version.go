package version

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apiclient/version"
	ksutil "github.com/argoproj/argo-cd/util/ksonnet"
)

type Server struct{}

// Version returns the version of the API server
func (s *Server) Version(context.Context, *empty.Empty) (*version.VersionMessage, error) {
	vers := common.GetVersion()
	ksonnetVersion, err := ksutil.KsonnetVersion()
	if err != nil {
		return nil, err
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
		KsonnetVersion: ksonnetVersion,
	}, nil
}

// AuthFuncOverride allows the version to be returned without auth
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
