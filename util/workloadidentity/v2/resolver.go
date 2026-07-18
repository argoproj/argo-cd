package v2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	jwtgo "github.com/golang-jwt/jwt/v5"
	log "github.com/sirupsen/logrus"

	"k8s.io/client-go/kubernetes"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/jwt"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/identity"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

// serviceAccountTokenPath is the standard in-cluster location of the pod's
// projected service account token. Variable so tests can point it at a fixture.
var serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"

// cachedPodSAName holds the pod service account name after the first
// successful read — the pod's service account cannot change during the
// process lifetime. Failures are not cached, so a transient read error is
// retried on the next call.
var cachedPodSAName atomic.Pointer[string]

// Resolver resolves workload identity credentials from Kubernetes service accounts
type Resolver struct {
	serviceAccounts corev1.ServiceAccountInterface
	credCache       *CredentialCache
}

// NewResolver creates a new workload identity resolver. Resolvers share a
// process-wide credential cache, so resolved repository tokens are reused
// across Resolver instances until they expire.
func NewResolver(clientset kubernetes.Interface, namespace string) *Resolver {
	return &Resolver{
		serviceAccounts: clientset.CoreV1().ServiceAccounts(namespace),
		credCache:       sharedCredentialCache,
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
	saName, err := getServiceAccountName(repository.Project)
	if err != nil {
		return nil, err
	}
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
		return nil, fmt.Errorf("unknown workload identity provider %q, must be one of: k8s, aws, gcp, azure", repository.WorkloadIdentityProvider)
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
// 1. Return still-valid credentials from the cache, if present
// 2. Get K8s service account token via TokenRequest API
// 3. Exchange token using the provided identity provider
// 4. Authenticate to the repository using the provided authenticator
// 5. Cache and return username/password for repo-server to use
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

	cacheKey := credentialCacheKey(repo)
	if creds, ok := r.credCache.Get(cacheKey); ok {
		log.WithFields(log.Fields{
			"project": repo.Project,
			"repoURL": repo.Repo,
		}).Debug("using cached workload identity credentials")
		return creds, nil
	}

	log.WithFields(log.Fields{
		"project": repo.Project,
		"repoURL": repo.Repo,
	}).Info("resolving workload identity credentials")

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

	r.credCache.Set(cacheKey, creds)

	log.WithFields(log.Fields{
		"project": repo.Project,
		"repoURL": repo.Repo,
	}).Info("successfully resolved workload identity credentials")
	return creds, nil
}

// getServiceAccountName returns the service account name for a given project.
// If projectName is empty, the pod's own service account is used.
func getServiceAccountName(projectName string) (string, error) {
	if projectName != "" {
		return "argocd-project-" + projectName, nil
	}
	return podServiceAccountName()
}

// podServiceAccountName returns the name of the service account the pod runs
// as, taken from the subject claim of the mounted service account token
// (system:serviceaccount:<namespace>:<name>). The signature is not verified:
// the token was issued to this pod by the kubelet and the subject is only
// used to name the SA for subsequent TokenRequest calls.
func podServiceAccountName() (string, error) {
	if name := cachedPodSAName.Load(); name != nil {
		return *name, nil
	}

	data, err := os.ReadFile(serviceAccountTokenPath)
	if err != nil {
		return "", fmt.Errorf("no project specified and the pod service account token could not be read: %w", err)
	}
	claims := jwtgo.MapClaims{}
	if _, _, err := jwtgo.NewParser().ParseUnverified(strings.TrimSpace(string(data)), claims); err != nil {
		return "", fmt.Errorf("failed to parse pod service account token: %w", err)
	}
	sub := jwt.StringField(claims, "sub")

	const prefix = "system:serviceaccount:"
	name, ok := strings.CutPrefix(sub, prefix)
	if ok {
		_, name, ok = strings.Cut(name, ":")
	}
	if !ok || name == "" {
		return "", fmt.Errorf("unexpected subject %q in pod service account token, want %s<namespace>:<name>", sub, prefix)
	}

	cachedPodSAName.Store(&name)
	return name, nil
}
