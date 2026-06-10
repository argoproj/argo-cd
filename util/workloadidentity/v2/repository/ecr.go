package repository

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// ECRAuthenticator exchanges AWS credentials for ECR authorization tokens
// This is a stub - full implementation would use AWS ECR SDK
type ECRAuthenticator struct{}

// NewECRAuthenticator creates a new ECR authenticator
func NewECRAuthenticator() *ECRAuthenticator {
	return &ECRAuthenticator{}
}

// Authenticate exchanges AWS credentials for ECR credentials
func (a *ECRAuthenticator) Authenticate(ctx context.Context, token *Token, repoURL string, cfg *Config) (*Credentials, error) {
	if token.Type != TokenTypeAWS {
		return nil, fmt.Errorf("ecr authenticator requires AWS credentials, got %s", token.Type)
	}

	if token.AWSCredentials == nil {
		return nil, errors.New("AWS credentials are nil")
	}

	log.WithField("region", token.AWSCredentials.Region).Info("ECR: requesting authorization token")

	// Create ECR client with temporary credentials from STS
	ecrCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(token.AWSCredentials.Region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			token.AWSCredentials.AccessKeyID,
			token.AWSCredentials.SecretAccessKey,
			token.AWSCredentials.SessionToken,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create ECR config: %w", err)
	}
	ecrClient := ecr.NewFromConfig(ecrCfg)

	// Get ECR authorization token
	log.Debug("ECR: calling GetAuthorizationToken API")
	authResult, err := ecrClient.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		log.WithError(err).Error("ECR: failed to get authorization token")
		return nil, fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(authResult.AuthorizationData) == 0 {
		return nil, errors.New("no ECR authorization data returned")
	}

	// Decode the base64-encoded authorization token
	decoded, err := base64.StdEncoding.DecodeString(*authResult.AuthorizationData[0].AuthorizationToken)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ECR authorization token: %w", err)
	}

	// ECR token format is "username:password"
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, errors.New("invalid ECR authorization token format")
	}

	log.WithField("region", token.AWSCredentials.Region).Info("ECR: successfully obtained authorization token")

	return &Credentials{
		Username: parts[0],
		Password: parts[1],
	}, nil
}

// Ensure ECRAuthenticator implements Authenticator
var _ Authenticator = (*ECRAuthenticator)(nil)
