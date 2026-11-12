package admin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/util/kube"
	"github.com/argoproj/argo-cd/v3/util/password"
)

func TestInitialPasswordReset(t *testing.T) {
	const namespace = "argocd"
	ctx := context.Background()

	// Create a fake Kubernetes client with a pre-existing argocd-secret
	existingSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"admin.password":      []byte("$2a$10$oldhashedpassword"),
			"admin.passwordMtime": []byte("2024-01-01T00:00:00Z"),
		},
	}
	kubeClientset := fake.NewSimpleClientset(existingSecret)

	// Generate a new random password
	newPassword, err := generateRandomPassword()
	require.NoError(t, err)
	assert.Len(t, newPassword, 16, "generated password should be 16 characters")

	// Hash the password
	hashedPassword, err := password.HashPassword(newPassword)
	require.NoError(t, err)
	assert.NotEmpty(t, hashedPassword)

	// Verify the generated password matches the hash
	valid, _ := password.VerifyPassword(newPassword, hashedPassword)
	assert.True(t, valid, "generated password should verify against its hash")

	// Use kube util to update argocd-secret (same logic as the reset command)
	ku := kube.NewKubeUtil(ctx, kubeClientset)
	err = ku.CreateOrUpdateSecretData(namespace, common.ArgoCDSecretName, map[string][]byte{
		"admin.password":      []byte(hashedPassword),
		"admin.passwordMtime": []byte("2026-11-12T00:00:00Z"),
	}, true)
	require.NoError(t, err)

	// Store plaintext password in argocd-initial-admin-secret
	err = ku.CreateOrUpdateSecretField(namespace, initialPasswordSecretName, "password", newPassword)
	require.NoError(t, err)

	// Verify argocd-secret was updated
	updatedSecret, err := kubeClientset.CoreV1().Secrets(namespace).Get(ctx, common.ArgoCDSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, hashedPassword, string(updatedSecret.Data["admin.password"]))
	assert.Equal(t, "2026-11-12T00:00:00Z", string(updatedSecret.Data["admin.passwordMtime"]))

	// Verify the new password verifies against the stored hash
	storedHash := string(updatedSecret.Data["admin.password"])
	valid, _ = password.VerifyPassword(newPassword, storedHash)
	assert.True(t, valid, "new password should verify against the stored hash in argocd-secret")

	// Verify argocd-initial-admin-secret was created with plaintext password
	initialSecret, err := kubeClientset.CoreV1().Secrets(namespace).Get(ctx, initialPasswordSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, newPassword, string(initialSecret.Data["password"]))
}

func TestInitialPasswordResetUpdatesExistingInitialSecret(t *testing.T) {
	const namespace = "argocd"
	ctx := context.Background()

	// Create fake client with both argocd-secret and pre-existing argocd-initial-admin-secret
	argocdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"admin.password":      []byte("$2a$10$oldhashedpassword"),
			"admin.passwordMtime": []byte("2024-01-01T00:00:00Z"),
		},
	}
	initialAdminSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      initialPasswordSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"password": []byte("old-initial-password"),
		},
	}
	kubeClientset := fake.NewSimpleClientset(argocdSecret, initialAdminSecret)

	// Generate new password and hash
	newPassword, err := generateRandomPassword()
	require.NoError(t, err)
	hashedPassword, err := password.HashPassword(newPassword)
	require.NoError(t, err)

	// Update argocd-secret
	ku := kube.NewKubeUtil(ctx, kubeClientset)
	err = ku.CreateOrUpdateSecretData(namespace, common.ArgoCDSecretName, map[string][]byte{
		"admin.password":      []byte(hashedPassword),
		"admin.passwordMtime": []byte("2026-11-12T12:00:00Z"),
	}, true)
	require.NoError(t, err)

	// Update argocd-initial-admin-secret
	err = ku.CreateOrUpdateSecretField(namespace, initialPasswordSecretName, "password", newPassword)
	require.NoError(t, err)

	// Verify argocd-initial-admin-secret was UPDATED (not duplicated)
	initialSecret, err := kubeClientset.CoreV1().Secrets(namespace).Get(ctx, initialPasswordSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, newPassword, string(initialSecret.Data["password"]))
	assert.NotEqual(t, "old-initial-password", string(initialSecret.Data["password"]))

	// Verify password verifies against the new hash
	valid, _ := password.VerifyPassword(newPassword, hashedPassword)
	assert.True(t, valid)
}

func TestInitialPasswordResetPreservesOtherSecretFields(t *testing.T) {
	const namespace = "argocd"
	ctx := context.Background()

	// Create argocd-secret with extra fields (server.secretkey, tls.crt, tls.key, etc.)
	argocdSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDSecretName,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"admin.password":      []byte("$2a$10$oldhashedpassword"),
			"admin.passwordMtime": []byte("2024-01-01T00:00:00Z"),
			"server.secretkey":    []byte("my-secret-key"),
			"tls.crt":             []byte("my-tls-cert"),
			"tls.key":             []byte("my-tls-key"),
		},
	}
	kubeClientset := fake.NewSimpleClientset(argocdSecret)

	newPassword, err := generateRandomPassword()
	require.NoError(t, err)
	hashedPassword, err := password.HashPassword(newPassword)
	require.NoError(t, err)

	// Merge update (merge=true) so other fields are preserved
	ku := kube.NewKubeUtil(ctx, kubeClientset)
	err = ku.CreateOrUpdateSecretData(namespace, common.ArgoCDSecretName, map[string][]byte{
		"admin.password":      []byte(hashedPassword),
		"admin.passwordMtime": []byte("2026-11-12T12:00:00Z"),
	}, true)
	require.NoError(t, err)

	// Verify other fields are still present
	updatedSecret, err := kubeClientset.CoreV1().Secrets(namespace).Get(ctx, common.ArgoCDSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, "my-secret-key", string(updatedSecret.Data["server.secretkey"]))
	assert.Equal(t, "my-tls-cert", string(updatedSecret.Data["tls.crt"]))
	assert.Equal(t, "my-tls-key", string(updatedSecret.Data["tls.key"]))

	// Verify password was updated
	assert.Equal(t, hashedPassword, string(updatedSecret.Data["admin.password"]))
}

