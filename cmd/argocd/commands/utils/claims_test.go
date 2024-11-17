package utils

import (
	"testing"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
)

func TestGetUserIdentifier(t *testing.T) {
	tests := []struct {
		name   string
		claims jwt.MapClaims
		want   string
	}{
		{
			name: "federated claims present",
			claims: jwt.MapClaims{
				"sub": "ignored:login",
				"federated_claims": map[string]interface{}{
					"user_id": "dex-user",
				},
			},
			want: "dex-user",
		},
		{
			name: "empty federated claims falls back to sub",
			claims: jwt.MapClaims{
				"sub": "test:apiKey",
				"federated_claims": map[string]interface{}{
					"user_id": "",
				},
			},
			want: "test:apiKey",
		},
		{
			name: "no federated claims uses sub",
			claims: jwt.MapClaims{
				"sub": "admin:login",
			},
			want: "admin:login",
		},
		{
			name:   "empty claims",
			claims: jwt.MapClaims{},
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
