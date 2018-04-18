package version

import (
	argocd "github.com/argoproj/argo-cd"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
)

type Server struct{}

// Version returns the version of the API server
func (s *Server) Version(context.Context, *empty.Empty) (*VersionMessage, error) {
	vers := argocd.GetVersion()
	return &VersionMessage{
		Version:      vers.Version,
		BuildDate:    vers.BuildDate,
		GitCommit:    vers.GitCommit,
		GitTag:       vers.GitTag,
		GitTreeState: vers.GitTreeState,
		GoVersion:    vers.GoVersion,
		Compiler:     vers.Compiler,
		Platform:     vers.Platform,
	}, nil
}

// AuthFuncOverride allows the version to be returned without auth
func (s *Server) AuthFuncOverride(ctx context.Context, fullMethodName string) (context.Context, error) {
	return ctx, nil
}
