package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	utiltest "github.com/argoproj/argo-cd/v2/util/test"
)

func Test_UnverifiedHasAudClaim(t *testing.T) {
	tokenForAud := func(t *testing.T, aud jwt.ClaimStrings) string {
		claims := jwt.RegisteredClaims{Audience: aud, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 24))}
		token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
		key, err := jwt.ParseRSAPrivateKeyFromPEM(utiltest.PrivateKey)
		require.NoError(t, err)
		tokenString, err := token.SignedString(key)
		require.NoError(t, err)
		return tokenString
	}

	testCases := []struct {
		name           string
		aud            jwt.ClaimStrings
		expectedHasAud bool
	}{
		{
			name:           "no audience",
			aud:            jwt.ClaimStrings{},
			expectedHasAud: false,
		},
		{
			name:           "one empty audience",
			aud:            jwt.ClaimStrings{""},
			expectedHasAud: true,
		},
		{
			name:           "one non-empty audience",
			aud:            jwt.ClaimStrings{"test"},
			expectedHasAud: true,
		},
	}

	for _, testCase := range testCases {
		testCaseCopy := testCase
		t.Run(testCaseCopy.name, func(t *testing.T) {
			t.Parallel()
			out, err := UnverifiedHasAudClaim(tokenForAud(t, testCaseCopy.aud))
			require.NoError(t, err)
			assert.Equal(t, testCaseCopy.expectedHasAud, out)
		})
	}
}
