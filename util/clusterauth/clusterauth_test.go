package clusterauth

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	"sigs.k8s.io/yaml"
)

const (
	testToken              = "eyJhbGciOiJSUzI1NiIsImtpZCI6IiJ9.eyJpc3MiOiJrdWJlcm5ldGVzL3NlcnZpY2VhY2NvdW50Iiwia3ViZXJuZXRlcy5pby9zZXJ2aWNlYWNjb3VudC9uYW1lc3BhY2UiOiJrdWJlLXN5c3RlbSIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VjcmV0Lm5hbWUiOiJhcmdvY2QtbWFuYWdlci10b2tlbi10ajc5ciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50Lm5hbWUiOiJhcmdvY2QtbWFuYWdlciIsImt1YmVybmV0ZXMuaW8vc2VydmljZWFjY291bnQvc2VydmljZS1hY2NvdW50LnVpZCI6IjkxZGQzN2NmLThkOTItMTFlOS1hMDkxLWQ2NWYyYWU3ZmE4ZCIsInN1YiI6InN5c3RlbTpzZXJ2aWNlYWNjb3VudDprdWJlLXN5c3RlbTphcmdvY2QtbWFuYWdlciJ9.ytZjt2pDV8-A7DBMR06zQ3wt9cuVEfq262TQw7sdra-KRpDpMPnziMhc8bkwvgW-LGhTWUh5iu1y-1QhEx6mtbCt7vQArlBRxfvM5ys6ClFkplzq5c2TtZ7EzGSD0Up7tdxuG9dvR6TGXYdfFcG779yCdZo2H48sz5OSJfdEriduMEY1iL5suZd3ebOoVi1fGflmqFEkZX6SvxkoArl5mtNP6TvZ1eTcn64xh4ws152hxio42E-eSnl_CET4tpB5vgP5BVlSKW2xB7w2GJxqdETA5LJRI_OilY77dTOp8cMr_Ck3EOeda3zHfh4Okflg8rZFEeAuJYahQNeAILLkcA"
	testBearerTokenTimeout = 5 * time.Second
)

var testClaims = ServiceAccountClaims{
	"kube-system",
	"argocd-manager-token-tj79r",
	"argocd-manager",
	"91dd37cf-8d92-11e9-a091-d65f2ae7fa8d",
	jwt.RegisteredClaims{
		Subject: "system:serviceaccount:kube-system:argocd-manager",
		Issuer:  "kubernetes/serviceaccount",
	},
}

func newServiceAccount(t *testing.T) *corev1.ServiceAccount {
	t.Helper()
	saBytes, err := os.ReadFile("./testdata/argocd-manager-sa.yaml")
	require.NoError(t, err)
	var sa corev1.ServiceAccount
	err = yaml.Unmarshal(saBytes, &sa)
	require.NoError(t, err)
	return &sa
}

func newServiceAccountSecret(t *testing.T) *corev1.Secret {
	t.Helper()
	secretBytes, err := os.ReadFile("./testdata/argocd-manager-sa-token.yaml")
	require.NoError(t, err)
	var secret corev1.Secret
	err = yaml.Unmarshal(secretBytes, &secret)
	require.NoError(t, err)
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
		rsa, err := cs.CoreV1().ServiceAccounts("kube-system").Get(t.Context(), "argocd-manager", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, rsa)
	})

	t.Run("SA exists already", func(t *testing.T) {
		cs := fake.NewClientset(ns, sa)
		err := CreateServiceAccount(cs, "argocd-manager", "kube-system")
		require.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("kube-system").Get(t.Context(), "argocd-manager", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, rsa)
	})

	t.Run("Invalid namespace", func(t *testing.T) {
		cs := fake.NewClientset()
		err := CreateServiceAccount(cs, "argocd-manager", "invalid")
		require.NoError(t, err)
		rsa, err := cs.CoreV1().ServiceAccounts("invalid").Get(t.Context(), "argocd-manager", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, rsa)
	})
}

