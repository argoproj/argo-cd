package commands

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSignedRequestWithRetry(t *testing.T) {
	ctx := context.Background()

	t.Run("will return signed request on first attempt", func(t *testing.T) {
		// given
		t.Parallel()
		mock := &signedRequestMock{
			returnFunc: func(m *signedRequestMock) (string, error) {
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
					return "", fmt.Errorf("some error")
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
			returnFunc: func(m *signedRequestMock) (string, error) {
				return "", fmt.Errorf("some error")
			},
		}

		// when
		signed, err := getSignedRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-name", "", "", mock.getSignedRequestMock)

		// then
		require.Error(t, err)
		assert.Equal(t, "", signed)
	})
}

type signedRequestMock struct {
	getSignedRequestCalls int
	returnFunc            func(m *signedRequestMock) (string, error)
}

func (m *signedRequestMock) getSignedRequestMock(clusterName, roleARN string, profile string) (string, error) {
	m.getSignedRequestCalls++
	return m.returnFunc(m)
}
