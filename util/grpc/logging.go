package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang-jwt/jwt/v5"
	grpc_logging "github.com/grpc-ecosystem/go-grpc-middleware/logging"
	ctx_logrus "github.com/grpc-ecosystem/go-grpc-middleware/tags/logrus"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func logRequest(ctx context.Context, entry *logrus.Entry, info string, pbMsg any, logClaims bool) {
	if logClaims {
		claims := ctx.Value("claims")
		mapClaims, ok := claims.(jwt.MapClaims)
		if ok {
			copy := make(map[string]any)
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

func (l *loggingServerStream) SendMsg(m any) error {
	return l.ServerStream.SendMsg(m)
}

func (l *loggingServerStream) RecvMsg(m any) error {
	err := l.ServerStream.RecvMsg(m)
	if err == nil {
		logRequest(l.ServerStream.Context(), l.entry, l.info, m, l.logClaims)
	}
	return err
}

func PayloadStreamServerInterceptor(entry *logrus.Entry, logClaims bool, decider grpc_logging.ServerPayloadLoggingDecider) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if !decider(stream.Context(), info.FullMethod, srv) {
			return handler(srv, stream)
		}
		logEntry := entry.WithFields(ctx_logrus.Extract(stream.Context()).Data)
		newStream := &loggingServerStream{ServerStream: stream, entry: logEntry, logClaims: logClaims, info: "received streaming call " + info.FullMethod}
		return handler(srv, newStream)
	}
}

func PayloadUnaryServerInterceptor(entry *logrus.Entry, logClaims bool, decider grpc_logging.ServerPayloadLoggingDecider) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if !decider(ctx, info.FullMethod, info.Server) {
			return handler(ctx, req)
		}
		logEntry := entry.WithFields(ctx_logrus.Extract(ctx).Data)
		logRequest(ctx, logEntry, "received unary call "+info.FullMethod, req, logClaims)
		resp, err := handler(ctx, req)
		return resp, err
	}
}
