package db

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestSecretsRepositoryBackend_CreateRepository(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	testee := &secretsRepositoryBackend{db: &db{
		ns:            "test",
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(context.TODO(), clientset, "test"),
	}}

	input := &appsv1.Repository{
		Name:                  "ArgoCD",
		Repo:                  "git@github.com:argoproj/argo-cd.git",
		Username:              "someUsername",
		Password:              "somePassword",
		InsecureIgnoreHostKey: true,
		EnableLFS:             true,
	}

	output, err := testee.CreateRepository(context.TODO(), input)
	assert.NoError(t, err)
	assert.Same(t, input, output)

	secret, err := clientset.CoreV1().Secrets("test").Get(
		context.TODO(),
		RepoURLToSecretName("repoconfig", input.Repo),
		metav1.GetOptions{},
	)
	assert.NotNil(t, secret)
	assert.NoError(t, err)

	assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
	assert.Equal(t, common.LabelValueSecretTypeRepoConfig, secret.Labels[common.LabelKeySecretType])

	assert.Equal(t, input.Name, string(secret.Data["name"]))
	assert.Equal(t, input.Repo, string(secret.Data["repo"]))
	assert.Equal(t, input.Username, string(secret.Data["username"]))
	assert.Equal(t, input.Password, string(secret.Data["password"]))
	assert.Equal(t, strconv.FormatBool(input.InsecureIgnoreHostKey), string(secret.Data["insecureIgnoreHostKey"]))
	assert.Equal(t, strconv.FormatBool(input.EnableLFS), string(secret.Data["enableLfs"]))
}

func TestSecretsRepositoryBackend_GetRepository(t *testing.T) {
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test",
				Name:        RepoURLToSecretName("repoconfig", "git@github.com:argoproj/argo-cd.git"),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte("ArgoCD"),
				"repo":     []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte("UserManagedRepo"),
				"repo":     []byte("git@github.com:argoproj/argoproj.git"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
	}

	clientset := fake.NewSimpleClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            "test",
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(context.TODO(), clientset, "test"),
	}}

	repository, err := testee.GetRepository(context.TODO(), "git@github.com:argoproj/argo-cd.git")
	assert.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "ArgoCD", repository.Name)
	assert.Equal(t, "git@github.com:argoproj/argo-cd.git", repository.Repo)
	assert.Equal(t, "someUsername", repository.Username)
	assert.Equal(t, "somePassword", repository.Password)

	repository, err = testee.GetRepository(context.TODO(), "git@github.com:argoproj/argoproj.git")
	assert.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "UserManagedRepo", repository.Name)
	assert.Equal(t, "git@github.com:argoproj/argoproj.git", repository.Repo)
	assert.Equal(t, "someOtherUsername", repository.Username)
	assert.Equal(t, "someOtherPassword", repository.Password)
}

func TestSecretsRepositoryBackend_ListRepositories(t *testing.T) {
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test",
				Name:        RepoURLToSecretName("repoconfig", "git@github.com:argoproj/argo-cd.git"),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte("ArgoCD"),
				"repo":     []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte("UserManagedRepo"),
				"repo":     []byte("git@github.com:argoproj/argoproj.git"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
	}

	clientset := fake.NewSimpleClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            "test",
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(context.TODO(), clientset, "test"),
	}}

	repositories, err := testee.ListRepositories(context.TODO(), nil)
	assert.NoError(t, err)
	assert.Len(t, repositories, 2)

	for _, repository := range repositories {
		if repository.Name == "ArgoCD" {
			assert.Equal(t, "git@github.com:argoproj/argo-cd.git", repository.Repo)
			assert.Equal(t, "someUsername", repository.Username)
			assert.Equal(t, "somePassword", repository.Password)
		} else if repository.Name == "UserManagedRepo" {
			assert.Equal(t, "git@github.com:argoproj/argoproj.git", repository.Repo)
			assert.Equal(t, "someOtherUsername", repository.Username)
			assert.Equal(t, "someOtherPassword", repository.Password)
		} else {
			assert.Fail(t, "unexpected repository found in list")
		}
	}
}

