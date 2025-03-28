package commands

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSignedRequestWithRetry(t *testing.T) {
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

func (m *signedRequestMock) getSignedRequestMock(_, _ string, _ string) (string, error) {
	m.getSignedRequestCalls++
	return m.returnFunc(m)
}
