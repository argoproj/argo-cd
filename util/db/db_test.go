package db

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

const (
	testNamespace = "default"
)

func getClientset(objects ...runtime.Object) *fake.Clientset {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: testNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: testNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}
	return fake.NewClientset(append(objects, &cm, &secret)...)
}

func TestCreateRepository(t *testing.T) {
	clientset := getClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	repo, err := db.CreateRepository(t.Context(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), RepoURLToSecretName(repoSecretPrefix, repo.Repo, ""), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
	assert.Equal(t, "test-username", string(secret.Data[username]))
	assert.Equal(t, "test-password", string(secret.Data[password]))
	assert.Empty(t, secret.Data[sshPrivateKey])
}

func TestCreateProjectScopedRepository(t *testing.T) {
	clientset := getClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	repo, err := db.CreateRepository(t.Context(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
		Project:  "test-project",
	})
	require.NoError(t, err)

	otherRepo, err := db.CreateRepository(t.Context(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "other-username",
		Password: "other-password",
		Project:  "other-project",
	})
	require.NoError(t, err)

	_, err = db.CreateRepository(t.Context(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "wrong-username",
		Password: "wrong-password",
	})
	require.NoError(t, err)

	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), RepoURLToSecretName(repoSecretPrefix, repo.Repo, "test-project"), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
	assert.Equal(t, "test-username", string(secret.Data[username]))
	assert.Equal(t, "test-password", string(secret.Data[password]))
	assert.Equal(t, "test-project", string(secret.Data[project]))
	assert.Empty(t, secret.Data[sshPrivateKey])

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), RepoURLToSecretName(repoSecretPrefix, otherRepo.Repo, "other-project"), metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
	assert.Equal(t, "other-username", string(secret.Data[username]))
	assert.Equal(t, "other-password", string(secret.Data[password]))
	assert.Equal(t, "other-project", string(secret.Data[project]))
	assert.Empty(t, secret.Data[sshPrivateKey])
}

func TestCreateRepoCredentials(t *testing.T) {
	clientset := getClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	creds, err := db.CreateRepositoryCredentials(t.Context(), &v1alpha1.RepoCreds{
		URL:      "https://github.com/argoproj/",
		Username: "test-username",
		Password: "test-password",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/argoproj/", creds.URL)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), RepoURLToSecretName(credSecretPrefix, creds.URL, ""), metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
	assert.Equal(t, "test-username", string(secret.Data[username]))
	assert.Equal(t, "test-password", string(secret.Data[password]))
	assert.Empty(t, secret.Data[sshPrivateKey])

	created, err := db.CreateRepository(t.Context(), &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argo-cd",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/argoproj/argo-cd", created.Repo)

	// There seems to be a race or some other hiccup in the fake K8s clientset used for this test.
	// Just give it a little time to settle.
	time.Sleep(1 * time.Second)

	repo, err := db.GetRepository(t.Context(), created.Repo, "")
	require.NoError(t, err)
	assert.Equal(t, "test-username", repo.Username)
	assert.Equal(t, "test-password", repo.Password)
}

func TestGetRepositoryCredentials(t *testing.T) {
	clientset := getClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)
	_, err := db.CreateRepositoryCredentials(t.Context(), &v1alpha1.RepoCreds{
		URL:      "https://secured",
		Username: "test-username",
		Password: "test-password",
	})
	require.NoError(t, err)

	tests := []struct {
		name    string
		repoURL string
		want    *v1alpha1.RepoCreds
	}{
		{
			name:    "TestUnknownRepo",
			repoURL: "https://unknown/repo",
			want:    nil,
		},
		{
			name:    "TestKnownRepo",
			repoURL: "https://known/repo",
			want:    nil,
		},
		{
			name:    "TestSecuredRepo",
			repoURL: "https://secured/repo",
			want:    &v1alpha1.RepoCreds{URL: "https://secured", Username: "test-username", Password: "test-password"},
		},
		{
			name:    "TestMissingRepo",
			repoURL: "https://missing/repo",
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := db.GetRepositoryCredentials(t.Context(), tt.repoURL)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateExistingRepository(t *testing.T) {
	clientset := getClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	_, err := db.CreateRepository(t.Context(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
	})
	require.NoError(t, err)

	_, err = db.CreateRepository(t.Context(), &v1alpha1.Repository{
		Repo:     "https://github.com/argoproj/argocd-example-apps",
		Username: "test-username",
		Password: "test-password",
	})
	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Convert(err).Code())
}

func TestGetRepository(t *testing.T) {
	clientset := getClientset(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "known-repo-secret",
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
			},
		},
		Data: map[string][]byte{
			"url": []byte("https://known/repo"),
		},
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "secured-repo-secret",
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
			},
		},
		Data: map[string][]byte{
			"url": []byte("https://secured/repo"),
		},
	}, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "secured-repo-creds-secret",
			Annotations: map[string]string{
				common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
			},
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds,
			},
		},
		Data: map[string][]byte{
			"url":      []byte("https://secured"),
			"username": []byte("test-username"),
			"password": []byte("test-password"),
		},
	})
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

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
			got, err := db.GetRepository(t.Context(), tt.repoURL, "")
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCreateClusterSuccessful(t *testing.T) {
	server := "https://mycluster"
	clientset := getClientset()
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	_, err := db.CreateCluster(t.Context(), &v1alpha1.Cluster{
		Server: server,
	})
	require.NoError(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), "cluster-mycluster-3274446258", metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, server, string(secret.Data["server"]))
	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
}

