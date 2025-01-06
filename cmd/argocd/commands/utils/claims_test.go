package utils

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestGetUserIdentifier(t *testing.T) {
	tests := []struct {
		name   string
		claims *ArgoClaims
		want   string
	}{
		{
			name: "when both dex and sub defined - prefer dex user_id",
			claims: &ArgoClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: "ignored:login",
				},
				FederatedClaims: &FederatedClaims{
					UserID: "dex-user",
				},
			},
			want: "dex-user",
		},
		{
			name: "when both dex and sub defined but dex user_id empty - fallback to sub",
			claims: &ArgoClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: "test:apiKey",
				},
				FederatedClaims: &FederatedClaims{
					UserID: "",
				},
			},
			want: "test:apiKey",
		},
		{
			name: "when only sub is defined (no dex) - use sub",
			claims: &ArgoClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					Subject: "admin:login",
				},
			},
			want: "admin:login",
		},
		{
			name:   "when neither dex nor sub defined - return empty",
			claims: &ArgoClaims{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetUserIdentifier(tt.claims)
			assert.Equal(t, tt.want, got)
		})
	}
}
