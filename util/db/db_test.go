package db

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
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
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: config,
	}
	return fake.NewSimpleClientset(append(objects, &cm, &secret)...)
}

func TestCreateRepository(t *testing.T) {
	clientset := getClientset(nil)
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	repo, err := db.CreateRepository(context.Background(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
	})
	assert.Nil(t, err)
	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), RepoURLToSecretName(repoSecretPrefix, repo.Repo), metav1.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
	assert.Equal(t, string(secret.Data[username]), "test-username")
	assert.Equal(t, string(secret.Data[password]), "test-password")
	assert.Empty(t, secret.Data[sshPrivateKey])
}

func TestCreateRepoCredentials(t *testing.T) {
	clientset := getClientset(nil)
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	creds, err := db.CreateRepositoryCredentials(context.Background(), &v1alpha1.RepoCreds{
		URL:      "https://github.com/argoproj/",
		Username: "test-username",
		Password: "test-password",
	})
	assert.Nil(t, err)
	assert.Equal(t, "https://github.com/argoproj/", creds.URL)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), RepoURLToSecretName(credSecretPrefix, creds.URL), metav1.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
	assert.Equal(t, string(secret.Data[username]), "test-username")
	assert.Equal(t, string(secret.Data[password]), "test-password")
	assert.Empty(t, secret.Data[sshPrivateKey])

	created, err := db.CreateRepository(context.Background(), &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argo-cd",
	})
	assert.Nil(t, err)
	assert.Equal(t, "https://github.com/argoproj/argo-cd", created.Repo)

	// There seems to be a race or some other hiccup in the fake K8s clientset used for this test.
	// Just give it a little time to settle.
	time.Sleep(1 * time.Second)

	repo, err := db.GetRepository(context.Background(), created.Repo)
	assert.NoError(t, err)
	assert.Equal(t, "test-username", repo.Username)
	assert.Equal(t, "test-password", repo.Password)
}

func TestGetRepositoryCredentials(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://known/repo
- url: https://secured/repo
- url: https://missing/repo
`,
		"repository.credentials": `
- url: https://secured
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: managed-secret
    key: password
- url: https://missing
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: missing-managed-secret
    key: password
`}
	clientset := getClientset(config, newManagedSecret())
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	tests := []struct {
		name    string
		repoURL string
		want    *v1alpha1.RepoCreds
		wantErr bool
	}{
		{
			name:    "TestUnknownRepo",
			repoURL: "https://unknown/repo",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "TestKnownRepo",
			repoURL: "https://known/repo",
			want:    nil,
			wantErr: false,
		},
		{
			name:    "TestSecuredRepo",
			repoURL: "https://secured/repo",
			want:    &v1alpha1.RepoCreds{URL: "https://secured", Username: "test-username", Password: "test-password"},
			wantErr: false,
		},
		{
			name:    "TestMissingRepo",
			repoURL: "https://missing/repo",
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetRepositoryCredentials(context.TODO(), tt.repoURL)

			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.IsNotFound(err))
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateExistingRepository(t *testing.T) {
	clientset := getClientset(map[string]string{
		"repositories": `- url: https://github.com/argoproj/argocd-example-apps`,
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	_, err := db.CreateRepository(context.Background(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
	})
	assert.NotNil(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Convert(err).Code())
}

