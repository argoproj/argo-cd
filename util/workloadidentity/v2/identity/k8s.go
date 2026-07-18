package identity

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	typedCoreV1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/argoproj/argo-cd/v3/util/workloadidentity/v2/repository"
)

// K8sProvider passes through the K8s service account JWT directly.
// Use this when the target service can validate K8s JWTs directly via OIDC federation.
//
// If no audience is configured, defaults to "kubernetes.default.svc".
//
// The underlying ServiceAccount object is resolved lazily via LoadSA, so providers
// that don't need it (notably the AWS Pod Identity path, which uses the pod's own
// IAM identity) don't force operators to provision a per-project SA.
type K8sProvider struct {
	saName          string
	namespace       string
	serviceAccounts typedCoreV1.ServiceAccountInterface
	pods            typedCoreV1.PodInterface
	cachedSA        *corev1.ServiceAccount
}

func (p *K8sProvider) DefaultRepositoryAuthenticator() repository.Authenticator {
	return repository.NewHTTPTemplateAuthenticator()
}

// NewK8sProvider creates a new K8s passthrough provider.
func NewK8sProvider(clientset kubernetes.Interface, namespace, saName string) *K8sProvider {
	coreV1 := clientset.CoreV1()
	return &K8sProvider{
		saName:          saName,
		namespace:       namespace,
		serviceAccounts: coreV1.ServiceAccounts(namespace),
		pods:            coreV1.Pods(namespace),
	}
}

// SAName returns the project-scoped service account name this provider is configured
// to use. The SA may not exist in the cluster — see LoadSA for the actual fetch.
func (p *K8sProvider) SAName() string { return p.saName }

// Namespace returns the namespace the project-scoped service account lives in.
func (p *K8sProvider) Namespace() string { return p.namespace }

// LoadSA fetches and caches the project-scoped service account. Callers that need
// annotations on the SA (IRSA role ARN, GCP/Azure client IDs, etc.) call this;
// callers that only need to mint a TokenRequest (or rely on the pod's own identity,
// like AWS Pod Identity) do not.
func (p *K8sProvider) LoadSA(ctx context.Context) (*corev1.ServiceAccount, error) {
	if p.cachedSA != nil {
		return p.cachedSA, nil
	}
	sa, err := p.serviceAccounts.Get(ctx, p.saName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get service account %s: %w", p.saName, err)
	}
	p.cachedSA = sa
	return sa, nil
}

// GetToken requests a K8s token with the configured audience and returns it directly.
// When running inside a pod, the token is bound to the current pod so that
// pod-identity-aware validators (e.g. EKS Pod Identity) can verify the caller.
func (p *K8sProvider) GetToken(ctx context.Context, audience, _ string) (*repository.Token, error) {
	if audience == "" {
		// Default to the Kubernetes API server audience
		audience = "kubernetes.default.svc"
	}

	log.WithFields(log.Fields{
		"serviceAccount": p.saName,
		"audience":       audience,
	}).Info("K8s provider: requesting token for OIDC federation")

	// Request token with 1 hour expiration
	duration := int64(3600)
	tokenRequest := &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{audience},
			ExpirationSeconds: &duration,
		},
	}

	// Bind the token to the current pod if we can determine it.
	// This is required for EKS Pod Identity and similar systems that
	// validate the token is bound to the requesting pod.
	if boundRef, err := p.boundPodRef(ctx); err != nil {
		log.WithError(err).Warn("K8s provider: could not resolve current pod for bound token, requesting unbound token")
	} else if boundRef != nil {
		tokenRequest.Spec.BoundObjectRef = boundRef
	}

	resp, err := p.serviceAccounts.CreateToken(
		ctx,
		p.saName,
		tokenRequest,
		metav1.CreateOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create token for service account %s: %w", p.saName, err)
	}

	token := &repository.Token{
		Type:  repository.TokenTypeBearer,
		Token: resp.Status.Token,
	}
	if !resp.Status.ExpirationTimestamp.IsZero() {
		token.ExpiresAt = &resp.Status.ExpirationTimestamp.Time
	}
	return token, nil
}

// boundPodRef returns a BoundObjectReference for the pod we're running in,
// or (nil, nil) if we can't determine the pod name from the environment.
func (p *K8sProvider) boundPodRef(ctx context.Context) (*authv1.BoundObjectReference, error) {
	podName := os.Getenv("HOSTNAME")
	if podName == "" {
		return nil, nil
	}

	pod, err := p.pods.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s: %w", podName, err)
	}

	return &authv1.BoundObjectReference{
		Kind:       "Pod",
		APIVersion: "v1",
		Name:       pod.Name,
		UID:        pod.UID,
	}, nil
}

// Ensure K8sProvider implements Provider
var _ Provider = (*K8sProvider)(nil)
