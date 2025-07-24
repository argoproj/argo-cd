package claims

import (
	"reflect"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			got := tt.claims.GetUserIdentifier()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapClaimsToArgoClaims(t *testing.T) {
	expectedExpiredAt := jwt.NewNumericDate(time.Now().Add(time.Hour))
	expectedIssuedAt := jwt.NewNumericDate(time.Now().Add(time.Hour * -2))
	expectedNotBefore := jwt.NewNumericDate(time.Now().Add(time.Hour * -3))

	tests := []struct {
		name    string
		claims  jwt.MapClaims
		want    *ArgoClaims
		wantErr bool
	}{
		{
			name:   "nil claims",
			claims: nil,
			want:   &ArgoClaims{},
		},
		{
			name:   "empty claims",
			claims: jwt.MapClaims{},
			want:   &ArgoClaims{},
		},
		{
			name: "invalid claims",
			claims: jwt.MapClaims{
				"email_verified": "not-a-bool",
			},
			wantErr: true,
		},
		{
			name: "all registered known claims",
			claims: jwt.MapClaims{
				"jti": "jti",
				"iss": "iss",
				"sub": "sub",
				"aud": "aud",
				"iat": expectedIssuedAt.Unix(),
				"exp": expectedExpiredAt.Unix(),
				"nbf": expectedNotBefore.Unix(),
			},
			want: &ArgoClaims{
				RegisteredClaims: jwt.RegisteredClaims{
					ID:        "jti",
					Issuer:    "iss",
					Subject:   "sub",
					Audience:  jwt.ClaimStrings{"aud"},
					ExpiresAt: expectedExpiredAt,
					IssuedAt:  expectedIssuedAt,
					NotBefore: expectedNotBefore,
				},
			},
		},
		{
			name: "all argo claims",
			claims: jwt.MapClaims{
				"email":          "email@test.com",
				"email_verified": true,
				"name":           "the-name",
				"groups": []string{
					"my-org:my-team2",
					"my-org:my-team1",
				},
				"federated_claims": map[string]any{
					"connector_id": "my-connector",
					"user_id":      "user-id",
				},
			},
			want: &ArgoClaims{
				Email:         "email@test.com",
				EmailVerified: true,
				Name:          "the-name",
				FederatedClaims: &FederatedClaims{
					ConnectorID: "my-connector",
					UserID:      "user-id",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := MapClaimsToArgoClaims(tt.claims)
			if tt.wantErr {
				assert.Error(t, err, "MapClaimsToArgoClaims()")
			} else {
				require.NoError(t, err, "MapClaimsToArgoClaims()")
				assert.Truef(t, reflect.DeepEqual(got, tt.want), "MapClaimsToArgoClaims() = %v, want %v", got, tt.want)
			}
		})
	}
}
