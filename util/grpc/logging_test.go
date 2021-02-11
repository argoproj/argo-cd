package grpc

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"github.com/argoproj/argo-cd/pkg/apiclient/account"
)

func Test_JSONLogging(t *testing.T) {
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	var buf bytes.Buffer
	l.SetOutput(&buf)
	entry := logrus.NewEntry(l)

	c := context.Background()
	req := new(account.CreateTokenRequest)
	req.Name = "create-token-name"
	info := &grpc.UnaryServerInfo{}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return nil, nil
	}
	decider := func(ctx context.Context, fullMethodName string, servingObject interface{}) bool {
		return true
	}
	interceptor := PayloadUnaryServerInterceptor(entry, false, decider)
	_, err := interceptor(c, req, info, handler)
	assert.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, fmt.Sprintf(`"grpc.request.content":{"name":"%s"`, req.Name))
}
