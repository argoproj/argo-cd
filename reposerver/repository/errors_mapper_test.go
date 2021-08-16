package repository

import (
	"errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mapError(t *testing.T) {
	var ok bool
	assert.Equal(t, mapError(nil), nil)

	defaultErrorMsg := "default error"
	defaultError := mapError(errors.New(defaultErrorMsg))
	_, ok = defaultError.(interface{ GRPCStatus() *status.Status })
	assert.Equal(t, ok, false)
	assert.Equal(t, defaultError.Error(), defaultErrorMsg)

	grpcErrorMsg := "grpc error"
	grpcError := mapError(status.Errorf(codes.Unknown, grpcErrorMsg))
	se, ok := grpcError.(interface{ GRPCStatus() *status.Status })
	assert.Equal(t, ok, true)
	assert.Equal(t, se.GRPCStatus().Code(), codes.Unknown)
	assert.Equal(t, se.GRPCStatus().Message(), grpcErrorMsg)

	notFoundMsg := "repository not found"
	notFound := mapError(status.Errorf(codes.NotFound, notFoundMsg))
	se1, ok := notFound.(interface{ GRPCStatus() *status.Status })
	assert.Equal(t, ok, true)
	assert.Equal(t, se1.GRPCStatus().Code(), codes.NotFound)
	assert.Equal(t, se1.GRPCStatus().Message(), notFoundMsg)
}
