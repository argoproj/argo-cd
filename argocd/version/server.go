package version

import (
	argocd "github.com/argoproj/argo-cd"
	"golang.org/x/net/context"
)

type Server struct{}

// Version returns the version of the API server
func (v *Server) Version(context.Context, *VersionMessage) (*VersionMessage, error) {
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
