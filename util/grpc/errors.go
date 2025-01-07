package grpc

import (
	"context"
	"errors"

	giterr "github.com/go-git/go-git/v5/plumbing/transport"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
)

func rewrapError(err error, code codes.Code) error {
	return status.Error(code, err.Error())
}

func gitErrToGRPC(err error) error {
	if err == nil {
		return err
	}
	errMsg := err.Error()
	if grpcStatus := UnwrapGRPCStatus(err); grpcStatus != nil {
		errMsg = grpcStatus.Message()
	}

	switch errMsg {
	case giterr.ErrRepositoryNotFound.Error():
		err = rewrapError(errors.New(errMsg), codes.NotFound)
	}
	return err
}

// UnwrapGRPCStatus will attempt to cast the given error into a grpc Status
// object unwrapping all existing inner errors. Will return nil if none of the
// nested errors can be casted.
func UnwrapGRPCStatus(err error) *status.Status {
	if se, ok := err.(interface{ GRPCStatus() *status.Status }); ok {
		return se.GRPCStatus()
	}
	e := errors.Unwrap(err)
	if e == nil {
		return nil
	}
	return UnwrapGRPCStatus(e)
}

// kubeErrToGRPC converts a Kubernetes error into a gRPC code + error. The gRPC code we translate
// it to is significant, because it eventually maps back to an HTTP status code determined by
// grpc-gateway. See:
// https://github.com/grpc-ecosystem/grpc-gateway/blob/v2.11.3/runtime/errors.go#L36
// https://go.dev/src/net/http/status.go
func kubeErrToGRPC(err error) error {
	/*
		Unmapped source Kubernetes API errors as of 2022-10-05:
		* IsGone => 410 (DEPRECATED by ResourceExpired)
		* IsResourceExpired => 410
		* IsUnexpectedServerError
		* IsUnexpectedObjectError

		Unmapped target gRPC codes as of 2022-10-05:
		* Canceled Code = 1
		* Unknown Code = 2
		* OutOfRange Code = 11
		* DataLoss Code = 15
	*/

	switch {
	case apierr.IsNotFound(err):
		err = rewrapError(err, codes.NotFound)
	case apierr.IsAlreadyExists(err):
		err = rewrapError(err, codes.AlreadyExists)
	case apierr.IsInvalid(err):
		err = rewrapError(err, codes.InvalidArgument)
	case apierr.IsMethodNotSupported(err):
		err = rewrapError(err, codes.Unimplemented)
	case apierr.IsServiceUnavailable(err):
		err = rewrapError(err, codes.Unavailable)
	case apierr.IsBadRequest(err):
		err = rewrapError(err, codes.FailedPrecondition)
	case apierr.IsUnauthorized(err):
		err = rewrapError(err, codes.Unauthenticated)
	case apierr.IsForbidden(err):
		err = rewrapError(err, codes.PermissionDenied)
	case apierr.IsTimeout(err):
		err = rewrapError(err, codes.DeadlineExceeded)
	case apierr.IsServerTimeout(err):
		err = rewrapError(err, codes.Unavailable)
	case apierr.IsConflict(err):
		err = rewrapError(err, codes.Aborted)
	case apierr.IsTooManyRequests(err):
		err = rewrapError(err, codes.ResourceExhausted)
	case apierr.IsInternalError(err):
		err = rewrapError(err, codes.Internal)
	default:
		// This is necessary as GRPC Status don't support wrapped errors:
		// https://github.com/grpc/grpc-go/issues/2934
		if grpcStatus := UnwrapGRPCStatus(err); grpcStatus != nil {
			err = status.Error(grpcStatus.Code(), grpcStatus.Message())
		}
	}
	return err
}

// ErrorCodeGitUnaryServerInterceptor replaces Kubernetes errors with relevant gRPC equivalents, if any.
func ErrorCodeGitUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		resp, err = handler(ctx, req)
		return resp, gitErrToGRPC(err)
	}
}

// ErrorCodeGitStreamServerInterceptor replaces Kubernetes errors with relevant gRPC equivalents, if any.
func ErrorCodeGitStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		return gitErrToGRPC(err)
	}
}

// ErrorCodeK8sUnaryServerInterceptor replaces Kubernetes errors with relevant gRPC equivalents, if any.
func ErrorCodeK8sUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		resp, err = handler(ctx, req)
		return resp, kubeErrToGRPC(err)
	}
}

// ErrorCodeK8sStreamServerInterceptor replaces Kubernetes errors with relevant gRPC equivalents, if any.
func ErrorCodeK8sStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		return kubeErrToGRPC(err)
	}
}
