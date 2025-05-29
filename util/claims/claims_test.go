package claims

import (
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func TestGetUserIdentifier(t *testing.T) {
	tests := []struct {
		name   string
		claims jwt.MapClaims
		want   string
	}{
		{
			name: "when both dex and sub defined - prefer dex user_id",
			claims: jwt.MapClaims{
				"sub": "ignored:login",
				"federated_claims": map[string]any{
					"user_id": "dex-user",
				},
			},
			want: "dex-user",
		},
		{
			name: "when both dex and sub defined but dex user_id empty - fallback to sub",
			claims: jwt.MapClaims{
				"sub": "test:apiKey",
				"federated_claims": map[string]any{
					"user_id": "",
				},
			},
			want: "test:apiKey",
		},
		{
			name: "when only sub is defined (no dex) - use sub",
			claims: jwt.MapClaims{
				"sub": "admin:login",
			},
			want: "admin:login",
		},
		{
			name:   "when neither dex nor sub defined - return empty",
			claims: jwt.MapClaims{},
			want:   "",
		},
		{
			name:   "nil claims",
			claims: nil,
			want:   "",
		},
		{
			name: "invalid subject",
			claims: jwt.MapClaims{
				"sub": nil,
			},
			want: "",
		},
		{
			name: "invalid federated_claims",
			claims: jwt.MapClaims{
				"sub":              "test:apiKey",
				"federated_claims": "invalid",
			},
			want: "test:apiKey",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetUserIdentifier(tt.claims)
			assert.Equal(t, tt.want, got)
		})
	}
}