func _MockK8STokenController(objects kubetesting.ObjectTracker) kubetesting.ReactionFunc {
	return (func(action kubetesting.Action) (bool, runtime.Object, error) {
		secret, ok := action.(kubetesting.CreateAction).GetObject().(*corev1.Secret)
		if !ok {
			return false, nil, nil
		}
		_, err := objects.Get(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
			secret.Namespace,
			secret.Annotations[corev1.ServiceAccountNameKey],
			metav1.GetOptions{})
		if err != nil {
			return false, nil, nil
		}
		if secret.Data == nil {
			secret.Data = map[string][]byte{}
		}
		if secret.Data[corev1.ServiceAccountTokenKey] == nil {
			secret.Data[corev1.ServiceAccountTokenKey] = []byte(testToken)
		}
		return false, secret, nil
	})
}

func TestInstallClusterManagerRBAC(t *testing.T) {
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}
	legacyAutoSecret := &corev1.Secret{
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
				Kind:            legacyAutoSecret.GetObjectKind().GroupVersionKind().Kind,
				APIVersion:      legacyAutoSecret.APIVersion,
				Name:            legacyAutoSecret.GetName(),
				Namespace:       legacyAutoSecret.GetNamespace(),
				UID:             legacyAutoSecret.GetUID(),
				ResourceVersion: legacyAutoSecret.GetResourceVersion(),
			},
		},
	}
	longLivedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa.Name + SATokenSecretSuffix,
			Namespace: "test",
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
		Data: map[string][]byte{
			"token": []byte("barfoo"),
		},
	}

	t.Run("Cluster Scope - Success", func(t *testing.T) {
		cs := fake.NewClientset(ns, legacyAutoSecret, sa)
		cs.PrependReactor("create", "secrets", _MockK8STokenController(cs.Tracker()))
		token, err := InstallClusterManagerRBAC(cs, "test", nil, testBearerTokenTimeout)
		require.NoError(t, err)
		assert.Equal(t, testToken, token)
	})

	t.Run("Cluster Scope - Missing data in secret", func(t *testing.T) {
		nsecret := legacyAutoSecret.DeepCopy()
		nsecret.Data = make(map[string][]byte)
		cs := fake.NewClientset(ns, nsecret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", nil, testBearerTokenTimeout)
		require.Error(t, err)
		assert.Empty(t, token)
	})

	t.Run("Namespace Scope - Success", func(t *testing.T) {
		cs := fake.NewClientset(ns, sa, longLivedSecret)
		cs.PrependReactor("create", "secrets", _MockK8STokenController(cs.Tracker()))
		token, err := InstallClusterManagerRBAC(cs, "test", []string{"nsa"}, testBearerTokenTimeout)
		require.NoError(t, err)
		assert.Equal(t, "barfoo", token)
	})

	t.Run("Namespace Scope - Missing data in secret", func(t *testing.T) {
		nsecret := legacyAutoSecret.DeepCopy()
		nsecret.Data = make(map[string][]byte)
		cs := fake.NewClientset(ns, nsecret, sa)
		token, err := InstallClusterManagerRBAC(cs, "test", []string{"nsa"}, testBearerTokenTimeout)
		require.Error(t, err)
		assert.Empty(t, token)
	})
}

func TestUninstallClusterManagerRBAC(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cs := fake.NewClientset(newServiceAccountSecret(t))
		err := UninstallClusterManagerRBAC(cs)
		require.NoError(t, err)
	})
}

func TestGenerateNewClusterManagerSecret(t *testing.T) {
	kubeclientset := fake.NewClientset(newServiceAccountSecret(t))
	kubeclientset.ReactionChain = nil

	generatedSecret := newServiceAccountSecret(t)
	generatedSecret.Name = "argocd-manager-token-abc123"
	generatedSecret.Data = map[string][]byte{
		"token": []byte("fake-token"),
	}

	kubeclientset.AddReactor("*", "secrets", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return true, generatedSecret, nil
	})

	created, err := GenerateNewClusterManagerSecret(kubeclientset, &testClaims)
	require.NoError(t, err)
	assert.Equal(t, "argocd-manager-token-abc123", created.Name)
	assert.Equal(t, "fake-token", string(created.Data["token"]))
}

