package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang-jwt/jwt/v4"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/logging"
	ctx_logrus "github.com/grpc-ecosystem/go-grpc-middleware/tags/logrus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func logRequest(entry *logrus.Entry, info string, pbMsg interface{}, ctx context.Context, logClaims bool) {
	if logClaims {
		claims := ctx.Value("claims")
		mapClaims, ok := claims.(jwt.MapClaims)
		if ok {
			copy := make(map[string]interface{})
			for k, v := range mapClaims {
				if k != "groups" || entry.Logger.IsLevelEnabled(logrus.DebugLevel) {
					copy[k] = v
				}
			}
			if data, err := json.Marshal(copy); err == nil {
				entry = entry.WithField("grpc.request.claims", string(data))
			}
		}
	}
	if p, ok := pbMsg.(proto.Message); ok {
		entry = entry.WithField("grpc.request.content", &jsonpbMarshalleble{p})
	}
	entry.Info(info)
}

type jsonpbMarshalleble struct {
	proto.Message
}

func (j *jsonpbMarshalleble) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	m := &jsonpb.Marshaler{}
	err := m.Marshal(&b, j.Message)
	if err != nil {
		return nil, fmt.Errorf("jsonpb serializer failed: %w", err)
	}
	return b.Bytes(), nil
}

type loggingServerStream struct {
	grpc.ServerStream
	entry     *logrus.Entry
	logClaims bool
	info      string
}

func (l *loggingServerStream) SendMsg(m interface{}) error {
	return l.ServerStream.SendMsg(m)
}

func (l *loggingServerStream) RecvMsg(m interface{}) error {
	err := l.ServerStream.RecvMsg(m)
	if err == nil {
		logRequest(l.entry, l.info, m, l.ServerStream.Context(), l.logClaims)
	}
	return err
}

func PayloadStreamServerInterceptor(entry *logrus.Entry, logClaims bool, decider grpc_logging.ServerPayloadLoggingDecider) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !decider(stream.Context(), info.FullMethod, srv) {
			return handler(srv, stream)
		}
		logEntry := entry.WithFields(ctx_logrus.Extract(stream.Context()).Data)
		newStream := &loggingServerStream{ServerStream: stream, entry: logEntry, logClaims: logClaims, info: fmt.Sprintf("received streaming call %s", info.FullMethod)}
		return handler(srv, newStream)
	}
}

func PayloadUnaryServerInterceptor(entry *logrus.Entry, logClaims bool, decider grpc_logging.ServerPayloadLoggingDecider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if !decider(ctx, info.FullMethod, info.Server) {
			return handler(ctx, req)
		}
		logEntry := entry.WithFields(ctx_logrus.Extract(ctx).Data)
		logRequest(logEntry, fmt.Sprintf("received unary call %s", info.FullMethod), req, ctx, logClaims)
		resp, err := handler(ctx, req)
		return resp, err
	}
}
