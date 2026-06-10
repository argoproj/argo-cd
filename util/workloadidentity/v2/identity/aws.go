package identity

// This file implements two AWS authentication paths for ECR access:
// EKS Pod Identity (preferred, documented first) and IRSA (fallback, documented below).
//
// # AWS EKS Pod Identity Setup (preferred path)
//
// One controller SA, one IAM role, one Pod Identity Association. Per-project scoping
// is done via the `argocd-project` session tag injected on a chained AssumeRole call,
// referenced from resource policies via `aws:PrincipalTag/argocd-project`.
//
// ## Required AWS/EKS Setup
//
// 1. Install the EKS Pod Identity agent addon on the cluster:
//
//	aws eks create-addon --cluster-name "$CLUSTER_NAME" --addon-name eks-pod-identity-agent
//
// 2. Create an IAM policy for ECR access (per-project scoping via PrincipalTag goes
//    on the resource policies, NOT here — this policy just grants the actions):
//
//	cat <<EOF > argocd-ecr-policy.json
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [
//	        {
//	            "Effect": "Allow",
//	            "Action": [
//	                "ecr:GetAuthorizationToken",
//	                "ecr:BatchCheckLayerAvailability",
//	                "ecr:GetDownloadUrlForLayer",
//	                "ecr:BatchGetImage"
//	            ],
//	            "Resource": "*"
//	        }
//	    ]
//	}
//	EOF
//	aws iam create-policy --policy-name ArgoCD-ECR-ReadOnly \
//	    --policy-document file://argocd-ecr-policy.json
//
// 3. Create the controller IAM role with a trust policy that allows BOTH the Pod
//    Identity service principal AND self-assume + TagSession (required for the
//    chained AssumeRole that injects the argocd-project session tag):
//
//	export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
//	export ROLE_NAME="argocd-controller"
//
//	cat <<EOF > trust-policy.json
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [
//	        {
//	            "Sid": "AllowPodIdentityToAssume",
//	            "Effect": "Allow",
//	            "Principal": { "Service": "pods.eks.amazonaws.com" },
//	            "Action": ["sts:AssumeRole", "sts:TagSession"]
//	        },
//	        {
//	            "Sid": "AllowSelfAssumeForProjectTagInjection",
//	            "Effect": "Allow",
//	            "Principal": {
//	                "AWS": "arn:aws:iam::${AWS_ACCOUNT_ID}:role/${ROLE_NAME}"
//	            },
//	            "Action": ["sts:AssumeRole", "sts:TagSession"]
//	        }
//	    ]
//	}
//	EOF
//
//	aws iam create-role --role-name "$ROLE_NAME" \
//	    --assume-role-policy-document file://trust-policy.json
//	aws iam attach-role-policy --role-name "$ROLE_NAME" \
//	    --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/ArgoCD-ECR-ReadOnly"
//
//    Both `sts:TagSession` lines are non-negotiable — without them, the corresponding
//    step fails with AccessDenied on the tags specifically (reads like a generic auth
//    error, easy to misdiagnose). If self-referencing the role ARN feels awkward, the
//    second statement's Principal can be `"AWS": "arn:aws:iam::${AWS_ACCOUNT_ID}:root"`
//    — same effect, leans on the identity policy for scoping.
//
// 4. Bind the controller ServiceAccount to the role via a Pod Identity Association:
//
//	aws eks create-pod-identity-association \
//	    --cluster-name "$CLUSTER_NAME" \
//	    --namespace argocd \
//	    --service-account argocd-application-controller \
//	    --role-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:role/${ROLE_NAME}"
//
// 5. Reference the `aws:PrincipalTag/argocd-project` session tag in the resource
//    policies you actually want scoped per project (ECR repo policy, S3 bucket policy,
//    KMS key policy, etc.). Example ECR repository policy:
//
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [
//	        {
//	            "Effect": "Allow",
//	            "Principal": { "AWS": "arn:aws:iam::ACCOUNT:role/argocd-controller" },
//	            "Action": ["ecr:BatchGetImage", "ecr:GetDownloadUrlForLayer"],
//	            "Condition": {
//	                "StringEquals": { "aws:PrincipalTag/argocd-project": "team-platform" }
//	            }
//	        }
//	    ]
//	}
//
// ## What's NOT needed (vs IRSA)
//
//   - No OIDC provider on the cluster
//   - No `sts:AssumeRoleWithWebIdentity` permission anywhere
//   - No `eks.amazonaws.com/role-arn` annotation on the SA
//   - No per-project service accounts (one controller SA covers all projects)
//   - No `sts:GetCallerIdentity` permission in the identity policy (implicit for all callers)
//
// ## Authentication Flow (Pod Identity)
//
// 1. Pod Identity webhook injects AWS_CONTAINER_CREDENTIALS_FULL_URI + token file path
// 2. SDK's default credential chain resolves Pod Identity creds via the container creds provider
// 3. If the repository has a project: STS GetCallerIdentity → derive role ARN → AssumeRole
//    with `argocd-project=<project>` as a session tag
// 4. If the repository has no project: return base Pod Identity creds untagged
// 5. Use the resulting credentials to call ECR GetAuthorizationToken
//
// ────────────────────────────────────────────────────────────────────────────────
//
// # AWS IRSA (IAM Roles for Service Accounts) Setup (fallback path)
//
// This is the legacy path, used when AWS_CONTAINER_CREDENTIALS_FULL_URI is not set.
//
// ## Required AWS/EKS Setup
//
// 1. Ensure your EKS cluster has an OIDC provider configured:
//
//	export CLUSTER_NAME="<your-cluster-name>"
//	export AWS_REGION="<your-region>"
//
//	# Check if OIDC provider exists
//	aws eks describe-cluster --name $CLUSTER_NAME --query "cluster.identity.oidc.issuer" --output text
//
//	# If not set up, create the OIDC provider
//	eksctl utils associate-iam-oidc-provider --cluster $CLUSTER_NAME --approve
//
// 2. Get the OIDC provider URL and account ID:
//
//	export OIDC_PROVIDER=$(aws eks describe-cluster --name $CLUSTER_NAME \
//	    --query "cluster.identity.oidc.issuer" --output text | sed -e "s/^https:\/\///")
//	export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
//
// 3. Create an IAM policy for ECR access:
//
//	cat <<EOF > ecr-policy.json
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [
//	        {
//	            "Effect": "Allow",
//	            "Action": [
//	                "ecr:GetAuthorizationToken",
//	                "ecr:BatchCheckLayerAvailability",
//	                "ecr:GetDownloadUrlForLayer",
//	                "ecr:BatchGetImage"
//	            ],
//	            "Resource": "*"
//	        }
//	    ]
//	}
//	EOF
//	aws iam create-policy --policy-name ArgoCD-ECR-ReadOnly --policy-document file://ecr-policy.json
//
// 4. Create an IAM role with trust policy for the ArgoCD service account:
//
//	export ARGOCD_NAMESPACE="argocd"
//	export PROJECT_NAME="default"
//	export ROLE_NAME="argocd-project-${PROJECT_NAME}"
//
//	cat <<EOF > trust-policy.json
//	{
//	    "Version": "2012-10-17",
//	    "Statement": [
//	        {
//	            "Effect": "Allow",
//	            "Principal": {
//	                "Federated": "arn:aws:iam::${AWS_ACCOUNT_ID}:oidc-provider/${OIDC_PROVIDER}"
//	            },
//	            "Action": "sts:AssumeRoleWithWebIdentity",
//	            "Condition": {
//	                "StringEquals": {
//	                    "${OIDC_PROVIDER}:sub": "system:serviceaccount:${ARGOCD_NAMESPACE}:argocd-project-${PROJECT_NAME}",
//	                    "${OIDC_PROVIDER}:aud": "sts.amazonaws.com"
//	                }
//	            }
//	        }
//	    ]
//	}
//	EOF
//
//	aws iam create-role --role-name $ROLE_NAME --assume-role-policy-document file://trust-policy.json
//	aws iam attach-role-policy --role-name $ROLE_NAME \
//	    --policy-arn arn:aws:iam::${AWS_ACCOUNT_ID}:policy/ArgoCD-ECR-ReadOnly
//
// ## Required Kubernetes ServiceAccount Annotations
//
// The Kubernetes ServiceAccount (argocd-project-<name>) needs this annotation:
//
//   - eks.amazonaws.com/role-arn: The full ARN of the IAM role to assume
//     Example: arn:aws:iam::123456789012:role/argocd-project-default
//
// ## Required Repository Secret Fields
//
//   - useWorkloadIdentity: "true"
//   - workloadIdentityProvider: "aws"
//   - project: "<argocd-project-name>" (maps to argocd-project-<name> ServiceAccount)
//
// ## Optional Configuration
//
//   - workloadIdentityTokenURL: Override STS endpoint (for GovCloud, China regions, etc.)
//     Example for GovCloud: "https://sts.us-gov-west-1.amazonaws.com"
//
// ## Authentication Flow
//
// 1. Request a K8s token for the project ServiceAccount via TokenRequest API
//    (with audience "sts.amazonaws.com")
// 2. Call AWS STS AssumeRoleWithWebIdentity with the K8s token
// 3. Use the temporary credentials to call ECR GetAuthorizationToken
// 4. Return the ECR credentials for use with the registry
//
// ## Region Detection (applies to both paths)
//
// The AWS region is automatically extracted from the ECR repository URL.
// Example: 123456789012.dkr.ecr.us-west-2.amazonaws.com → us-west-2

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	ststypes "github.com/aws/aws-sdk-go-v2/service/sts/types"
	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

const (
	// AnnotationAWSRoleARN is the EKS annotation for IAM role
	AnnotationAWSRoleARN = "eks.amazonaws.com/role-arn"

	// DefaultAWSAudience is the default STS audience for IRSA
	DefaultAWSAudience = "sts.amazonaws.com"

	// PodIdentityAudience is the audience for EKS Pod Identity tokens
	PodIdentityAudience = "pods.eks.amazonaws.com"

	// EnvPodIdentityAgentURI is set by the Pod Identity webhook when the agent is available.
	// Presence of this env var is our signal that the SDK's default credential chain will
	// resolve to Pod Identity creds (via the container credentials provider).
	EnvPodIdentityAgentURI = "AWS_CONTAINER_CREDENTIALS_FULL_URI"

	// SessionTagProject is the AWS STS session tag key used to scope per-project access
	// when chaining AssumeRole on top of EKS Pod Identity credentials. IAM policies on
	// downstream resources can reference this via aws:PrincipalTag/argocd-project.
	SessionTagProject = "argocd-project"

	// chainedSessionDuration is the AWS hard cap for chained STS AssumeRole sessions (1h).
	chainedSessionDuration = int32(3600)
)

// AWSProvider exchanges K8s JWTs for AWS credentials via STS
type AWSProvider struct {
	repo *v1alpha1.Repository
	k8s  *K8sProvider
}

func (p *AWSProvider) DefaultRepositoryAuthenticator() repository.Authenticator {
	return repository.NewECRAuthenticator()
}

// NewAWSProvider creates a new AWS identity provider
func NewAWSProvider(repo *v1alpha1.Repository, k8s *K8sProvider) *AWSProvider {
	return &AWSProvider{
		repo: repo,
		k8s:  k8s,
	}
}

// GetToken exchanges a K8s JWT for AWS credentials.
// If Pod Identity is available (webhook-injected env var present), the SDK's default
// credential chain resolves to Pod Identity creds, and we chain AssumeRole on top to
// inject the argocd-project session tag. Otherwise, we fall back to IRSA.
func (p *AWSProvider) GetToken(ctx context.Context, audience string, tokenURL string) (*repository.Token, error) {
	// ECR region is derived from the repository URL — this is the region the ECR
	// authenticator needs for GetAuthorizationToken, independent of where STS runs.
	ecrRegion := extractAWSRegion(p.repo.Repo)

	if os.Getenv(EnvPodIdentityAgentURI) != "" {
		token, err := p.getTokenViaPodIdentity(ctx, ecrRegion)
		if err == nil {
			return token, nil
		}

		log.WithFields(log.Fields{
			"serviceAccount": p.k8s.SAName(),
			"error":          err.Error(),
		}).Warn("AWS Pod Identity: failed, falling back to IRSA")
	}

	return p.getTokenViaIRSA(ctx, audience, tokenURL, ecrRegion)
}

// getTokenViaPodIdentity uses the SDK's default credential chain (which transparently
// calls the Pod Identity agent via the webhook-injected env vars) to obtain base creds.
// If the repository has a project set, AssumeRole is chained on the same role to inject
// the argocd-project session tag; otherwise the credential is treated as global and the
// base Pod Identity creds are returned directly. The chained path requires the role's
// trust policy to allow both sts:AssumeRole AND sts:TagSession from its own assumed-role
// principal (or the account root).
func (p *AWSProvider) getTokenViaPodIdentity(ctx context.Context, region string) (*repository.Token, error) {
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	project := p.repo.Project
	if project == "" {
		log.Info("AWS Pod Identity: no project set; returning base Pod Identity credentials")
		creds, err := awsCfg.Credentials.Retrieve(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve Pod Identity credentials: %w", err)
		}
		token := &repository.Token{
			Type: repository.TokenTypeAWS,
			AWSCredentials: &repository.AWSCredentials{
				AccessKeyID:     creds.AccessKeyID,
				SecretAccessKey: creds.SecretAccessKey,
				SessionToken:    creds.SessionToken,
				Region:          region,
			},
		}
		if creds.CanExpire {
			token.AWSCredentials.Expiration = &creds.Expires
		}
		return token, nil
	}

	stsClient := sts.NewFromConfig(awsCfg)

	// Discover the assumed-role principal so we can derive the underlying IAM role ARN
	// for the chained AssumeRole call. The Pod Identity webhook doesn't expose the role
	// ARN as an env var, so a single GetCallerIdentity round-trip is the price of dropping
	// the explicit AssumeRoleForPodIdentity call.
	caller, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to identify Pod Identity caller: %w", err)
	}
	roleARN, err := roleARNFromAssumedRoleARN(aws.ToString(caller.Arn))
	if err != nil {
		return nil, fmt.Errorf("failed to derive role ARN from caller identity: %w", err)
	}

	roleSessionName := "argocd-proj-" + project
	duration := chainedSessionDuration

	log.WithFields(log.Fields{
		"roleARN":         roleARN,
		"project":         project,
		"roleSessionName": roleSessionName,
	}).Info("AWS Pod Identity: chaining AssumeRole with argocd-project session tag")

	// Chained AssumeRole drops the original session's tags — EKS Pod Identity injects
	// kubernetes-namespace, kubernetes-service-account, kubernetes-pod-{name,uid},
	// eks-cluster-{name,arn} on the base session, and none of those survive this hop.
	// They can be reconstructed if IAM policies need them:
	//   - kubernetes-namespace / kubernetes-service-account: free from p.k8s.sa
	//   - kubernetes-pod-name / kubernetes-pod-uid: needs downward API env vars
	//   - eks-cluster-name / eks-cluster-arn: needs env var or IMDS detection
	// Also consider TransitiveTagKeys for argocd-project if downstream consumers do
	// their own AssumeRole and need the tag to persist across further chains.
	out, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleARN),
		RoleSessionName: aws.String(roleSessionName),
		DurationSeconds: &duration,
		Tags: []ststypes.Tag{
			{Key: aws.String(SessionTagProject), Value: aws.String(project)},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("AssumeRole with %s session tag failed for role %s: %w", SessionTagProject, roleARN, err)
	}

	log.WithFields(log.Fields{
		"roleARN":    roleARN,
		"project":    project,
		"expiration": out.Credentials.Expiration,
	}).Info("AWS Pod Identity: obtained project-tagged credentials")

	return &repository.Token{
		Type: repository.TokenTypeAWS,
		AWSCredentials: &repository.AWSCredentials{
			AccessKeyID:     aws.ToString(out.Credentials.AccessKeyId),
			SecretAccessKey: aws.ToString(out.Credentials.SecretAccessKey),
			SessionToken:    aws.ToString(out.Credentials.SessionToken),
			Expiration:      out.Credentials.Expiration,
			Region:          region,
		},
	}, nil
}

// roleARNFromAssumedRoleARN converts an assumed-role ARN (returned by STS/EKS auth) into
// the underlying IAM role ARN. Accepts either form for robustness:
//
//	arn:aws:sts::123456789012:assumed-role/MyRole/MySession -> arn:aws:iam::123456789012:role/MyRole
//	arn:aws:iam::123456789012:role/MyRole                   -> unchanged
func roleARNFromAssumedRoleARN(input string) (string, error) {
	parsed, err := arn.Parse(input)
	if err != nil {
		return "", fmt.Errorf("invalid ARN %q: %w", input, err)
	}
	switch parsed.Service {
	case "iam":
		if strings.HasPrefix(parsed.Resource, "role/") {
			return input, nil
		}
		return "", fmt.Errorf("expected role resource in IAM ARN, got %q", parsed.Resource)
	case "sts":
		// sts resource format: "assumed-role/ROLE_NAME/SESSION_NAME"
		parts := strings.SplitN(parsed.Resource, "/", 3)
		if len(parts) < 2 || parts[0] != "assumed-role" {
			return "", fmt.Errorf("expected assumed-role resource in STS ARN, got %q", parsed.Resource)
		}
		return arn.ARN{
			Partition: parsed.Partition,
			Service:   "iam",
			AccountID: parsed.AccountID,
			Resource:  "role/" + parts[1],
		}.String(), nil
	default:
		return "", fmt.Errorf("unexpected service %q in ARN: %s", parsed.Service, input)
	}
}

// getTokenViaIRSA exchanges a K8s token for AWS credentials via STS AssumeRoleWithWebIdentity.
// The SDK resolves its own region for STS (from AWS_REGION/AWS_DEFAULT_REGION injected by the
// EKS webhook). ecrRegion is only used in the returned credentials for ECR GetAuthorizationToken.
func (p *AWSProvider) getTokenViaIRSA(ctx context.Context, audience string, tokenURL string, ecrRegion string) (*repository.Token, error) {
	sa, err := p.k8s.LoadSA(ctx)
	if err != nil {
		return nil, err
	}
	saName := sa.Name
	roleARN := sa.Annotations[AnnotationAWSRoleARN]
	if roleARN == "" {
		return nil, fmt.Errorf("service account %s missing %s annotation", saName, AnnotationAWSRoleARN)
	}

	if audience == "" {
		audience = DefaultAWSAudience
	}

	k8sToken, err := p.k8s.GetToken(ctx, audience, "")
	if err != nil {
		return nil, fmt.Errorf("failed to request K8s token: %w", err)
	}

	// Let the SDK resolve region from env (AWS_REGION/AWS_DEFAULT_REGION injected by EKS webhook)
	awsCfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	log.WithFields(log.Fields{
		"serviceAccount": saName,
		"roleARN":        roleARN,
		"stsRegion":      awsCfg.Region,
		"ecrRegion":      ecrRegion,
	}).Info("AWS IRSA: assuming IAM role with web identity")

	stsEndpoint := tokenURL
	if stsEndpoint != "" {
		log.WithField("stsEndpoint", stsEndpoint).Debug("AWS IRSA: using custom STS endpoint")
	}

	var stsOpts []func(*sts.Options)
	if stsEndpoint != "" {
		stsOpts = append(stsOpts, func(o *sts.Options) {
			o.BaseEndpoint = aws.String(stsEndpoint)
		})
	}
	stsClient := sts.NewFromConfig(awsCfg, stsOpts...)

	roleSessionName := "argocd-" + saName
	durationSeconds := int32(3600)
	log.WithFields(log.Fields{
		"roleSessionName": roleSessionName,
	}).Debug("AWS IRSA: calling STS AssumeRoleWithWebIdentity")

	assumeResult, err := stsClient.AssumeRoleWithWebIdentity(ctx, &sts.AssumeRoleWithWebIdentityInput{
		RoleArn:          aws.String(roleARN),
		WebIdentityToken: aws.String(k8sToken.Token),
		RoleSessionName:  aws.String(roleSessionName),
		DurationSeconds:  &durationSeconds,
	})
	if err != nil {
		log.WithFields(log.Fields{
			"roleARN": roleARN,
			"error":   err.Error(),
		}).Error("AWS IRSA: failed to assume role")
		return nil, fmt.Errorf("failed to assume role %s: %w", roleARN, err)
	}

	log.WithFields(log.Fields{
		"roleARN":    roleARN,
		"expiration": assumeResult.Credentials.Expiration,
	}).Info("AWS IRSA: successfully assumed IAM role")

	return &repository.Token{
		Type: repository.TokenTypeAWS,
		AWSCredentials: &repository.AWSCredentials{
			AccessKeyID:     *assumeResult.Credentials.AccessKeyId,
			SecretAccessKey: *assumeResult.Credentials.SecretAccessKey,
			SessionToken:    *assumeResult.Credentials.SessionToken,
			Expiration:      assumeResult.Credentials.Expiration,
			Region:          ecrRegion,
		},
	}, nil
}

// extractAWSRegion extracts the AWS region from an ECR repository URL
// Example: 123456789012.dkr.ecr.us-west-2.amazonaws.com → us-west-2
func extractAWSRegion(repoURL string) string {
	// Remove oci:// prefix if present
	repoURL = strings.TrimPrefix(repoURL, "oci://")

	// Split by dots: ["123456789012", "dkr", "ecr", "us-west-2", "amazonaws", "com"]
	parts := strings.Split(repoURL, ".")
	if len(parts) >= 4 && parts[1] == "dkr" && parts[2] == "ecr" {
		return parts[3]
	}

	// Default to us-east-1 if we can't parse the region
	return "us-east-1"
}

// Ensure AWSProvider implements Provider
var _ Provider = (*AWSProvider)(nil)
