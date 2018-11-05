package db

import (
	"context"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/argoproj/argo-cd/common"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"github.com/argoproj/argo-cd/util/settings"
)

const (
	testNamespace = "default"
)

func getClientset(config map[string]string, objects ...runtime.Object) *fake.Clientset {
	secret := v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	cm := v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: testNamespace,
		},
		Data: config,
	}
	return fake.NewSimpleClientset(append(objects, &cm, &secret)...)
}

func TestCreateRepository(t *testing.T) {
	clientset := getClientset(nil)
	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)

	repo, err := db.CreateRepository(context.Background(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
	})
	assert.Nil(t, err)
	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(repoURLToSecretName(repo.Repo), metav1.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, common.ManagedByArgoCDAnnotationValue, secret.Annotations[common.ManagedByAnnotation])
	assert.Equal(t, string(secret.Data[username]), "test-username")
	assert.Equal(t, string(secret.Data[password]), "test-password")
	assert.Nil(t, secret.Data[sshPrivateKey])
}

func TestCreateExistingRepository(t *testing.T) {
	clientset := getClientset(map[string]string{
		"repositories": `- url: https://github.com/argoproj/argocd-example-apps`,
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)

	_, err := db.CreateRepository(context.Background(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
	})
	assert.NotNil(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Convert(err).Code())
}

func TestDeleteRepositoryManagedSecrets(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://github.com/argoproj/argocd-example-apps
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: managed-secret
    key: password
`}
	clientset := getClientset(config, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-secret",
			Namespace: testNamespace,
			Annotations: map[string]string{
				common.ManagedByAnnotation: common.ManagedByArgoCDAnnotationValue,
			},
		},
		Data: map[string][]byte{
			username: []byte("test-username"),
			password: []byte("test-password"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)

	err := db.DeleteRepository(context.Background(), "https://github.com/argoproj/argocd-example-apps")
	assert.Nil(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get("managed-secret", metav1.GetOptions{})
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))

	cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get("argocd-cm", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "", cm.Data["repositories"])
}

func TestDeleteRepositoryUnmanagedSecrets(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://github.com/argoproj/argocd-example-apps
  usernameSecret:
    name: unmanaged-secret
    key: username
  passwordSecret:
    name: unmanaged-secret
    key: password
`}
	clientset := getClientset(config, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unmanaged-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			username: []byte("test-username"),
			password: []byte("test-password"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)

	err := db.DeleteRepository(context.Background(), "https://github.com/argoproj/argocd-example-apps")
	assert.Nil(t, err)

	s, err := clientset.CoreV1().Secrets(testNamespace).Get("unmanaged-secret", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "test-username", string(s.Data[username]))
	assert.Equal(t, "test-password", string(s.Data[password]))

	cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get("argocd-cm", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "", cm.Data["repositories"])
}

func TestUpdateRepositoryWithManagedSecrets(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://github.com/argoproj/argocd-example-apps
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: managed-secret
    key: password
`}
	clientset := getClientset(config, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-secret",
			Namespace: testNamespace,
			Annotations: map[string]string{
				common.ManagedByAnnotation: common.ManagedByArgoCDAnnotationValue,
			},
		},
		Data: map[string][]byte{
			username: []byte("test-username"),
			password: []byte("test-password"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)

	_, err := db.UpdateRepository(context.Background(), &v1alpha1.Repository{Repo: "https://github.com/argoproj/argocd-example-apps", Password: "", Username: ""})
	assert.Nil(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get("managed-secret", metav1.GetOptions{})
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))

	cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get("argocd-cm", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "- url: https://github.com/argoproj/argocd-example-apps", strings.Trim(cm.Data["repositories"], "\n"))
}

func TestGetClusterSuccessful(t *testing.T) {
	clusterURL := "https://mycluster"
	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster-443",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"server": []byte(clusterURL),
			"config": []byte("{}"),
		},
	})

	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)
	cluster, err := db.GetCluster(context.Background(), clusterURL)
	assert.Nil(t, err)
	assert.Equal(t, clusterURL, cluster.Server)
}

func TestGetNonExistingCluster(t *testing.T) {
	clusterURL := "https://mycluster"
	clientset := getClientset(nil)

	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)
	_, err := db.GetCluster(context.Background(), clusterURL)
	assert.NotNil(t, err)
	status, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, status.Code())
}

func TestGetClusterFallbackToLegacyName(t *testing.T) {
	clusterURL := "https://mycluster"
	legacyClusterName := "cluster-mycluster-3274446258"

	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      legacyClusterName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"server": []byte(clusterURL),
			"config": []byte("{}"),
		},
	})

	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)
	cluster, err := db.GetCluster(context.Background(), clusterURL)
	assert.Nil(t, err)
	assert.Equal(t, clusterURL, cluster.Server)
}

func TestCreateClusterSuccessful(t *testing.T) {
	clusterURL := "https://mycluster"
	clientset := getClientset(nil)
	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)

	_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
		Server: clusterURL,
	})
	assert.Nil(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get("mycluster-443", metav1.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, clusterURL, string(secret.Data["server"]))
	assert.Equal(t, common.ManagedByArgoCDAnnotationValue, secret.Annotations[common.ManagedByAnnotation])
}

func TestDeleteClusterWithLegacyName(t *testing.T) {
	clusterURL := "https://mycluster"
	legacyClusterName := "cluster-mycluster-3274446258"

	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      legacyClusterName,
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"server": []byte(clusterURL),
			"config": []byte("{}"),
		},
	})

	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)
	err := db.DeleteCluster(context.Background(), clusterURL)
	assert.Nil(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(legacyClusterName, metav1.GetOptions{})
	assert.NotNil(t, err)

	assert.True(t, errors.IsNotFound(err))
}

func TestDeleteClusterWithUnmanagedSecret(t *testing.T) {
	clusterURL := "https://mycluster"
	clusterName := "mycluster-443"

	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.SecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte(clusterURL),
			"config": []byte("{}"),
		},
	})

	db := NewDB(testNamespace, settings.NewSettingsManager(clientset, testNamespace), clientset)
	err := db.DeleteCluster(context.Background(), clusterURL)
	assert.Nil(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(clusterName, metav1.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, 0, len(secret.Labels))
}