func TestGetRepository(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://known/repo
- url: https://secured/repo
`,
		"repository.credentials": `
- url: https://secured
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: managed-secret
    key: password
`}
	clientset := getClientset(config, newManagedSecret())
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	tests := []struct {
		name    string
		repoURL string
		want    *v1alpha1.Repository
	}{
		{
			name:    "TestUnknownRepo",
			repoURL: "https://unknown/repo",
			want:    &v1alpha1.Repository{Repo: "https://unknown/repo"},
		},
		{
			name:    "TestKnownRepo",
			repoURL: "https://known/repo",
			want:    &v1alpha1.Repository{Repo: "https://known/repo"},
		},
		{
			name:    "TestSecuredRepo",
			repoURL: "https://secured/repo",
			want:    &v1alpha1.Repository{Repo: "https://secured/repo", Username: "test-username", Password: "test-password", InheritedCreds: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetRepository(context.TODO(), tt.repoURL)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func newManagedSecret() *v1.Secret {
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-secret",
			Namespace: testNamespace,
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
		Data: map[string][]byte{
			username: []byte("test-username"),
			password: []byte("test-password"),
		},
	}
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
	clientset := getClientset(config, newManagedSecret())
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	err := db.DeleteRepository(context.Background(), "https://github.com/argoproj/argocd-example-apps")
	assert.Nil(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), "managed-secret", metav1.GetOptions{})
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))

	cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), "argocd-cm", metav1.GetOptions{})
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
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	err := db.DeleteRepository(context.Background(), "https://github.com/argoproj/argocd-example-apps")
	assert.Nil(t, err)

	s, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), "unmanaged-secret", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "test-username", string(s.Data[username]))
	assert.Equal(t, "test-password", string(s.Data[password]))

	cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), "argocd-cm", metav1.GetOptions{})
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
  sshPrivateKeySecret:
    name: managed-secret
    key: sshPrivateKey
`}
	clientset := getClientset(config, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-secret",
			Namespace: testNamespace,
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
		Data: map[string][]byte{
			username:      []byte("test-username"),
			password:      []byte("test-password"),
			sshPrivateKey: []byte("test-ssh-private-key"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	repo, err := db.GetRepository(context.Background(), "https://github.com/argoproj/argocd-example-apps")
	assert.Nil(t, err)
	assert.Equal(t, "test-username", repo.Username)
	assert.Equal(t, "test-password", repo.Password)
	assert.Equal(t, "test-ssh-private-key", repo.SSHPrivateKey)

	_, err = db.UpdateRepository(context.Background(), &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argocd-example-apps", Password: "", Username: "", SSHPrivateKey: ""})
	assert.Nil(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), "managed-secret", metav1.GetOptions{})
	assert.NotNil(t, err)
	assert.True(t, errors.IsNotFound(err))

	cm, err := clientset.CoreV1().ConfigMaps(testNamespace).Get(context.Background(), "argocd-cm", metav1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, "- url: https://github.com/argoproj/argocd-example-apps", strings.Trim(cm.Data["repositories"], "\n"))
}

func TestRepositorySecretsTrim(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://github.com/argoproj/argocd-example-apps
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: managed-secret
    key: password
  sshPrivateKeySecret:
    name: managed-secret
    key: sshPrivateKey
  tlsClientCertDataSecret:
    name: managed-secret
    key: tlsClientCertData
  tlsClientCertKeySecret:
    name: managed-secret
    key: tlsClientCertKey
  githubAppPrivateKeySecret:
    name: managed-secret
    key: githubAppPrivateKey
`}
	clientset := getClientset(config, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managed-secret",
			Namespace: testNamespace,
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
		Data: map[string][]byte{
			username:            []byte("test-username\n\n"),
			password:            []byte("test-password\r\r"),
			sshPrivateKey:       []byte("test-ssh-private-key\n\r"),
			tlsClientCertData:   []byte("test-tls-client-cert-data\n\r"),
			tlsClientCertKey:    []byte("test-tls-client-cert-key\n\r"),
			githubAppPrivateKey: []byte("test-github-app-private-key\n\r"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	repo, err := db.GetRepository(context.Background(), "https://github.com/argoproj/argocd-example-apps")
	assert.Nil(t, err)
	teststruct := []struct {
		expectedSecret  string
		retrievedSecret string
	}{
		{
			"test-username",
			repo.Username,
		},
		{
			"test-password",
			repo.Password,
		},
		{
			"test-ssh-private-key",
			repo.SSHPrivateKey,
		},
		{
			"test-tls-client-cert-data",
			repo.TLSClientCertData,
		},
		{
			"test-tls-client-cert-key",
			repo.TLSClientCertKey,
		},
		{
			"test-github-app-private-key",
			repo.GithubAppPrivateKey,
		},
	}
	for _, tt := range teststruct {
		assert.Equal(t, tt.expectedSecret, tt.retrievedSecret)
	}
}

func TestGetClusterSuccessful(t *testing.T) {
	server := "my-cluster"
	name := "my-name"
	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte(server),
			"name":   []byte(name),
			"config": []byte("{}"),
		},
	})

	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	cluster, err := db.GetCluster(context.Background(), server)
	assert.NoError(t, err)
	assert.Equal(t, server, cluster.Server)
	assert.Equal(t, name, cluster.Name)
}

func TestGetNonExistingCluster(t *testing.T) {
	server := "https://mycluster"
	clientset := getClientset(nil)

	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	_, err := db.GetCluster(context.Background(), server)
	assert.NotNil(t, err)
	status, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, status.Code())
}

func TestCreateClusterSuccessful(t *testing.T) {
	server := "https://mycluster"
	clientset := getClientset(nil)
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
		Server: server,
	})
	assert.Nil(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), "cluster-mycluster-3274446258", metav1.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, server, string(secret.Data["server"]))
	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
}

func TestDeleteClusterWithManagedSecret(t *testing.T) {
	clusterURL := "https://mycluster"
	clusterName := "cluster-mycluster-3274446258"

	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
		Data: map[string][]byte{
			"server": []byte(clusterURL),
			"config": []byte("{}"),
		},
	})

	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	err := db.DeleteCluster(context.Background(), clusterURL)
	assert.Nil(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), clusterName, metav1.GetOptions{})
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
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte(clusterURL),
			"config": []byte("{}"),
		},
	})

	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	err := db.DeleteCluster(context.Background(), clusterURL)
	assert.Nil(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(context.Background(), clusterName, metav1.GetOptions{})
	assert.Nil(t, err)

	assert.Equal(t, 0, len(secret.Labels))
}

func TestFuzzyEquivalence(t *testing.T) {
	clientset := getClientset(nil)
	ctx := context.Background()
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	repo, err := db.CreateRepository(ctx, &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argocd-example-apps",
	})
	assert.Nil(t, err)
	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)

	repo, err = db.CreateRepository(ctx, &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argocd-example-apps.git",
	})
	assert.Contains(t, err.Error(), "already exists")
	assert.Nil(t, repo)

	repo, err = db.CreateRepository(ctx, &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argocd-example-APPS",
	})
	assert.Contains(t, err.Error(), "already exists")
	assert.Nil(t, repo)

	repo, err = db.GetRepository(ctx, "https://github.com/argoproj/argocd-example-APPS")
	assert.Nil(t, err)
	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)
}

func TestListHelmRepositories(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://argoproj.github.io/argo-helm
  name: argo
  type: helm
  usernameSecret:
    name: test-secret
    key: username
  passwordSecret:
    name: test-secret
    key: password
  tlsClientCertDataSecret:
    name: test-secret
    key: cert
  tlsClientCertKeySecret:
    name: test-secret
    key: key
`}
	clientset := getClientset(config, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"username": []byte("test-username"),
			"password": []byte("test-password"),
			"ca":       []byte("test-ca"),
			"cert":     []byte("test-cert"),
			"key":      []byte("test-key"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)

	repos, err := db.ListRepositories(context.Background())
	assert.Nil(t, err)
	assert.Len(t, repos, 1)
	repo := repos[0]
	assert.Equal(t, "https://argoproj.github.io/argo-helm", repo.Repo)
	assert.Equal(t, "helm", repo.Type)
	assert.Equal(t, "argo", repo.Name)
	assert.Equal(t, "test-username", repo.Username)
	assert.Equal(t, "test-password", repo.Password)
	assert.Equal(t, "test-cert", repo.TLSClientCertData)
	assert.Equal(t, "test-key", repo.TLSClientCertKey)
}

func TestHelmRepositorySecretsTrim(t *testing.T) {
	config := map[string]string{
		"repositories": `
- url: https://argoproj.github.io/argo-helm
  name: argo
  type: helm
  usernameSecret:
    name: test-secret
    key: username
  passwordSecret:
    name: test-secret
    key: password
  tlsClientCertDataSecret:
    name: test-secret
    key: cert
  tlsClientCertKeySecret:
    name: test-secret
    key: key
`}
	clientset := getClientset(config, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"username": []byte("test-username\r\n"),
			"password": []byte("test-password\r\n"),
			"cert":     []byte("test-cert\n\r"),
			"key":      []byte("test-key\n\r"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	repo, err := db.GetRepository(context.Background(), "https://argoproj.github.io/argo-helm")

	assert.Nil(t, err)
	teststruct := []struct {
		expectedSecret  string
		retrievedSecret string
	}{
		{
			"test-username",
			repo.Username,
		},
		{
			"test-password",
			repo.Password,
		},
		{
			"test-cert",
			repo.TLSClientCertData,
		},
		{
			"test-key",
			repo.TLSClientCertKey,
		},
	}
	for _, tt := range teststruct {
		assert.Equal(t, tt.expectedSecret, tt.retrievedSecret)
	}
}

func TestGetClusterServersByName(t *testing.T) {
	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
		Data: map[string][]byte{
			"name":   []byte("my-cluster-name"),
			"server": []byte("https://my-cluster-server"),
			"config": []byte("{}"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	servers, err := db.GetClusterServersByName(context.Background(), "my-cluster-name")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"https://my-cluster-server"}, servers)
}

func TestGetClusterServersByName_InClusterNotConfigured(t *testing.T) {
	clientset := getClientset(nil)
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	servers, err := db.GetClusterServersByName(context.Background(), "in-cluster")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{v1alpha1.KubernetesInternalAPIServerAddr}, servers)
}

func TestGetClusterServersByName_InClusterConfigured(t *testing.T) {
	clientset := getClientset(nil, &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster-secret",
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
		},
		Data: map[string][]byte{
			"name":   []byte("in-cluster-renamed"),
			"server": []byte(v1alpha1.KubernetesInternalAPIServerAddr),
			"config": []byte("{}"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(context.Background(), clientset, testNamespace), clientset)
	servers, err := db.GetClusterServersByName(context.Background(), "in-cluster-renamed")
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{v1alpha1.KubernetesInternalAPIServerAddr}, servers)
}
