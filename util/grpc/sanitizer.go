package grpc

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

type sanitizerKey string

const (
	contextKey sanitizerKey = "sanitizer"
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
	AddRegexReplacement(regex *regexp.Regexp, replacement string)
}

type sanitizer struct {
	replacers []func(in string) string
}

// NewSanitizer returns a new Sanitizer instance
func NewSanitizer() *sanitizer {
	return &sanitizer{}
}

// AddReplacement adds a replacement to the Sanitizer
func (s *sanitizer) AddReplacement(val string, replacement string) {
	s.replacers = append(s.replacers, func(in string) string {
		return strings.ReplaceAll(in, val, replacement)
	})
}

// AddRegexReplacement adds a replacement to the sanitizer using regexp
func (s *sanitizer) AddRegexReplacement(regex *regexp.Regexp, replacement string) {
	s.replacers = append(s.replacers, func(in string) string {
		return regex.ReplaceAllString(in, replacement)
	})
}

// Replace replaces all occurrences of the configured values in the sanitizer with the replacements
func (s *sanitizer) Replace(val string) string {
	for _, replacer := range s.replacers {
		val = replacer(val)
	}
	return val
}