func TestSecretsRepositoryBackend_UpdateRepository(t *testing.T) {
	managedRepository := &appsv1.Repository{
		Name:     "Managed",
		Repo:     "git@github.com:argoproj/argo-cd.git",
		Username: "someUsername",
		Password: "somePassword",
	}
	userProvidedRepository := &appsv1.Repository{
		Name:     "User Provided",
		Repo:     "git@github.com:argoproj/argoproj.git",
		Username: "someOtherUsername",
		Password: "someOtherPassword",
	}
	newRepository := &appsv1.Repository{
		Name:     "New",
		Repo:     "git@github.com:argoproj/argo-events.git",
		Username: "foo",
		Password: "bar",
	}

	managedSecretName := RepoURLToSecretName("repoconfig", managedRepository.Repo)
	newSecretName := RepoURLToSecretName("repoconfig", newRepository.Repo)
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test",
				Name:        managedSecretName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte(managedRepository.Name),
				"repo":     []byte(managedRepository.Repo),
				"username": []byte(managedRepository.Username),
				"password": []byte(managedRepository.Password),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte(userProvidedRepository.Name),
				"repo":     []byte(userProvidedRepository.Repo),
				"username": []byte(userProvidedRepository.Username),
				"password": []byte(userProvidedRepository.Password),
			},
		},
	}

	clientset := fake.NewSimpleClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            "test",
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(context.TODO(), clientset, "test"),
	}}

	managedRepository.Username = "newUsername"
	updateRepository, err := testee.UpdateRepository(context.TODO(), managedRepository)
	assert.NoError(t, err)
	assert.NotSame(t, managedRepository, updateRepository)
	assert.Equal(t, managedRepository.Username, updateRepository.Username)

	secret, err := clientset.CoreV1().Secrets("test").Get(context.TODO(), managedSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newUsername", string(secret.Data["username"]))

	userProvidedRepository.Username = "newOtherUsername"
	updateRepository, err = testee.UpdateRepository(context.TODO(), userProvidedRepository)
	assert.NoError(t, err)
	assert.NotSame(t, userProvidedRepository, updateRepository)
	assert.Equal(t, userProvidedRepository.Username, updateRepository.Username)

	secret, err = clientset.CoreV1().Secrets("test").Get(context.TODO(), "user-managed", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newOtherUsername", string(secret.Data["username"]))

	updateRepository, err = testee.UpdateRepository(context.TODO(), newRepository)
	assert.NoError(t, err)
	assert.Same(t, newRepository, updateRepository)

	secret, err = clientset.CoreV1().Secrets("test").Get(context.TODO(), newSecretName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "foo", string(secret.Data["username"]))
}

func TestSecretsRepositoryBackend_DeleteRepository(t *testing.T) {
	managedSecretName := RepoURLToSecretName("repoconfig", "git@github.com:argoproj/argo-cd.git")
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   "test",
				Name:        managedSecretName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte("ArgoCD"),
				"repo":     []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoConfig},
			},
			Data: map[string][]byte{
				"name":     []byte("UserManagedRepo"),
				"repo":     []byte("git@github.com:argoproj/argoproj.git"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
	}

	clientset := fake.NewSimpleClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            "test",
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(context.TODO(), clientset, "test"),
	}}

	err := testee.DeleteRepository(context.TODO(), "git@github.com:argoproj/argo-cd.git")
	assert.NoError(t, err)

	_, err = clientset.CoreV1().Secrets("test").Get(context.TODO(), managedSecretName, metav1.GetOptions{})
	assert.Error(t, err)

	err = testee.DeleteRepository(context.TODO(), "git@github.com:argoproj/argoproj.git")
	assert.NoError(t, err)

	secret, err := clientset.CoreV1().Secrets("test").Get(context.TODO(), "user-managed", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Empty(t, secret.Labels[common.LabelValueSecretTypeRepoConfig])
}
