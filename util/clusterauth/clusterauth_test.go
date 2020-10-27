package clusterauth

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/util/errors"
)

const (
	testToken = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJrdWJlLXN5c3RlbSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VjcmV0Lm5hbWUiOiJhcmdvY2QtbWFuYWdlci10b2tlbi10ajc5ciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJhcmdvY2QtbWFuYWdlciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjkxZGQzN2NmLThkOTItMTFlOS1hMDkxLWQ2NWYyYWU3ZmE4ZCIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDprdWJlLXN5c3RlbTphcmdvY2QtbWFuYWdlciJ9.ytZjt2pDV8-A7DBMR06zQ3wt9cuVEfq262TQw7sdra-KRpDpMPnziMhc8bkwvgW-LGhTWUh5iu1y-1QhEx6mtbCt7vQArlBRxfvM5ys6ClFkplzq5c2TtZ7EzGSD0Up7tdxuG9dvR6TGXYdfFcG779yCdZo2H48sz5OSJfdEriduMEY1iL5suZd3ebOoVi1fGflmqFEkZX6SvxkoArl5mtNP6TvZ1eTcn64xh4ws152hxio42E-eSnl_CET4tpB5vgP5BVlSKW2xB7w2GJxqdETA5LJRI_OilY77dTOp8cMr_Ck3EOeda3zHfh4Okflg8rZFEeAuJYahQNeAILLkcA"
)

var (
	testClaims = ServiceAccountClaims{
		Sub:                "system:serviceaccount:kube-system:argocd-manager",
		Iss:                "kubernetes/serviceaccount",
		Namespace:          "kube-system",
		SecretName:         "argocd-manager-token-tj79r",
		ServiceAccountName: "argocd-manager",
		ServiceAccountUID:  "91dd37cf-8d92-11e9-a091-d65f2ae7fa8d",
	}
)

func newServiceAccount() *corev1.ServiceAccount {
	saBytes, err := ioutil.ReadFile("./testdata/argocd-manager-sa.yaml")
	errors.CheckError(err)
	var sa corev1.ServiceAccount
	err = yaml.Unmarshal(saBytes, &sa)
	errors.CheckError(err)
	return &sa
}

func newServiceAccountSecret() *corev1.Secret {
	secretBytes, err := ioutil.ReadFile("./testdata/argocd-manager-sa-token.yaml")
	errors.CheckError(err)
	var secret corev1.Secret
	err = yaml.Unmarshal(secretBytes, &secret)
	errors.CheckError(err)
	return &secret
}

func TestParseServiceAccountToken(t *testing.T) {
	claims, err := ParseServiceAccountToken(testToken)
	assert.NoError(t, err)
	assert.Equal(t, testClaims, *claims)
}

func TestCreateServiceAccount(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}
	sa := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ServiceAccount",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-manager",
			Namespace: "kube-system",
		},
	}

	t.Run("New SA", func(t *testing.T) {
		cs := fake.NewSimpleClientset(ns)
		err := CreateServiceAccount(cs, "argocd-manager", "kube-system")
		assert.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("kube-system").Get(context.Background(), "argocd-manager", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, rsa)
	})

	t.Run("SA exists already", func(t *testing.T) {
		cs := fake.NewSimpleClientset(ns, sa)
		err := CreateServiceAccount(cs, "argocd-manager", "kube-system")
		assert.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("kube-system").Get(context.Background(), "argocd-manager", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, rsa)
	})

	t.Run("Invalid name", func(t *testing.T) {
		cs := fake.NewSimpleClientset(ns)
		err := CreateServiceAccount(cs, "", "kube-system")
		assert.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("kube-system").Get(context.Background(), "argocd-manager", metav1.GetOptions{})
		assert.Error(t, err)
		assert.Nil(t, rsa)
	})

	t.Run("Invalid namespace", func(t *testing.T) {
		cs := fake.NewSimpleClientset()
		err := CreateServiceAccount(cs, "argocd-manager", "invalid")
		assert.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("invalid").Get(context.Background(), "argocd-manager", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.NotNil(t, rsa)
	})
}

