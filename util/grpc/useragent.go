package grpc

import (
	"strings"

	"github.com/Masterminds/semver/v3"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UserAgentUnaryServerInterceptor returns a UnaryServerInterceptor which enforces a minimum client
// version in the user agent
func UserAgentUnaryServerInterceptor(clientName, constraintStr string) grpc.UnaryServerInterceptor {
	semVerConstraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		panic(err)
	}
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := userAgentEnforcer(ctx, clientName, constraintStr, semVerConstraint); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

// UserAgentStreamServerInterceptor returns a StreamServerInterceptor which enforces a minimum client
// version in the user agent
func UserAgentStreamServerInterceptor(clientName, constraintStr string) grpc.StreamServerInterceptor {
	semVerConstraint, err := semver.NewConstraint(constraintStr)
	if err != nil {
		panic(err)
	}
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := userAgentEnforcer(stream.Context(), clientName, constraintStr, semVerConstraint); err != nil {
			return err
		}
		return handler(srv, stream)
	}
}

func userAgentEnforcer(ctx context.Context, clientName, constraintStr string, semVerConstraint *semver.Constraints) error {
	var userAgents []string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for _, ua := range md["user-agent"] {
			// ua is a string like "argocd-client/v0.11.0+cde040e grpc-go/1.15.0"
			userAgents = append(userAgents, strings.Fields(ua)...)
			break
		}
	}
	if isLegacyClient(userAgents) {
		return status.Errorf(codes.FailedPrecondition, "unsatisfied client version constraint: %s", constraintStr)
	}

	for _, userAgent := range userAgents {
		uaSplit := strings.Split(userAgent, "/")
		if len(uaSplit) != 2 || uaSplit[0] != clientName {
			// User-agent was supplied, but client/format is not one we care about (e.g. grpc-go)
			continue
		}
		// remove pre-release part
		versionStr := strings.Split(uaSplit[1], "-")[0]
		// We have matched the client name to the one we care about
		uaVers, err := semver.NewVersion(versionStr)
		if err != nil {
			return status.Errorf(codes.InvalidArgument, "could not parse version from user-agent: %s", userAgent)
		}
		if ok, errs := semVerConstraint.Validate(uaVers); !ok {
			return status.Errorf(codes.FailedPrecondition, "unsatisfied client version constraint: %s", errs[0].Error())
		}
		return nil
	}
	// If we get here, the caller either did not supply user-agent, supplied one which we don't
	// care about. This implies it is a from a custom generated client, so we permit the request.
	// We really only want to enforce user-agent version constraints for clients under our
	// control which we know to have compatibility issues
	return nil
}

// isLegacyClient checks if the request was made from a legacy Argo CD client (i.e. v0.10 CLI).
// The heuristic is that a single default 'grpc-go' user-agent was specified with one of the
// previous versions of grpc-go we used in the past (1.15.0, 1.10.0).
// Starting in v0.11, both of the gRPC clients we maintain (pkg/apiclient and grpc-gateway) started
// supplying a explicit user-agent tied to the Argo CD version.
func isLegacyClient(userAgents []string) bool {
	return len(userAgents) == 1 && (userAgents[0] == "grpc-go/1.15.0" || userAgents[0] == "grpc-go/1.10.0")
}
