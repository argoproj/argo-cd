package commands

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials/stscreds"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientauthv1beta1 "k8s.io/client-go/pkg/apis/clientauthentication/v1beta1"

	"github.com/argoproj/argo-cd/v2/util/errors"
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
	)
	var command = &cobra.Command{
		Use: "aws",
		Run: func(c *cobra.Command, args []string) {
			sess, err := session.NewSession()
			errors.CheckError(err)
			stsAPI := sts.New(sess)
			if roleARN != "" {
				creds := stscreds.NewCredentials(sess, roleARN)
				stsAPI = sts.New(sess, &aws.Config{Credentials: creds})
			}
			request, _ := stsAPI.GetCallerIdentityRequest(&sts.GetCallerIdentityInput{})
			request.HTTPRequest.Header.Add(clusterIDHeader, clusterName)
			presignedURLString, err := request.Presign(requestPresignParam)
			errors.CheckError(err)
			token := v1Prefix + base64.RawURLEncoding.EncodeToString([]byte(presignedURLString))
			// Set token expiration to 1 minute before the presigned URL expires for some cushion
			tokenExpiration := time.Now().Local().Add(presignedURLExpiration - 1*time.Minute)
			_, _ = fmt.Fprint(os.Stdout, formatJSON(token, tokenExpiration))
		},
	}
	command.Flags().StringVar(&clusterName, "cluster-name", "", "AWS Cluster name")
	command.Flags().StringVar(&roleARN, "role-arn", "", "AWS Role ARN")
	return command
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
