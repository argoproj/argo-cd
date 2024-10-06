package common

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test env var not set for EnvGRPCKeepAliveMin
func Test_GRPCKeepAliveMinNotSet(t *testing.T) {
	grpcKeepAliveMin := GetGRPCKeepAliveEnforcementMinimum()
	grpcKeepAliveExpectedMin := defaultGRPCKeepAliveEnforcementMinimum
	assert.Equal(t, grpcKeepAliveExpectedMin, grpcKeepAliveMin)

	grpcKeepAliveTime := GetGRPCKeepAliveTime()
	assert.Equal(t, 2*grpcKeepAliveExpectedMin, grpcKeepAliveTime)
}

// Test valid env var set for EnvGRPCKeepAliveMin
func Test_GRPCKeepAliveMinIsSet(t *testing.T) {
	numSeconds := 15
	os.Setenv(EnvGRPCKeepAliveMin, fmt.Sprintf("%ds", numSeconds))

	grpcKeepAliveMin := GetGRPCKeepAliveEnforcementMinimum()
	grpcKeepAliveExpectedMin := time.Duration(numSeconds) * time.Second
	assert.Equal(t, grpcKeepAliveExpectedMin, grpcKeepAliveMin)

	grpcKeepAliveTime := GetGRPCKeepAliveTime()
	assert.Equal(t, 2*grpcKeepAliveExpectedMin, grpcKeepAliveTime)
}

// Test invalid env var set for EnvGRPCKeepAliveMin
func Test_GRPCKeepAliveMinIncorrectlySet(t *testing.T) {
	numSeconds := 15
	os.Setenv(EnvGRPCKeepAliveMin, fmt.Sprintf("%d", numSeconds))

	grpcKeepAliveMin := GetGRPCKeepAliveEnforcementMinimum()
	grpcKeepAliveExpectedMin := defaultGRPCKeepAliveEnforcementMinimum
	assert.Equal(t, grpcKeepAliveExpectedMin, grpcKeepAliveMin)

	grpcKeepAliveTime := GetGRPCKeepAliveTime()
	assert.Equal(t, 2*grpcKeepAliveExpectedMin, grpcKeepAliveTime)
}
