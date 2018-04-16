package grpc

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
)

func kubeErrToGRPC(err error) error {
	/*
		Unmapped source Kubernetes API errors as of 2018-04-16:
		* IsConflict => 409
		* IsGone => 410
		* IsResourceExpired => 410
		* IsServerTimeout => 500
		* IsTooManyRequests => 429
		* IsUnexpectedServerError => should probably be a panic
		* IsUnexpectedObjectError => should probably be a panic

		Unmapped target gRPC codes as of 2018-04-16:
		* Canceled Code = 1
		* Unknown Code = 2
		* ResourceExhausted Code = 8
		* Aborted Code = 10
		* OutOfRange Code = 11
		* DataLoss Code = 15
	*/

	rewrapError := func(err error, code codes.Code) error {
		return status.Errorf(code, err.Error())
	}

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
	case apierr.IsInternalError(err):
		err = rewrapError(err, codes.Internal)

	}
	return err
}

// ErrorCodeUnaryServerInterceptor replaces Kubernetes errors with relevant gRPC equivalents, if any.
func ErrorCodeUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		resp, err = handler(ctx, req)
		return resp, kubeErrToGRPC(err)
	}
}

// ErrorCodeStreamServerInterceptor replaces Kubernetes errors with relevant gRPC equivalents, if any.
func ErrorCodeStreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		return kubeErrToGRPC(err)
	}
}
