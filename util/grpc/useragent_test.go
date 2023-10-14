package grpc

import (
	"context"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func Test_UserAgentEnforcer(t *testing.T) {
	t.Run("Test enforcing valid user-agent", func(t *testing.T) {
		clientName := "argo-cd"
		constraintStr := "^1"
		semverConstraint, _ := semver.NewConstraint(constraintStr)
		md := metadata.New(map[string]string{"user-agent": "argo-cd/1.0"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		err := userAgentEnforcer(ctx, clientName, constraintStr, semverConstraint)
		require.NoError(t, err)
	})
	t.Run("Test enforcing ignored user-agent", func(t *testing.T) {
		clientName := "argo-cd"
		constraintStr := "^1"
		semverConstraint, _ := semver.NewConstraint(constraintStr)
		md := metadata.New(map[string]string{"user-agent": "flux/3.0"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		err := userAgentEnforcer(ctx, clientName, constraintStr, semverConstraint)
		require.NoError(t, err)
	})
	t.Run("Test enforcing user-agent with version not matching constraint", func(t *testing.T) {
		clientName := "argo-cd"
		constraintStr := "^1"
		semverConstraint, _ := semver.NewConstraint(constraintStr)
		md := metadata.New(map[string]string{"user-agent": "argo-cd/3.0"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		err := userAgentEnforcer(ctx, clientName, constraintStr, semverConstraint)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsatisfied client version constraint")
	})
	t.Run("Test legacy user-agent", func(t *testing.T) {
		clientName := "argo-cd"
		constraintStr := "^1"
		semverConstraint, _ := semver.NewConstraint(constraintStr)
		md := metadata.New(map[string]string{"user-agent": "grpc-go/1.15.0"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		err := userAgentEnforcer(ctx, clientName, constraintStr, semverConstraint)
		require.Error(t, err)
		require.Contains(t, err.Error(), "unsatisfied client version constraint")
	})
	t.Run("Test invalid version", func(t *testing.T) {
		clientName := "argo-cd"
		constraintStr := "^1"
		semverConstraint, _ := semver.NewConstraint(constraintStr)
		md := metadata.New(map[string]string{"user-agent": "argo-cd/super"})
		ctx := metadata.NewIncomingContext(context.Background(), md)
		err := userAgentEnforcer(ctx, clientName, constraintStr, semverConstraint)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not parse version")
	})
}
