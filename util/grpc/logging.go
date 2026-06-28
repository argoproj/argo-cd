package grpc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/golang-jwt/jwt/v5"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/selector"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func logRequest(ctx context.Context, entry *logrus.Entry, info string, pbMsg any, logClaims bool) {
	if logClaims {
		claims := ctx.Value("claims")
		mapClaims, ok := claims.(jwt.MapClaims)
		if ok {
			claimsCopy := make(map[string]any)
			for k, v := range mapClaims {
				if k != "groups" || entry.Logger.IsLevelEnabled(logrus.DebugLevel) {
					claimsCopy[k] = v
				}
			}
			if data, err := json.Marshal(claimsCopy); err == nil {
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

type reporter struct {
	ctx       context.Context
	entry     *logrus.Entry
	logClaims bool
	info      string
}

func (r *reporter) PostCall(_ error, _ time.Duration) {}

func (r *reporter) PostMsgSend(_ any, _ error, _ time.Duration) {}

func (r *reporter) PostMsgReceive(payload any, err error, _ time.Duration) {
	if err == nil {
		logRequest(r.ctx, r.entry, r.info, payload, r.logClaims)
	}
}

func PayloadStreamServerInterceptor(entry *logrus.Entry, logClaims bool, decider func(context.Context, interceptors.CallMeta) bool) grpc.StreamServerInterceptor {
	return selector.StreamServerInterceptor(interceptors.StreamServerInterceptor(reportable(entry, "streaming", logClaims)), selector.MatchFunc(decider))
}

func PayloadUnaryServerInterceptor(entry *logrus.Entry, logClaims bool, decider func(context.Context, interceptors.CallMeta) bool) grpc.UnaryServerInterceptor {
	return selector.UnaryServerInterceptor(interceptors.UnaryServerInterceptor(reportable(entry, "unary", logClaims)), selector.MatchFunc(decider))
}

func reportable(entry *logrus.Entry, callType string, logClaims bool) interceptors.CommonReportableFunc {
	return func(ctx context.Context, c interceptors.CallMeta) (interceptors.Reporter, context.Context) {
		return &reporter{
			ctx:       ctx,
			entry:     entry,
			info:      fmt.Sprintf("received %s call %s", callType, c.FullMethod()),
			logClaims: logClaims,
		}, ctx
	}
}

// InterceptorLogger adapts logrus logger to interceptor logger.
func InterceptorLogger(l logrus.FieldLogger) logging.Logger {
	return logging.LoggerFunc(func(_ context.Context, lvl logging.Level, msg string, fields ...any) {
		f := make(map[string]any, len(fields)/2)
		i := logging.Fields(fields).Iterator()
		for i.Next() {
			k, v := i.At()
			f[k] = v
		}
		l := l.WithFields(f)

		switch lvl {
		case logging.LevelDebug:
			l.Debug(msg)
		case logging.LevelInfo:
			l.Info(msg)
		case logging.LevelWarn:
			l.Warn(msg)
		case logging.LevelError:
			l.Error(msg)
		default:
			panic(fmt.Sprintf("unknown level %v", lvl))
		}
	})
}
