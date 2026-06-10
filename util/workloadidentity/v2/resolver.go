package v2

import (
	"context"
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/identity"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

// Standard cloud provider annotation fields (on service accounts)
const (
	AnnotationAWSRoleARN = "eks.amazonaws.com/role-arn"
)

// Resolver resolves workload identity credentials from Kubernetes service accounts
type Resolver struct {
	serviceAccounts corev1.ServiceAccountInterface
}

// NewResolver creates a new workload identity resolver
func NewResolver(clientset kubernetes.Interface, namespace string) *Resolver {
	return &Resolver{
		serviceAccounts: clientset.CoreV1().ServiceAccounts(namespace),
	}
}

// NewIdentityProvider creates an identity provider based on the provider name.
// This is a convenience function for callers who don't want to manage provider instantiation.
//
// The project-scoped service account is resolved lazily by the returned provider — paths
// that don't need an SA (notably AWS EKS Pod Identity, which uses the pod's own IAM
// identity and injects a session tag from the repository's project field) do not require
// `argocd-project-<project>` to exist in the cluster.
func NewIdentityProvider(repository *v1alpha1.Repository, clientset kubernetes.Interface, ns string) (identity.Provider, error) {
	saName := getServiceAccountName(repository.Project)
	k8sProvider := identity.NewK8sProvider(clientset, ns, saName)
	switch repository.WorkloadIdentityProvider {
	case "k8s":
		return k8sProvider, nil
	case "aws":
		return identity.NewAWSProvider(repository, k8sProvider), nil
	case "gcp":
		return identity.NewGCPProvider(k8sProvider), nil
	case "azure":
		return identity.NewAzureProvider(repository, k8sProvider), nil
	default:
		return nil, nil
	}
}

// NewAuthenticator creates a repository authenticator based on the authenticator name.
// This is a convenience function for callers who don't want to manage authenticator instantiation.
func NewAuthenticator(authenticator string) repository.Authenticator {
	switch authenticator {
	case "ecr":
		return repository.NewECRAuthenticator()
	case "passthrough":
		return repository.NewPassthroughAuthenticator()
	case "acr":
		return repository.NewACRAuthenticator()
	case "http":
		return repository.NewHTTPTemplateAuthenticator()
	default:
		return nil
	}
}

// ResolveCredentials resolves workload identity credentials for a repository.
//
// Parameters:
//   - idProvider: The identity provider to use for token exchange (use NewIdentityProvider to create one)
//   - repoAuth: The repository authenticator to use (use NewAuthenticator to create one)
//   - repo: The repository containing workload identity configuration
//
// The process is:
// 1. Get K8s service account token via TokenRequest API
// 2. Exchange token using the provided identity provider
// 3. Authenticate to the repository using the provided authenticator
// 4. Return username/password for repo-server to use
func (r *Resolver) ResolveCredentials(ctx context.Context, idProvider identity.Provider, repoAuth repository.Authenticator, repo *v1alpha1.Repository) (*repository.Credentials, error) {
	if idProvider == nil {
		return nil, errors.New("identity provider is required")
	}
	if repoAuth == nil {
		return nil, errors.New("repository authenticator is required")
	}
	if repo == nil {
		return nil, errors.New("repository is required")
	}

	log.WithFields(log.Fields{
		"project": repo.Project,
		"repoURL": repo.Repo,
	}).Info("resolving workload identity credentials")

	// Determine service account name from project
	saName := getServiceAccountName(repo.Project)
	log.WithField("serviceAccount", saName).Debug("using service account for workload identity")

	log.Info("exchanging credentials with identity provider")
	idToken, err := idProvider.GetToken(ctx, repo.WorkloadIdentityAudience, repo.WorkloadIdentityTokenURL)
	if err != nil {
		return nil, fmt.Errorf("identity provider failed: %w", err)
	}
	log.WithField("tokenType", idToken.Type).Info("obtained identity token")

	log.WithField("repoURL", repo.Repo).Info("authenticating to repository")
	creds, err := repoAuth.Authenticate(ctx, idToken, repo.Repo, &repository.Config{
		Username:              repo.WorkloadIdentityUsername,
		Insecure:              repo.Insecure,
		AuthHost:              repo.WorkloadIdentityAuthHost,
		Method:                repo.WorkloadIdentityMethod,
		PathTemplate:          repo.WorkloadIdentityPathTemplate,
		BodyTemplate:          repo.WorkloadIdentityBodyTemplate,
		AuthType:              repo.WorkloadIdentityAuthType,
		Params:                repo.WorkloadIdentityParams,
		ResponseTokenField:    repo.WorkloadIdentityResponseTokenField,
		ResponseUsernameField: repo.WorkloadIdentityResponseUsernameField,
	})
	if err != nil {
		return nil, err
	}

	log.WithFields(log.Fields{
		"project": repo.Project,
		"repoURL": repo.Repo,
	}).Info("successfully resolved workload identity credentials")
	return creds, nil
}

// getServiceAccountName returns the service account name for a given project
// If projectName is empty, it returns the global service account name
func getServiceAccountName(projectName string) string {
	if projectName == "" {
		return "argocd-global"
	}
	return "argocd-project-" + projectName
}
