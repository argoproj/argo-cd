package commands

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"

	"github.com/argoproj/argo-cd/v3/util/errors"
)

const (
	clusterIDHeader = "x-k8s-aws-id"
	// The sts GetCallerIdentity request is valid for 15 minutes regardless of this parameters value after it has been
	// signed, but we set this unused parameter to 60 for legacy reasons (we check for a value between 0 and 60 on the
	// server side in 0.3.0 or earlier).  IT IS IGNORED.  If we can get STS to support x-amz-expires, then we should
	// set this parameter to the actual expiration, and make it configurable.
	requestPresignParam = 60
	// The actual token expiration (presigned STS urls are valid for 15 minutes after timestamp in x-amz-date).
	presignedURLExpiration = 15 * time.Minute
	v1Prefix               = "k8s-aws-v1."
)

// newAWSCommand returns a new instance of an aws command that generates k8s auth token
// implementation is "inspired" by https://github.com/kubernetes-sigs/aws-iam-authenticator/blob/e61f537662b64092ed83cb76e600e023f627f628/pkg/token/token.go#L316
func newAWSCommand() *cobra.Command {
	var (
		clusterName string
		roleARN     string
		profile     string
	)
	command := &cobra.Command{
		Use: "aws",
		Run: func(c *cobra.Command, _ []string) {
			ctx := c.Context()

			presignedURLString, err := getSignedRequestWithRetry(ctx, time.Minute, 5*time.Second, clusterName, roleARN, profile, getSignedRequest)
			errors.CheckError(err)
			token := v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLString))
			// Set token expiration to 1 minute before the presigned URL expires for some cushion
			tokenExpiration := time.Now().Local().Add(presignedURLExpiration - 1*time.Minute)
			_, _ = fmt.Fprint(os.Stdout, formatJSON(token, tokenExpiration))
		},
	}
	command.Flags().StringVar(&clusterName, "cluster-name", "", "AWS Cluster name")
	command.Flags().StringVar(&roleARN, "role-arn", "", "AWS Role ARN")
	command.Flags().StringVar(&profile, "profile", "", "AWS Profile")
	return command
}

type getSignedRequestFunc func(clusterName, roleARN string, profile string) (string, error)

func getSignedRequestWithRetry(ctx context.Context, timeout, interval time.Duration, clusterName, roleARN string, profile string, fn getSignedRequestFunc) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		signed, err := fn(clusterName, roleARN, profile)
		if err == nil {
			return signed, nil
		}
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout while trying to get signed aws request: last error: %w", err)
		case <-time.After(interval):
		}
	}
}

func getSignedRequest(clusterName, roleARN string, profile string) (string, error) {
	ctx := context.Background()

	// Build config options
	var configOpts []func(*config.LoadOptions) error
	if profile != "" {
		configOpts = append(configOpts, config.WithSharedConfigProfile(profile))
	}

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return "", fmt.Errorf("error loading AWS config: %w", err)
	}

	// Assume role if provided
	if roleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		creds := stscreds.NewAssumeRoleProvider(stsClient, roleARN)
		cfg.Credentials = aws.NewCredentialsCache(creds)
	}

	// Create STS client with custom header middleware
	stsClient := sts.NewFromConfig(cfg, func(o *sts.Options) {
		o.APIOptions = append(o.APIOptions, func(stack *middleware.Stack) error {
			return stack.Build.Add(middleware.BuildMiddlewareFunc("AddClusterIDHeader", func(
				ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler,
			) (middleware.BuildOutput, middleware.Metadata, error) {
				req, ok := in.Request.(*smithyhttp.Request)
				if ok {
					req.Header.Add(clusterIDHeader, clusterName)
				}
				return next.HandleBuild(ctx, in)
			}), middleware.After)
		})
	})

	// Create presign client and presign the request
	presignClient := sts.NewPresignClient(stsClient)
	presigned, err := presignClient.PresignGetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("error presigning AWS request: %w", err)
	}

	return presigned.URL, nil
}

func formatJSON(token string, expiration time.Time) string {
	expirationTimestamp := metav1.NewTime(expiration)
	execInput := &clientauthv1beta1.ExecCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "client.authentication.k8s.io/v1beta1",
			Kind:       "ExecCredential",
		},
		Status: &clientauthv1beta1.ExecCredentialStatus{
			ExpirationTimestamp: &expirationTimestamp,
			Token:               token,
		},
	}
	enc, _ := json.Marshal(execInput)
	return string(enc)
}