func TestRotateServiceAccountSecrets(t *testing.T) {
	generatedSecret := newServiceAccountSecret(t)
	generatedSecret.Name = "argocd-manager-token-abc123"
	generatedSecret.Data = map[string][]byte{
		"token": []byte("fake-token"),
	}

	kubeclientset := fake.NewClientset(newServiceAccount(t), newServiceAccountSecret(t), generatedSecret)

	err := RotateServiceAccountSecrets(kubeclientset, &testClaims, generatedSecret)
	require.NoError(t, err)

	// Verify service account references new secret and old secret is deleted
	saClient := kubeclientset.CoreV1().ServiceAccounts(testClaims.Namespace)
	sa, err := saClient.Get(t.Context(), testClaims.ServiceAccountName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, []corev1.ObjectReference{
		{
			Name: "argocd-manager-token-abc123",
		},
	}, sa.Secrets)
	secretsClient := kubeclientset.CoreV1().Secrets(testClaims.Namespace)
	_, err = secretsClient.Get(t.Context(), testClaims.SecretName, metav1.GetOptions{})
	assert.True(t, apierrors.IsNotFound(err))
}

func TestGetServiceAccountBearerToken(t *testing.T) {
	sa := newServiceAccount(t)
	tokenSecret := newServiceAccountSecret(t)
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
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArgoCDManagerServiceAccount,
			Namespace: ns.Name,
		},
	}
	manualSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArgoCDManagerServiceAccount + SATokenSecretSuffix,
			Namespace: ns.Name,
			Annotations: map[string]string{
				corev1.ServiceAccountNameKey: sa.Name,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}

	assertOnlyOneTokenExists := func(t *testing.T, cs *fake.Clientset) {
		t.Helper()
		got, err := getOrCreateServiceAccountTokenSecret(cs, ArgoCDManagerServiceAccount, ns.Name)
		require.NoError(t, err)
		assert.Equal(t, ArgoCDManagerServiceAccount+SATokenSecretSuffix, got)

		list, err := cs.Tracker().List(schema.GroupVersionResource{Version: "v1", Resource: "secrets"},
			schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, ns.Name, metav1.ListOptions{})
		require.NoError(t, err)
		secretList, ok := list.(*corev1.SecretList)
		require.True(t, ok)
		assert.Len(t, secretList.Items, 1)
		obj, err := cs.Tracker().Get(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
			ns.Name, ArgoCDManagerServiceAccount)
		require.NoError(t, err, "ServiceAccount %s not found but was expected to be found", ArgoCDManagerServiceAccount)

		assert.Empty(t, obj.(*corev1.ServiceAccount).Secrets, 0)
	}
	t.Run("Token secret exists", func(t *testing.T) {
		cs := fake.NewClientset(ns, sa, manualSecret)
		assertOnlyOneTokenExists(t, cs)
	})

	t.Run("Token secret does not exist", func(t *testing.T) {
		cs := fake.NewClientset(ns, sa)
		assertOnlyOneTokenExists(t, cs)
	})

	t.Run("Error on secret creation", func(t *testing.T) {
		cs := fake.NewClientset(ns, sa)
		cs.PrependReactor("create", "secrets", func(kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, &corev1.Secret{}, errors.New("testing error case")
		})
		got, err := getOrCreateServiceAccountTokenSecret(cs, ArgoCDManagerServiceAccount, ns.Name)
		require.Error(t, err)
		assert.Empty(t, got)
	})
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
	assert.Equal(t, ArgoCDManagerServiceAccount+SATokenSecretSuffix, got)

	obj, err := cs.Tracker().Get(schema.GroupVersionResource{Version: "v1", Resource: "serviceaccounts"},
		ns.Name, ArgoCDManagerServiceAccount)
	require.NoError(t, err, "ServiceAccount %s not found but was expected to be found", ArgoCDManagerServiceAccount)

	sa := obj.(*corev1.ServiceAccount)
	assert.Len(t, sa.Secrets, 1)

	// Adding if statement to prevent case where secret not found
	// since accessing name by first index.
	if len(sa.Secrets) != 0 {
		assert.Equal(t, "sa-secret", sa.Secrets[0].Name)
	}
}
