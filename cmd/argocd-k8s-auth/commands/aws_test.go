package commands

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetSignedRequestWithRetry(t *testing.T) {
	ctx := context.Background()

	t.Run("will return signed request on first attempt", func(t *testing.T) {
		// given
		t.Parallel()
		mock := &signedRequestMock{
			returnFunc: func(m *signedRequestMock) (string, time.Time, error) {
				return "token", time.Now().Add(time.Minute * 30), nil
			},
		}

		// when
		signed, _, err := getSignedRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-name", "", mock.getSignedRequestMock)

		// then
		assert.NoError(t, err)
		assert.Equal(t, "token", signed)
	})
	t.Run("will return signed request on third attempt", func(t *testing.T) {
		// given
		t.Parallel()
		mock := &signedRequestMock{
			returnFunc: func(m *signedRequestMock) (string, time.Time, error) {
				if m.getSignedRequestCalls < 3 {
					return "", time.Time{}, fmt.Errorf("some error")
				}
				return "token", time.Now().Add(time.Minute * 30), nil
			},
		}

		// when
		signed, _, err := getSignedRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-name", "", mock.getSignedRequestMock)

		// then
		assert.NoError(t, err)
		assert.Equal(t, "token", signed)
	})
	t.Run("will return error on timeout", func(t *testing.T) {
		// given
		t.Parallel()
		mock := &signedRequestMock{
			returnFunc: func(m *signedRequestMock) (string, time.Time, error) {
				return "", time.Time{}, fmt.Errorf("some error")
			},
		}

		// when
		signed, _, err := getSignedRequestWithRetry(ctx, time.Second, time.Millisecond, "cluster-name", "", mock.getSignedRequestMock)

		// then
		assert.Error(t, err)
		assert.Equal(t, "", signed)
	})
}

type signedRequestMock struct {
	getSignedRequestCalls int
	returnFunc            func(m *signedRequestMock) (string, time.Time, error)
}

func (m *signedRequestMock) getSignedRequestMock(clusterName, roleARN string) (string, time.Time, error) {
	m.getSignedRequestCalls++
	return m.returnFunc(m)
}
