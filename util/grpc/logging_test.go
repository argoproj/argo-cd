package grpc

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/argoproj/argo-cd/v3/pkg/apiclient/account"
)

func Test_JSONLogging(t *testing.T) {
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	var buf bytes.Buffer
	l.SetOutput(&buf)
	entry := logrus.NewEntry(l)

	c := t.Context()
	req := new(account.CreateTokenRequest)
	req.Name = "create-token-name"
	info := &grpc.UnaryServerInfo{}
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, nil
	}
	decider := func(_ context.Context, _ interceptors.CallMeta) bool {
		return true
	}
	interceptor := PayloadUnaryServerInterceptor(entry, false, decider)
	_, err := interceptor(c, req, info, handler)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, fmt.Sprintf(`"grpc.request.content":{"name":%q`, req.Name))
}

func Test_logRequest(t *testing.T) {
	c := t.Context()
	//nolint:staticcheck
	c = context.WithValue(c, "claims", jwt.MapClaims{"groups": []string{"expected-group-claim"}})
	req := new(account.CreateTokenRequest)
	req.Name = "create-token-name"
	info := &grpc.UnaryServerInfo{}
	handler := func(_ context.Context, _ any) (any, error) {
		return nil, nil
	}
	decider := func(_ context.Context, _ interceptors.CallMeta) bool {
		return true
	}

	t.Run("with debug enabled, group claims are logged", func(t *testing.T) {
		l := logrus.New()
		l.SetFormatter(&logrus.JSONFormatter{})
		var buf bytes.Buffer
		l.SetOutput(&buf)
		l.SetLevel(logrus.DebugLevel)
		entry := logrus.NewEntry(l)

		interceptor := PayloadUnaryServerInterceptor(entry, true, decider)

		_, err := interceptor(c, req, info, handler)
		require.NoError(t, err)

		out := buf.String()
		assert.Contains(t, out, "expected-group-claim")
	})

	t.Run("with debug not enabled, group claims aren't logged", func(t *testing.T) {
		l := logrus.New()
		l.SetFormatter(&logrus.JSONFormatter{})
		var buf bytes.Buffer
		l.SetOutput(&buf)
		l.SetLevel(logrus.InfoLevel)
		entry := logrus.NewEntry(l)

		interceptor := PayloadUnaryServerInterceptor(entry, true, decider)

		_, err := interceptor(c, req, info, handler)
		require.NoError(t, err)

		out := buf.String()
		assert.NotContains(t, out, "expected-group-claim")
	})
}
