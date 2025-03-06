package clusterauth

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/util/errors"
)

const (
	testToken              = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJrdWJlLXN5c3RlbSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VjcmV0Lm5hbWUiOiJhcmdvY2QtbWFuYWdlci10b2tlbi10ajc5ciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJhcmdvY2QtbWFuYWdlciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjkxZGQzN2NmLThkOTItMTFlOS1hMDkxLWQ2NWYyYWU3ZmE4ZCIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDprdWJlLXN5c3RlbTphcmdvY2QtbWFuYWdlciJ9.ytZjt2pDV8-A7DBMR06zQ3wt9cuVEfq262TQw7sdra-KRpDpMPnziMhc8bkwvgW-LGhTWUh5iu1y-1QhEx6mtbCt7vQArlBRxfvM5ys6ClFkplzq5c2TtZ7EzGSD0Up7tdxuG9dvR6TGXYdfFcG779yCdZo2H48sz5OSJfdEriduMEY1iL5suZd3ebOoVi1fGflmqFEkZX6SvxkoArl5mtNP6TvZ1eTcn64xh4ws152hxio42E-eSnl_CET4tpB5vgP5BVlSKW2xB7w2GJxqdETA5LJRI_OilY77dTOp8cMr_Ck3EOeda3zHfh4Okflg8rZFEeAuJYahQNeAILLkcA"
	testBearerTokenTimeout = 5 * time.Second
)

var testClaims = ServiceAccountClaims{
	Sub:                "system:serviceaccount:kube-system:argocd-manager",
	Iss:                "kubernetes/serviceaccount",
	Namespace:          "kube-system",
	SecretName:         "argocd-manager-token-tj79r",
	ServiceAccountName: "argocd-manager",
	ServiceAccountUID:  "91dd37cf-8d92-11e9-a091-d65f2ae7fa8d",
}

func newServiceAccount() *corev1.ServiceAccount {
	saBytes, err := os.ReadFile("./testdata/argocd-manager-sa.yaml")
	errors.CheckError(err)
	var sa corev1.ServiceAccount
	err = yaml.Unmarshal(saBytes, &sa)
	errors.CheckError(err)
	return &sa
}

func newServiceAccountSecret() *corev1.Secret {
	secretBytes, err := os.ReadFile("./testdata/argocd-manager-sa-token.yaml")
	errors.CheckError(err)
	var secret corev1.Secret
	err = yaml.Unmarshal(secretBytes, &secret)
	errors.CheckError(err)
	return &secret
}

