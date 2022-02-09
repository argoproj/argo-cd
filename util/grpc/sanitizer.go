package grpc

import (
	"errors"
	"strings"

	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	contextKey = "sanitizer"
)

// ErrorSanitizerUnaryServerInterceptor returns a new unary server interceptor that sanitizes error messages
// and provides Sanitizer to define replacements.
func ErrorSanitizerUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		sanitizer := NewSanitizer()
		resp, err = handler(ContextWithSanitizer(ctx, sanitizer), req)
		if err == nil {
			return resp, nil
		}

		if se, ok := err.(interface{ GRPCStatus() *status.Status }); ok {
			return resp, status.Error(se.GRPCStatus().Code(), sanitizer.Replace(se.GRPCStatus().Message()))
		}

		return resp, errors.New(sanitizer.Replace(err.Error()))
	}
}

// ContextWithSanitizer returns a new context with sanitizer set.
func ContextWithSanitizer(ctx context.Context, sanitizer Sanitizer) context.Context {
	return context.WithValue(ctx, contextKey, sanitizer)
}

// SanitizerFromContext returns sanitizer from context.
func SanitizerFromContext(ctx context.Context) (Sanitizer, bool) {
	res, ok := ctx.Value(contextKey).(Sanitizer)
	return res, ok
}

// Sanitizer provides methods to define list of strings and replacements
type Sanitizer interface {
	Replace(s string) string
	AddReplacement(val string, replacement string)
}

type sanitizer struct {
	replacements map[string]string
}

// NewSanitizer returns a new Sanitizer instance
func NewSanitizer() *sanitizer {
	return &sanitizer{
		replacements: map[string]string{},
	}
}

// AddReplacement adds a replacement to the Sanitizer
func (s *sanitizer) AddReplacement(val string, replacement string) {
	s.replacements[val] = replacement
}

// Replace replaces all occurrences of the configured values in the sanitizer with the replacements
func (s *sanitizer) Replace(val string) string {
	for k, v := range s.replacements {
		val = strings.Replace(val, k, v, -1)
	}
	return val
}
