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
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func Test_JSONLogging(t *testing.T) {
	t.Parallel()
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

// Test_JSONLogging_EmbeddedKubernetesType is a regression test for
// https://github.com/argoproj/argo-cd/issues/29869: a request message that embeds a Kubernetes API
// type (here, an Application's metav1.ObjectMeta) must still produce a log entry, even though gogo's
// jsonpb can no longer marshal that type directly.
func Test_JSONLogging_EmbeddedKubernetesType(t *testing.T) {
	t.Parallel()
	l := logrus.New()
	l.SetFormatter(&logrus.JSONFormatter{})
	var buf bytes.Buffer
	l.SetOutput(&buf)
	entry := logrus.NewEntry(l)

	c := t.Context()
	// ApplicationService/Update sends *v1alpha1.Application directly; it embeds metav1.ObjectMeta,
	// which is what gogo jsonpb can no longer marshal (see the issue for the underlying k8s change).
	req := &v1alpha1.Application{
		Name: "my-app",
	}
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
	// Before the fix, MarshalJSON on the embedded ObjectMeta fails, logrus's JSON formatter drops the
	// whole entry, and buf stays empty.
	assert.Contains(t, out, `"msg":"received unary call`)
	assert.Contains(t, out, `"my-app"`)
}

// unmarshalableField fails both serializers jsonpbMarshalleble.MarshalJSON falls back between:
// gogo's jsonpb rejects it for not implementing proto.Message itself (the same failure mode as the
// Kubernetes types above), and its channel field is a type encoding/json cannot marshal either.
type unmarshalableField struct {
	Ch chan int `protobuf:"bytes,1,opt,name=ch" json:"ch"`
}

// unmarshalableMessage is a minimal proto.Message whose sole field is an unmarshalableField.
type unmarshalableMessage struct {
	Field unmarshalableField `protobuf:"bytes,1,opt,name=field" json:"field"`
}

func (*unmarshalableMessage) Reset()         {}
func (*unmarshalableMessage) String() string { return "" }
func (*unmarshalableMessage) ProtoMessage()  {}

// Test_JSONLogging_BothSerializersFail is a regression test for the case where jsonpb.Marshal and
// its encoding/json fallback both fail: MarshalJSON must report an error that identifies both
// failures rather than returning an invalid payload.
func Test_JSONLogging_BothSerializersFail(t *testing.T) {
	t.Parallel()
	j := &jsonpbMarshalleble{&unmarshalableMessage{}}

	data, err := j.MarshalJSON()

	require.Error(t, err)
	assert.Nil(t, data)
	assert.Contains(t, err.Error(), "jsonpb serializer failed")
	assert.Contains(t, err.Error(), "encoding/json fallback failed")
}

func Test_logRequest(t *testing.T) {
	t.Parallel()
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
		t.Parallel()
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
		t.Parallel()
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
