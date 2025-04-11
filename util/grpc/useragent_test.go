package grpc

import (
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func Test_UserAgentEnforcer(t *testing.T) {
	clientName := "argo-cd"
	semverConstraint, _ := semver.NewConstraint("^1")
	t.Run("Test enforcing valid user-agent", func(t *testing.T) {
		md := metadata.New(map[string]string{"user-agent": "argo-cd/1.0"})
		ctx := metadata.NewIncomingContext(t.Context(), md)
		err := userAgentEnforcer(ctx, clientName, semverConstraint)
		require.NoError(t, err)
	})
	t.Run("Test enforcing ignored user-agent", func(t *testing.T) {
		md := metadata.New(map[string]string{"user-agent": "flux/3.0"})
		ctx := metadata.NewIncomingContext(t.Context(), md)
		err := userAgentEnforcer(ctx, clientName, semverConstraint)
		require.NoError(t, err)
	})
	t.Run("Test enforcing user-agent with version not matching constraint", func(t *testing.T) {
		md := metadata.New(map[string]string{"user-agent": "argo-cd/3.0"})
		ctx := metadata.NewIncomingContext(t.Context(), md)
		err := userAgentEnforcer(ctx, clientName, semverConstraint)
		require.ErrorContains(t, err, "unsatisfied client version constraint")
	})
	t.Run("Test legacy user-agent", func(t *testing.T) {
		md := metadata.New(map[string]string{"user-agent": "grpc-go/1.15.0"})
		ctx := metadata.NewIncomingContext(t.Context(), md)
		err := userAgentEnforcer(ctx, clientName, semverConstraint)
		require.ErrorContains(t, err, "unsatisfied client version constraint: ^1")
	})
	t.Run("Test invalid version", func(t *testing.T) {
		md := metadata.New(map[string]string{"user-agent": "argo-cd/super"})
		ctx := metadata.NewIncomingContext(t.Context(), md)
		err := userAgentEnforcer(ctx, clientName, semverConstraint)
		require.ErrorContains(t, err, "could not parse version")
	})
}
