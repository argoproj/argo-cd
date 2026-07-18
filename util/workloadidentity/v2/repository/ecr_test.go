package repository

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubECRClient struct {
	output *ecr.GetAuthorizationTokenOutput
	err    error
}

func (s *stubECRClient) GetAuthorizationToken(_ context.Context, _ *ecr.GetAuthorizationTokenInput, _ ...func(*ecr.Options)) (*ecr.GetAuthorizationTokenOutput, error) {
	return s.output, s.err
}

func newTestECRAuthenticator(client ecrTokenAPI) *ECRAuthenticator {
	a := NewECRAuthenticator()
	a.newClient = func(_ aws.Config) ecrTokenAPI { return client }
	return a
}

func awsToken() *Token {
	return &Token{
		Type: TokenTypeAWS,
		AWSCredentials: &AWSCredentials{
			AccessKeyID:     "AKIA_TEST",
			SecretAccessKey: "secret",
			SessionToken:    "session",
			Region:          "us-east-1",
		},
	}
}

func TestECRAuthenticator_Success(t *testing.T) {
	expiry := time.Now().Add(12 * time.Hour).Truncate(time.Second)
	encoded := base64.StdEncoding.EncodeToString([]byte("AWS:ecr-password"))
	a := newTestECRAuthenticator(&stubECRClient{
		output: &ecr.GetAuthorizationTokenOutput{
			AuthorizationData: []types.AuthorizationData{
				{AuthorizationToken: &encoded, ExpiresAt: &expiry},
			},
		},
	})

	creds, err := a.Authenticate(t.Context(), awsToken(), "123456789.dkr.ecr.us-east-1.amazonaws.com/myrepo", nil)
	require.NoError(t, err)
	assert.Equal(t, "AWS", creds.Username)
	assert.Equal(t, "ecr-password", creds.Password)
	require.NotNil(t, creds.ExpiresAt)
	assert.Equal(t, expiry, *creds.ExpiresAt)
}

func TestECRAuthenticator_Errors(t *testing.T) {
	t.Run("wrong token type", func(t *testing.T) {
		a := newTestECRAuthenticator(&stubECRClient{})
		_, err := a.Authenticate(t.Context(), &Token{Type: TokenTypeBearer, Token: "tok"}, "repo", nil)
		require.ErrorContains(t, err, "requires AWS credentials")
	})

	t.Run("nil AWS credentials", func(t *testing.T) {
		a := newTestECRAuthenticator(&stubECRClient{})
		_, err := a.Authenticate(t.Context(), &Token{Type: TokenTypeAWS}, "repo", nil)
		require.ErrorContains(t, err, "AWS credentials are nil")
	})

	t.Run("no authorization data", func(t *testing.T) {
		a := newTestECRAuthenticator(&stubECRClient{output: &ecr.GetAuthorizationTokenOutput{}})
		_, err := a.Authenticate(t.Context(), awsToken(), "repo", nil)
		require.ErrorContains(t, err, "no ECR authorization data")
	})

	t.Run("malformed token", func(t *testing.T) {
		encoded := base64.StdEncoding.EncodeToString([]byte("no-colon"))
		a := newTestECRAuthenticator(&stubECRClient{
			output: &ecr.GetAuthorizationTokenOutput{
				AuthorizationData: []types.AuthorizationData{{AuthorizationToken: &encoded}},
			},
		})
		_, err := a.Authenticate(t.Context(), awsToken(), "repo", nil)
		require.ErrorContains(t, err, "invalid ECR authorization token format")
	})
}