func TestDeleteClusterWithManagedSecret(t *testing.T) {
	clusterURL := "https://mycluster"
	clusterName := "cluster-mycluster-3274446258"

	clientset := getClientset(&corev1.Secret{
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

	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)
	err := db.DeleteCluster(t.Context(), clusterURL)
	require.NoError(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), clusterName, metav1.GetOptions{})
	require.Error(t, err)

	assert.True(t, errors.IsNotFound(err))
}

func TestDeleteClusterWithUnmanagedSecret(t *testing.T) {
	clusterURL := "https://mycluster"
	clusterName := "mycluster-443"

	clientset := getClientset(&corev1.Secret{
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

	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)
	err := db.DeleteCluster(t.Context(), clusterURL)
	require.NoError(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), clusterName, metav1.GetOptions{})
	require.NoError(t, err)

	assert.Empty(t, secret.Labels)
}

func TestFuzzyEquivalence(t *testing.T) {
	clientset := getClientset()
	ctx := t.Context()
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	repo, err := db.CreateRepository(ctx, &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argocd-example-apps",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)

	repo, err = db.CreateRepository(ctx, &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argocd-example-apps.git",
	})
	require.ErrorContains(t, err, "already exists")
	assert.Nil(t, repo)

	repo, err = db.CreateRepository(ctx, &v1alpha1.Repository{
		Repo: "https://github.com/argoproj/argocd-example-APPS",
	})
	require.ErrorContains(t, err, "already exists")
	assert.Nil(t, repo)

	repo, err = db.GetRepository(ctx, "https://github.com/argoproj/argocd-example-APPS", "")
	require.NoError(t, err)
	assert.Equal(t, "https://github.com/argoproj/argocd-example-apps", repo.Repo)
}

func TestGetApplicationControllerReplicas(t *testing.T) {
	clientset := getClientset()
	expectedReplicas := int32(2)
	t.Setenv(common.EnvControllerReplicas, "2")
	db := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)
	replicas := db.GetApplicationControllerReplicas()
	assert.Equal(t, int(expectedReplicas), replicas)

	expectedReplicas = int32(3)
	clientset = getClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ApplicationController,
			Namespace: testNamespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &expectedReplicas,
		},
	})
	t.Setenv(common.EnvControllerReplicas, "2")
	db = NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)
	replicas = db.GetApplicationControllerReplicas()
	assert.Equal(t, int(expectedReplicas), replicas)
}