func TestInstallClusterManagerRBAC(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-secret",
			Namespace: "test",
		},
		Type: corev1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("foobar"),
		},
	}
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArgoCDManagerServiceAccount,
			Namespace: "test",
		},
		Secrets: []corev1.ObjectReference{
			corev1.ObjectReference{
				Kind:            secret.GetObjectKind().GroupVersionKind().Kind,
				APIVersion:      secret.APIVersion,
				Name:            secret.GetName(),
				Namespace:       secret.GetNamespace(),
				UID:             secret.GetUID(),
				ResourceVersion: secret.GetResourceVersion(),
			},
		},
	}

	t.Run("Cluster Scope - Success", func(t *testing.T) {
		cs := fake.NewSimpleClientset(ns, secret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", nil)
		assert.NoError(t, err)
		assert.Equal(t, "foobar", token)
	})

	t.Run("Cluster Scope - Missing data in secret", func(t *testing.T) {
		nsecret := secret.DeepCopy()
		nsecret.Data = make(map[string][]byte)
		cs := fake.NewSimpleClientset(ns, nsecret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", nil)
		assert.Error(t, err)
		assert.Empty(t, token)
	})

	t.Run("Namespace Scope - Success", func(t *testing.T) {
		cs := fake.NewSimpleClientset(ns, secret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", []string{"nsa"})
		assert.NoError(t, err)
		assert.Equal(t, "foobar", token)
	})

	t.Run("Namespace Scope - Missing data in secret", func(t *testing.T) {
		nsecret := secret.DeepCopy()
		nsecret.Data = make(map[string][]byte)
		cs := fake.NewSimpleClientset(ns, nsecret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", []string{"nsa"})
		assert.Error(t, err)
		assert.Empty(t, token)
	})

}

func TestUninstallClusterManagerRBAC(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cs := fake.NewSimpleClientset(newServiceAccountSecret())
		err := UninstallClusterManagerRBAC(cs)
		assert.NoError(t, err)
	})
}

func TestGenerateNewClusterManagerSecret(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(newServiceAccountSecret())
	kubeclientset.ReactionChain = nil

	generatedSecret := newServiceAccountSecret()
	generatedSecret.Name = "argocd-manager-token-abc123"
	generatedSecret.Data = map[string][]byte{
		"token": []byte("fake-token"),
	}

	kubeclientset.AddReactor("*", "secrets", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, generatedSecret, nil
	})

	created, err := GenerateNewClusterManagerSecret(kubeclientset, &testClaims)
	assert.NoError(t, err)
	assert.Equal(t, "argocd-manager-token-abc123", created.Name)
	assert.Equal(t, "fake-token", string(created.Data["token"]))
}

func TestRotateServiceAccountSecrets(t *testing.T) {
	generatedSecret := newServiceAccountSecret()
	generatedSecret.Name = "argocd-manager-token-abc123"
	generatedSecret.Data = map[string][]byte{
		"token": []byte("fake-token"),
	}

	kubeclientset := fake.NewSimpleClientset(newServiceAccount(), newServiceAccountSecret(), generatedSecret)

	err := RotateServiceAccountSecrets(kubeclientset, &testClaims, generatedSecret)
	assert.NoError(t, err)

	// Verify service account references new secret and old secret is deleted
	saClient := kubeclientset.CoreV1().ServiceAccounts(testClaims.Namespace)
	sa, err := saClient.Get(context.Background(), testClaims.ServiceAccountName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, sa.Secrets, []corev1.ObjectReference{
		{
			Name: "argocd-manager-token-abc123",
		},
	})
	secretsClient := kubeclientset.CoreV1().Secrets(testClaims.Namespace)
	_, err = secretsClient.Get(context.Background(), testClaims.SecretName, metav1.GetOptions{})
	assert.True(t, apierr.IsNotFound(err))
}

func TestGetServiceAccountBearerToken(t *testing.T) {
	sa := newServiceAccount()
	tokenSecret := newServiceAccountSecret()
	dockercfgSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-manager-dockercfg-d8j66",
			Namespace: "kube-system",
		},
		Type: corev1.SecretTypeDockercfg,
		// Skipping data, doesn't really matter.
	}
	sa.Secrets = []corev1.ObjectReference{
		{
			Name:      dockercfgSecret.Name,
			Namespace: dockercfgSecret.Namespace,
		},
		{
			Name:      tokenSecret.Name,
			Namespace: tokenSecret.Namespace,
		},
	}
	kubeclientset := fake.NewSimpleClientset(sa, dockercfgSecret, tokenSecret)

	token, err := GetServiceAccountBearerToken(kubeclientset, "kube-system", sa.Name)
	assert.NoError(t, err)
	assert.Equal(t, testToken, token)
}