func TestParseServiceAccountToken(t *testing.T) {
	claims, err := ParseServiceAccountToken(testToken)
	require.NoError(t, err)
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
		cs := fake.NewClientset(ns)
		err := CreateServiceAccount(cs, "argocd-manager", "kube-system")
		require.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("kube-system").Get(context.Background(), "argocd-manager", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, rsa)
	})

	t.Run("SA exists already", func(t *testing.T) {
		cs := fake.NewClientset(ns, sa)
		err := CreateServiceAccount(cs, "argocd-manager", "kube-system")
		require.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("kube-system").Get(context.Background(), "argocd-manager", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, rsa)
	})

	t.Run("Invalid namespace", func(t *testing.T) {
		cs := fake.NewClientset()
		err := CreateServiceAccount(cs, "argocd-manager", "invalid")
		require.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("invalid").Get(context.Background(), "argocd-manager", metav1.GetOptions{})
		require.NoError(t, err)
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
			{
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
		cs := fake.NewClientset(ns, secret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", nil, testBearerTokenTimeout)
		require.NoError(t, err)
		assert.Equal(t, "foobar", token)
	})

	t.Run("Cluster Scope - Missing data in secret", func(t *testing.T) {
		nsecret := secret.DeepCopy()
		nsecret.Data = make(map[string][]byte)
		cs := fake.NewClientset(ns, nsecret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", nil, testBearerTokenTimeout)
		require.Error(t, err)
		assert.Empty(t, token)
	})

	t.Run("Namespace Scope - Success", func(t *testing.T) {
		cs := fake.NewClientset(ns, secret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", []string{"nsa"}, testBearerTokenTimeout)
		require.NoError(t, err)
		assert.Equal(t, "foobar", token)
	})

	t.Run("Namespace Scope - Missing data in secret", func(t *testing.T) {
		nsecret := secret.DeepCopy()
		nsecret.Data = make(map[string][]byte)
		cs := fake.NewClientset(ns, nsecret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", []string{"nsa"}, testBearerTokenTimeout)
		require.Error(t, err)
		assert.Empty(t, token)
	})
}

func TestUninstallClusterManagerRBAC(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cs := fake.NewClientset(newServiceAccountSecret())
		err := UninstallClusterManagerRBAC(cs)
		require.NoError(t, err)
	})
}

func TestGenerateNewClusterManagerSecret(t *testing.T) {
	kubeclientset := fake.NewClientset(newServiceAccountSecret())
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
	require.NoError(t, err)
	assert.Equal(t, "argocd-manager-token-abc123", created.Name)
	assert.Equal(t, "fake-token", string(created.Data["token"]))
}

func TestRotateServiceAccountSecrets(t *testing.T) {
	generatedSecret := newServiceAccountSecret()
	generatedSecret.Name = "argocd-manager-token-abc123"
	generatedSecret.Data = map[string][]byte{
		"token": []byte("fake-token"),
	}

	kubeclientset := fake.NewClientset(newServiceAccount(), newServiceAccountSecret(), generatedSecret)

	err := RotateServiceAccountSecrets(kubeclientset, &testClaims, generatedSecret)
	require.NoError(t, err)

	// Verify service account references new secret and old secret is deleted
	saClient := kubeclientset.CoreV1().ServiceAccounts(testClaims.Namespace)
	sa, err := saClient.Get(context.Background(), testClaims.ServiceAccountName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []corev1.ObjectReference{
		{
			Name: "argocd-manager-token-abc123",
		},
	}, sa.Secrets)
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
	kubeclientset := fake.NewClientset(sa, dockercfgSecret, tokenSecret)

	token, err := GetServiceAccountBearerToken(kubeclientset, "kube-system", sa.Name, testBearerTokenTimeout)
	require.NoError(t, err)
	assert.Equal(t, testToken, token)
}

func Test_getOrCreateServiceAccountTokenSecret_NoSecretForSA(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}
	saWithoutSecret := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArgoCDManagerServiceAccount,
			Namespace: ns.Name,
		},
	}
	cs := fake.NewClientset(ns, saWithoutSecret)
	cs.PrependReactor("create", "secrets",
		func(a kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			s, ok := a.(kubetesting.CreateAction).GetObject().(*corev1.Secret)
			if !ok {
				return
			}

			if s.Name == "" && s.GenerateName != "" {
				s.SetName(names.SimpleNameGenerator.GenerateName(s.GenerateName))
			}

			s.Data = make(map[string][]byte)
			s.Data["token"] = []byte("fake-token")

			return
		})

	got, err := getOrCreateServiceAccountTokenSecret(cs, ArgoCDManagerServiceAccount, ns.Name)
	require.NoError(t, err)
	assert.Contains(t, got, "argocd-manager-token-")

	obj, err := cs.Tracker().Get(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
		ns.Name, ArgoCDManagerServiceAccount)
	if err != nil {
		t.Errorf("ServiceAccount %s not found but was expected to be found: %s", ArgoCDManagerServiceAccount, err.Error())
	}

	sa := obj.(*corev1.ServiceAccount)
	assert.Len(t, sa.Secrets, 1)
}

func Test_getOrCreateServiceAccountTokenSecret_SAHasSecret(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-secret",
			Namespace: ns.Name,
		},
		Type: corev1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("foobar"),
		},
	}

	saWithSecret := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArgoCDManagerServiceAccount,
			Namespace: ns.Name,
		},
		Secrets: []corev1.ObjectReference{
			{
				Kind:            secret.GetObjectKind().GroupVersionKind().Kind,
				APIVersion:      secret.APIVersion,
				Name:            secret.GetName(),
				Namespace:       secret.GetNamespace(),
				UID:             secret.GetUID(),
				ResourceVersion: secret.GetResourceVersion(),
			},
		},
	}

	cs := fake.NewClientset(ns, saWithSecret, secret)

	got, err := getOrCreateServiceAccountTokenSecret(cs, ArgoCDManagerServiceAccount, ns.Name)
	require.NoError(t, err)
	assert.Equal(t, "sa-secret", got)

	obj, err := cs.Tracker().Get(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
		ns.Name, ArgoCDManagerServiceAccount)
	if err != nil {
		t.Errorf("ServiceAccount %s not found but was expected to be found: %s", ArgoCDManagerServiceAccount, err.Error())
	}

	sa := obj.(*corev1.ServiceAccount)
	assert.Len(t, sa.Secrets, 1)

	// Adding if statement to prevent case where secret not found
	// since accessing name by first index.
	if len(sa.Secrets) != 0 {
		assert.Equal(t, "sa-secret", sa.Secrets[0].Name)
	}
}
