package cmpserver

import (
	"github.com/argoproj/argo-cd/v2/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// authToken provides interceptors to put our authentication token header on the outbound messages.
// we're not verifying the client, the client is verifying the server.
type authToken struct {
	token func() string
}

func (at *authToken) streamInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := ss.SendHeader(metadata.Pairs(common.PluginAuthTokenHeader, at.token()))
	if err != nil {
		return err
	}
	return handler(srv, ss)
}
