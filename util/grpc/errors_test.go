package grpc

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_gitErrToGRPC(t *testing.T) {
	var ok bool
	require.NoError(t, gitErrToGRPC(nil))

	defaultErrorMsg := "default error"
	defaultError := gitErrToGRPC(errors.New(defaultErrorMsg))
	_, ok = defaultError.(interface{ GRPCStatus() *status.Status })
	assert.False(t, ok)
	assert.Equal(t, defaultError.Error(), defaultErrorMsg)

	grpcErrorMsg := "grpc error"
	grpcError := gitErrToGRPC(status.Error(codes.Unknown, grpcErrorMsg))
	se, ok := grpcError.(interface{ GRPCStatus() *status.Status })
	assert.True(t, ok)
	assert.Equal(t, codes.Unknown, se.GRPCStatus().Code())
	assert.Equal(t, se.GRPCStatus().Message(), grpcErrorMsg)

	notFoundMsg := "repository not found"
	notFound := gitErrToGRPC(status.Error(codes.NotFound, notFoundMsg))
	se1, ok := notFound.(interface{ GRPCStatus() *status.Status })
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, se1.GRPCStatus().Code())
	assert.Equal(t, se1.GRPCStatus().Message(), notFoundMsg)
}

func Test_kubeErrToGRPC(t *testing.T) {
	type testCase struct {
		name               string
		givenErrFn         func() error
		expectedErrFn      func() error
		expectedGRPCStatus *status.Status
	}
	newForbiddenError := func() error {
		gr := schema.GroupResource{
			Group:    "apps",
			Resource: "Deployment",
		}
		return apierr.NewForbidden(gr, "some-app", fmt.Errorf("authentication error"))
	}
	newUnauthorizedError := func() error {
		return apierr.NewUnauthorized("unauthenticated")
	}
	cases := []*testCase{
		{
			name: "will return standard error if not grpc status",
			givenErrFn: func() error {
				return fmt.Errorf("standard error")
			},
			expectedErrFn: func() error {
				return fmt.Errorf("standard error")
			},
			expectedGRPCStatus: nil,
		},
		{
			name: "will return wrapped status if nested in err",
			givenErrFn: func() error {
				grpcStatus := status.New(codes.NotFound, "Not found")
				return fmt.Errorf("wrapped status: %w", grpcStatus.Err())
			},
			expectedErrFn: func() error {
				grpcStatus := status.New(codes.NotFound, "Not found")
				return grpcStatus.Err()
			},
			expectedGRPCStatus: status.New(codes.NotFound, "Not found"),
		},
		{
			name: "will return permission denied if apierr.IsForbidden",
			givenErrFn: func() error {
				return newForbiddenError()
			},
			expectedErrFn: func() error {
				err := newForbiddenError()
				grpcStatus := status.New(codes.PermissionDenied, err.Error())
				return grpcStatus.Err()
			},
			expectedGRPCStatus: status.New(codes.PermissionDenied, newForbiddenError().Error()),
		},
		{
			name: "will return unauthenticated if apierr.IsUnauthorized",
			givenErrFn: func() error {
				return newUnauthorizedError()
			},
			expectedErrFn: func() error {
				err := newUnauthorizedError()
				grpcStatus := status.New(codes.Unauthenticated, err.Error())
				return grpcStatus.Err()
			},
			expectedGRPCStatus: status.New(codes.Unauthenticated, newUnauthorizedError().Error()),
		},
		{
			name: "will return Unavailable if apierr.IsServerTimeout",
			givenErrFn: func() error {
				return apierr.NewServerTimeout(schema.GroupResource{}, "update", 1)
			},
			expectedErrFn: func() error {
				err := apierr.NewServerTimeout(schema.GroupResource{}, "update", 1)
				grpcStatus := status.New(codes.Unavailable, err.Error())
				return grpcStatus.Err()
			},
			expectedGRPCStatus: status.New(codes.Unavailable, apierr.NewServerTimeout(schema.GroupResource{}, "update", 1).Error()),
		},
		{
			name: "will return Aborted if apierr.IsConflict",
			givenErrFn: func() error {
				return apierr.NewConflict(schema.GroupResource{}, "foo", errors.New("foo"))
			},
			expectedErrFn: func() error {
				err := apierr.NewConflict(schema.GroupResource{}, "foo", errors.New("foo"))
				grpcStatus := status.New(codes.Aborted, err.Error())
				return grpcStatus.Err()
			},
			expectedGRPCStatus: status.New(codes.Aborted, apierr.NewConflict(schema.GroupResource{}, "foo", errors.New("foo")).Error()),
		},
		{
			name: "will return ResourceExhausted if apierr.IsTooManyRequests",
			givenErrFn: func() error {
				return apierr.NewTooManyRequests("foo", 1)
			},
			expectedErrFn: func() error {
				err := apierr.NewTooManyRequests("foo", 1)
				grpcStatus := status.New(codes.ResourceExhausted, err.Error())
				return grpcStatus.Err()
			},
			expectedGRPCStatus: status.New(codes.ResourceExhausted, apierr.NewTooManyRequests("foo", 1).Error()),
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			// when
			err := kubeErrToGRPC(c.givenErrFn())

			// then
			assert.Equal(t, c.expectedErrFn(), err, "error comparison mismatch")
			grpcStatus := UnwrapGRPCStatus(err)
			assert.Equal(t, c.expectedGRPCStatus, grpcStatus, "grpc status mismatch")
		})
	}
}
