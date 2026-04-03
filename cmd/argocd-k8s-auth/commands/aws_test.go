package commands

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSignedRequest(t *testing.T) {
	t.Parallel()

	t.Run("returns error when context is cancelled", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		url, err := getSignedRequest(ctx, "my-cluster", "", "")

		require.ErrorIs(t, err, context.Canceled)
		assert.Empty(t, url)
	})

	t.Run("returns error for non-existent profile", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		profile := "argocd-k8s-auth-test-nonexistent-profile-12345"

		url, err := getSignedRequest(ctx, "my-cluster", "", profile)

		require.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), "configuration", "error should mention configuration load failed")
	})

	t.Run("returns error when roleARN is provided and assume role fails", func(t *testing.T) {
		t.Parallel()
		ctx := context.Background()
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
			config.WithRegion("us-east-1"),
		)
		require.NoError(t, err)

		url, err := getSignedRequestWithConfig(ctx, "my-cluster", "arn:aws:iam::123456789012:role/NonExistentRole", cfg)

		require.Error(t, err)
		assert.Empty(t, url)
		assert.Contains(t, err.Error(), "presigning", "error should mention presigning failed when assume role is used")
	})
}

func TestGetSignedRequestWithRetry(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	t.Run("will return signed request on first attempt", func(t *testing.T) {
		// given
		t.Parallel()
		mock := &signedRequestMock{
			returnFunc: func(_ *signedRequestMock) (string, error) {
				return "token", nil
			},
		}

		// when
		signed, err := getSignedRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-name", "", "", mock.getSignedRequestMock)

		// then
		require.NoError(t, err)
		assert.Equal(t, "token", signed)
	})
	t.Run("will return signed request on third attempt", func(t *testing.T) {
		// given
		t.Parallel()
		mock := &signedRequestMock{
			returnFunc: func(m *signedRequestMock) (string, error) {
				if m.getSignedRequestCalls < 3 {
					return "", errors.New("some error")
				}
				return "token", nil
			},
		}

		// when
		signed, err := getSignedRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-name", "", "", mock.getSignedRequestMock)

		// then
		require.NoError(t, err)
		assert.Equal(t, "token", signed)
	})
	t.Run("will return error on timeout", func(t *testing.T) {
		// given
		t.Parallel()
		mock := &signedRequestMock{
			returnFunc: func(_ *signedRequestMock) (string, error) {
				return "", errors.New("some error")
			},
		}

		// when
		signed, err := getSignedRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-name", "", "", mock.getSignedRequestMock)

		// then
		require.Error(t, err)
		assert.Empty(t, signed)
	})
}

type signedRequestMock struct {
	getSignedRequestCalls int
	returnFunc            func(m *signedRequestMock) (string, error)
}

func (m *signedRequestMock) getSignedRequestMock(_ context.Context, _, _ string, _ string) (string, error) {
	m.getSignedRequestCalls++
	return m.returnFunc(m)
}
