package db

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/v3/common"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
)

func TestSecretsRepositoryBackend_CreateRepository(t *testing.T) {
	t.Parallel()

	type fixture struct {
		clientSet   *fake.Clientset
		repoBackend *secretsRepositoryBackend
	}
	repo := &appsv1.Repository{
		Name:                  "ArgoCD",
		Repo:                  "git@github.com:argoproj/argo-cd.git",
		Username:              "someUsername",
		Password:              "somePassword",
		InsecureIgnoreHostKey: false,
		EnableLFS:             true,
	}
	setup := func() *fixture {
		clientset := getClientset()
		settingsMgr := settings.NewSettingsManager(t.Context(), clientset, testNamespace)
		repoBackend := &secretsRepositoryBackend{db: &db{
			ns:            testNamespace,
			kubeclientset: clientset,
			settingsMgr:   settingsMgr,
		}}
		return &fixture{
			clientSet:   clientset,
			repoBackend: repoBackend,
		}
	}
	t.Run("will create repository successfully", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()

		// when
		output, err := f.repoBackend.CreateRepository(t.Context(), repo)

		// then
		require.NoError(t, err)
		assert.Same(t, repo, output)

		secret, err := f.clientSet.CoreV1().Secrets(testNamespace).Get(
			t.Context(),
			RepoURLToSecretName(repoSecretPrefix, repo.Repo, ""),
			metav1.GetOptions{},
		)
		assert.NotNil(t, secret)
		require.NoError(t, err)

		assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
		assert.Equal(t, common.LabelValueSecretTypeRepository, secret.Labels[common.LabelKeySecretType])

		assert.Equal(t, repo.Name, string(secret.Data["name"]))
		assert.Equal(t, repo.Repo, string(secret.Data["url"]))
		assert.Equal(t, repo.Username, string(secret.Data["username"]))
		assert.Equal(t, repo.Password, string(secret.Data["password"]))
		assert.Empty(t, string(secret.Data["insecureIgnoreHostKey"]))
		assert.Equal(t, strconv.FormatBool(repo.EnableLFS), string(secret.Data["enableLfs"]))
	})
	t.Run("will return proper error if secret does not have expected label", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()

		// when
		_, err := f.repoBackend.CreateRepository(t.Context(), repo)

		// then
		require.NoError(t, err)

		// given - remove the label from the secret
		secret, err := f.clientSet.CoreV1().Secrets(testNamespace).Get(
			t.Context(),
			RepoURLToSecretName(repoSecretPrefix, repo.Repo, ""),
			metav1.GetOptions{},
		)
		require.NoError(t, err)
		delete(secret.Labels, common.LabelKeySecretType)
		_, err = f.clientSet.CoreV1().Secrets(testNamespace).Update(t.Context(), secret, metav1.UpdateOptions{})
		require.NoError(t, err)

		// Resync informers to ensure the cache reflects the updated secret
		err = f.repoBackend.db.settingsMgr.ResyncInformers()
		require.NoError(t, err)

		// when - try to create the same repository again
		output, err := f.repoBackend.CreateRepository(t.Context(), repo)

		// then
		require.Error(t, err)
		assert.Nil(t, output)
		status, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, status.Code(), "got unexpected error: %v", err)
	})
	t.Run("will return proper error if secret already exists and does have the proper label", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()

		// when
		_, err := f.repoBackend.CreateRepository(t.Context(), repo)

		// then
		require.NoError(t, err)

		// when - try to create the same repository again
		output, err := f.repoBackend.CreateRepository(t.Context(), repo)

		// then
		require.Error(t, err)
		assert.Nil(t, output)
		status, ok := status.FromError(err)
		assert.True(t, ok)
		assert.Equal(t, codes.AlreadyExists, status.Code(), "got unexpected error: %v", err)
	})
}

func TestSecretsRepositoryBackend_GetRepository(t *testing.T) {
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", ""),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("ArgoCD"),
				"url":      []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", "testProject"),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("Scoped ArgoCD"),
				"url":      []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someScopedUsername"),
				"password": []byte("someScopedPassword"),
				"project":  []byte("testProject"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("UserManagedRepo"),
				"url":      []byte("git@github.com:argoproj/argoproj.git"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "other-user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("Scoped UserManagedRepo"),
				"url":      []byte("git@github.com:argoproj/argoproj.git"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
				"project":  []byte("testProject"),
			},
		},
	}

	clientset := getClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	repository, err := testee.GetRepository(t.Context(), "git@github.com:argoproj/argo-cd.git", "")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "ArgoCD", repository.Name)
	assert.Equal(t, "git@github.com:argoproj/argo-cd.git", repository.Repo)
	assert.Equal(t, "someUsername", repository.Username)
	assert.Equal(t, "somePassword", repository.Password)

	repository, err = testee.GetRepository(t.Context(), "git@github.com:argoproj/argoproj.git", "")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "UserManagedRepo", repository.Name)
	assert.Equal(t, "git@github.com:argoproj/argoproj.git", repository.Repo)
	assert.Equal(t, "someOtherUsername", repository.Username)
	assert.Equal(t, "someOtherPassword", repository.Password)

	repository, err = testee.GetRepository(t.Context(), "git@github.com:argoproj/argo-cd.git", "testProject")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "Scoped ArgoCD", repository.Name)
	assert.Equal(t, "git@github.com:argoproj/argo-cd.git", repository.Repo)
	assert.Equal(t, "someScopedUsername", repository.Username)
	assert.Equal(t, "someScopedPassword", repository.Password)
	assert.Equal(t, "testProject", repository.Project)

	repository, err = testee.GetRepository(t.Context(), "git@github.com:argoproj/argoproj.git", "testProject")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "Scoped UserManagedRepo", repository.Name)
	assert.Equal(t, "git@github.com:argoproj/argoproj.git", repository.Repo)
	assert.Equal(t, "someOtherUsername", repository.Username)
	assert.Equal(t, "someOtherPassword", repository.Password)
	assert.Equal(t, "testProject", repository.Project)
}

func TestSecretsRepositoryBackend_ListRepositories(t *testing.T) {
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", ""),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("ArgoCD"),
				"url":      []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("UserManagedRepo"),
				"url":      []byte("git@github.com:argoproj/argoproj.git"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
	}

	clientset := getClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	repositories, err := testee.ListRepositories(t.Context(), nil)
	require.NoError(t, err)
	assert.Len(t, repositories, 2)

	for _, repository := range repositories {
		switch repository.Name {
		case "ArgoCD":
			assert.Equal(t, "git@github.com:argoproj/argo-cd.git", repository.Repo)
			assert.Equal(t, "someUsername", repository.Username)
			assert.Equal(t, "somePassword", repository.Password)
		case "UserManagedRepo":
			assert.Equal(t, "git@github.com:argoproj/argoproj.git", repository.Repo)
			assert.Equal(t, "someOtherUsername", repository.Username)
			assert.Equal(t, "someOtherPassword", repository.Password)
		default:
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
	managedProjectRepository := &appsv1.Repository{
		Name:     "Managed",
		Repo:     "git@github.com:argoproj/argo-cd.git",
		Username: "someUsername",
		Password: "somePassword",
		Project:  "someProject",
	}
	userProvidedRepository := &appsv1.Repository{
		Name:     "User Provided",
		Repo:     "git@github.com:argoproj/argoproj.git",
		Username: "someOtherUsername",
		Password: "someOtherPassword",
	}
	userProvidedProjectRepository := &appsv1.Repository{
		Name:     "User Provided",
		Repo:     "git@github.com:argoproj/argoproj.git",
		Username: "someOtherUsername",
		Password: "someOtherPassword",
		Project:  "someProject",
	}
	newRepository := &appsv1.Repository{
		Name:     "New",
		Repo:     "git@github.com:argoproj/argo-events.git",
		Username: "foo",
		Password: "bar",
	}

	managedSecretName := RepoURLToSecretName(repoSecretPrefix, managedRepository.Repo, "")
	managedProjectSecretName := RepoURLToSecretName(repoSecretPrefix, managedProjectRepository.Repo, "someProject")
	newSecretName := RepoURLToSecretName(repoSecretPrefix, newRepository.Repo, "")
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        managedSecretName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte(managedRepository.Name),
				"url":      []byte(managedRepository.Repo),
				"username": []byte(managedRepository.Username),
				"password": []byte(managedRepository.Password),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        managedProjectSecretName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte(managedProjectRepository.Name),
				"url":      []byte(managedProjectRepository.Repo),
				"username": []byte(managedProjectRepository.Username),
				"password": []byte(managedProjectRepository.Password),
				"project":  []byte(managedProjectRepository.Project),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte(userProvidedRepository.Name),
				"url":      []byte(userProvidedRepository.Repo),
				"username": []byte(userProvidedRepository.Username),
				"password": []byte(userProvidedRepository.Password),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed-scoped",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte(userProvidedProjectRepository.Name),
				"url":      []byte(userProvidedProjectRepository.Repo),
				"username": []byte(userProvidedProjectRepository.Username),
				"password": []byte(userProvidedProjectRepository.Password),
				"project":  []byte(userProvidedProjectRepository.Project),
			},
		},
	}

	clientset := getClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	managedRepository.Username = "newUsername"
	updateRepository, err := testee.UpdateRepository(t.Context(), managedRepository)
	require.NoError(t, err)
	assert.Same(t, managedRepository, updateRepository)
	assert.Equal(t, managedRepository.Username, updateRepository.Username)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), managedSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newUsername", string(secret.Data["username"]))

	userProvidedRepository.Username = "newOtherUsername"
	updateRepository, err = testee.UpdateRepository(t.Context(), userProvidedRepository)
	require.NoError(t, err)
	assert.Same(t, userProvidedRepository, updateRepository)
	assert.Equal(t, userProvidedRepository.Username, updateRepository.Username)

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), "user-managed", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newOtherUsername", string(secret.Data["username"]))

	updateRepository, err = testee.UpdateRepository(t.Context(), newRepository)
	require.NoError(t, err)
	assert.Same(t, newRepository, updateRepository)

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), newSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "foo", string(secret.Data["username"]))

	managedProjectRepository.Username = "newUsername"
	updateRepository, err = testee.UpdateRepository(t.Context(), managedProjectRepository)
	require.NoError(t, err)
	assert.Same(t, managedProjectRepository, updateRepository)
	assert.Equal(t, managedProjectRepository.Username, updateRepository.Username)

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), managedProjectSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newUsername", string(secret.Data["username"]))

	userProvidedProjectRepository.Username = "newUsernameScoped"
	updateRepository, err = testee.UpdateRepository(t.Context(), userProvidedProjectRepository)
	require.NoError(t, err)
	assert.Same(t, userProvidedProjectRepository, updateRepository)
	assert.Equal(t, userProvidedProjectRepository.Username, updateRepository.Username)

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), "user-managed-scoped", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newUsernameScoped", string(secret.Data["username"]))
}

func TestSecretsRepositoryBackend_DeleteRepository(t *testing.T) {
	managedSecretName := RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", "")
	managedScopedSecretName := RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", "someProject")
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        managedSecretName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("ArgoCD"),
				"url":      []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        managedScopedSecretName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("ArgoCD"),
				"url":      []byte("git@github.com:argoproj/argo-cd.git"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
				"project":  []byte("someProject"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepository},
			},
			Data: map[string][]byte{
				"name":     []byte("UserManagedRepo"),
				"url":      []byte("git@github.com:argoproj/argoproj.git"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
	}

	clientset := getClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	err := testee.DeleteRepository(t.Context(), "git@github.com:argoproj/argo-cd.git", "")
	require.NoError(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), managedSecretName, metav1.GetOptions{})
	require.Error(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), managedScopedSecretName, metav1.GetOptions{})
	require.NoError(t, err)

	err = testee.DeleteRepository(t.Context(), "git@github.com:argoproj/argo-cd.git", "someProject")
	require.NoError(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), managedScopedSecretName, metav1.GetOptions{})
	require.Error(t, err)

	err = testee.DeleteRepository(t.Context(), "git@github.com:argoproj/argoproj.git", "")
	require.NoError(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), "user-managed", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Empty(t, secret.Labels[common.LabelValueSecretTypeRepository])
}

func TestSecretsRepositoryBackend_CreateRepoCreds(t *testing.T) {
	clientset := getClientset()
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	testCases := []struct {
		name      string
		repoCreds appsv1.RepoCreds
		// Note: URL needs to be a different one for every testCase
		// otherwise we would need to use the DeleteRepoCreds method to clean up the secret after each test
		// which results in an unwanted dependency in a unit test
	}{
		{
			name: "minimal_https_fields",
			repoCreds: appsv1.RepoCreds{
				URL:       "git@github.com:argoproj",
				Username:  "someUsername",
				Password:  "somePassword",
				EnableOCI: true,
			},
		},
		{
			name: "with_proxy",
			repoCreds: appsv1.RepoCreds{
				URL:      "git@github.com:kubernetes",
				Username: "anotherUsername",
				Password: "anotherPassword",
				Proxy:    "https://proxy.argoproj.io:3128",
			},
		},
		{
			name: "with_noProxy",
			repoCreds: appsv1.RepoCreds{
				URL:      "git@github.com:proxy",
				Username: "anotherUsername",
				Password: "anotherPassword",
				NoProxy:  ".example.com,127.0.0.1",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			output, err := testee.CreateRepoCreds(t.Context(), &testCase.repoCreds)
			require.NoError(t, err)
			assert.Same(t, &testCase.repoCreds, output)

			secret, err := clientset.CoreV1().Secrets(testNamespace).Get(
				t.Context(),
				RepoURLToSecretName(credSecretPrefix, testCase.repoCreds.URL, ""),
				metav1.GetOptions{},
			)
			assert.NotNil(t, secret)
			require.NoError(t, err)

			assert.Equal(t, common.AnnotationValueManagedByArgoCD, secret.Annotations[common.AnnotationKeyManagedBy])
			assert.Equal(t, common.LabelValueSecretTypeRepoCreds, secret.Labels[common.LabelKeySecretType])

			// check every possible field of the secret if it has the same (default) value as the repoCred struct
			// non-string fields must be parsed so that their default value matches the one of the corresponding type
			assert.Equal(t, testCase.repoCreds.URL, string(secret.Data["url"]))
			assert.Equal(t, testCase.repoCreds.Username, string(secret.Data["username"]))
			assert.Equal(t, testCase.repoCreds.Password, string(secret.Data["password"]))
			if enableOCI, err := strconv.ParseBool(string(secret.Data["githubAppPrivateKey"])); err == nil {
				assert.Equal(t, strconv.FormatBool(testCase.repoCreds.EnableOCI), enableOCI)
			}
			assert.Equal(t, testCase.repoCreds.SSHPrivateKey, string(secret.Data["sshPrivateKey"]))
			assert.Equal(t, testCase.repoCreds.TLSClientCertData, string(secret.Data["tlsClientCertData"]))
			assert.Equal(t, testCase.repoCreds.TLSClientCertKey, string(secret.Data["tlsClientCertKey"]))
			assert.Equal(t, testCase.repoCreds.Type, string(secret.Data["type"]))
			assert.Equal(t, testCase.repoCreds.GithubAppPrivateKey, string(secret.Data["githubAppPrivateKey"]))
			if githubAppPrivateKey, err := strconv.ParseInt(string(secret.Data["githubAppPrivateKey"]), 10, 64); err == nil {
				assert.Equal(t, testCase.repoCreds.GithubAppId, githubAppPrivateKey)
			}
			if githubAppID, err := strconv.ParseInt(string(secret.Data["githubAppId"]), 10, 64); err == nil {
				assert.Equal(t, testCase.repoCreds.GithubAppInstallationId, githubAppID)
			}
			assert.Equal(t, testCase.repoCreds.GitHubAppEnterpriseBaseURL, string(secret.Data["githubAppEnterpriseUrl"]))
			assert.Equal(t, testCase.repoCreds.Proxy, string(secret.Data["proxy"]))
			assert.Equal(t, testCase.repoCreds.NoProxy, string(secret.Data["noProxy"]))
		})
	}
}

func TestSecretsRepositoryBackend_GetRepoCreds(t *testing.T) {
	repoCredSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj", ""),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@github.com:argoproj"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@gitlab.com"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "proxy-repo",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@gitlab.com"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
				"proxy":    []byte("https://proxy.argoproj.io:3128"),
				"noProxy":  []byte(".example.com,127.0.0.1"),
			},
		},
	}

	clientset := getClientset(repoCredSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	repoCred, err := testee.GetRepoCreds(t.Context(), "git@github.com:argoproj")
	require.NoError(t, err)
	require.NotNil(t, repoCred)
	assert.Equal(t, "git@github.com:argoproj", repoCred.URL)
	assert.Equal(t, "someUsername", repoCred.Username)
	assert.Equal(t, "somePassword", repoCred.Password)

	repoCred, err = testee.GetRepoCreds(t.Context(), "git@gitlab.com")
	require.NoError(t, err)
	assert.NotNil(t, repoCred)
	assert.Equal(t, "git@gitlab.com", repoCred.URL)
	assert.Equal(t, "someOtherUsername", repoCred.Username)
	assert.Equal(t, "someOtherPassword", repoCred.Password)
	if repoCred.Proxy != "" {
		assert.Equal(t, "https://proxy.argoproj.io:3128", repoCred.Proxy)
	}
	if repoCred.NoProxy != "" {
		assert.Equal(t, ".example.com,127.0.0.1", repoCred.NoProxy)
	}
}

func TestSecretsRepositoryBackend_ListRepoCreds(t *testing.T) {
	repoCredSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj", ""),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@github.com:argoproj"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@gitlab.com"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
	}

	clientset := getClientset(repoCredSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	repoCreds, err := testee.ListRepoCreds(t.Context())
	require.NoError(t, err)
	assert.Len(t, repoCreds, 2)
	assert.Contains(t, repoCreds, "git@github.com:argoproj")
	assert.Contains(t, repoCreds, "git@gitlab.com")
}

func TestSecretsRepositoryBackend_UpdateRepoCreds(t *testing.T) {
	managedCreds := &appsv1.RepoCreds{
		URL:      "git@github.com:argoproj",
		Username: "someUsername",
		Password: "somePassword",
	}
	userProvidedCreds := &appsv1.RepoCreds{
		URL:      "git@gitlab.com",
		Username: "someOtherUsername",
		Password: "someOtherPassword",
	}
	newCreds := &appsv1.RepoCreds{
		URL:      "git@github.com:foobar",
		Username: "foo",
		Password: "bar",
	}

	managedCredsName := RepoURLToSecretName(credSecretPrefix, managedCreds.URL, "")
	newCredsName := RepoURLToSecretName(credSecretPrefix, newCreds.URL, "")
	repoCredSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        managedCredsName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte(managedCreds.URL),
				"username": []byte(managedCreds.Username),
				"password": []byte(managedCreds.Password),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte(userProvidedCreds.URL),
				"username": []byte(userProvidedCreds.Username),
				"password": []byte(userProvidedCreds.Password),
			},
		},
	}

	clientset := getClientset(repoCredSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	managedCreds.Username = "newUsername"
	updateRepoCreds, err := testee.UpdateRepoCreds(t.Context(), managedCreds)
	require.NoError(t, err)
	assert.NotSame(t, managedCreds, updateRepoCreds)
	assert.Equal(t, managedCreds.Username, updateRepoCreds.Username)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), managedCredsName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newUsername", string(secret.Data["username"]))

	userProvidedCreds.Username = "newOtherUsername"
	updateRepoCreds, err = testee.UpdateRepoCreds(t.Context(), userProvidedCreds)
	require.NoError(t, err)
	assert.NotSame(t, userProvidedCreds, updateRepoCreds)
	assert.Equal(t, userProvidedCreds.Username, updateRepoCreds.Username)

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), "user-managed", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "newOtherUsername", string(secret.Data["username"]))

	updateRepoCreds, err = testee.UpdateRepoCreds(t.Context(), newCreds)
	require.NoError(t, err)
	assert.Same(t, newCreds, updateRepoCreds)

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), newCredsName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "foo", string(secret.Data["username"]))
}

func TestSecretsRepositoryBackend_DeleteRepoCreds(t *testing.T) {
	managedSecretName := RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", "")
	repoSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        managedSecretName,
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@github.com:argoproj"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: testNamespace,
				Name:      "user-managed",
				Labels:    map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@gitlab.com"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
			},
		},
	}

	clientset := getClientset(repoSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	err := testee.DeleteRepoCreds(t.Context(), "git@github.com:argoproj")
	require.NoError(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), managedSecretName, metav1.GetOptions{})
	require.Error(t, err)

	err = testee.DeleteRepoCreds(t.Context(), "git@gitlab.com")
	require.NoError(t, err)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), "user-managed", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Empty(t, secret.Labels[common.LabelValueSecretTypeRepoCreds])
}

func TestSecretsRepositoryBackend_GetAllHelmRepoCreds(t *testing.T) {
	repoCredSecrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj", ""),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@github.com:argoproj"),
				"username": []byte("someUsername"),
				"password": []byte("somePassword"),
				"type":     []byte("helm"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   testNamespace,
				Name:        RepoURLToSecretName(repoSecretPrefix, "git@gitlab.com", ""),
				Annotations: map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD},
				Labels:      map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds},
			},
			Data: map[string][]byte{
				"url":      []byte("git@gitlab.com"),
				"username": []byte("someOtherUsername"),
				"password": []byte("someOtherPassword"),
				"type":     []byte("git"),
			},
		},
	}

	clientset := getClientset(repoCredSecrets...)
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}

	repoCreds, err := testee.GetAllHelmRepoCreds(t.Context())
	require.NoError(t, err)
	assert.Len(t, repoCreds, 1)
}

func TestRepoCredsToSecret(t *testing.T) {
	clientset := getClientset()
	testee := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
	}}
	s := &corev1.Secret{}
	creds := &appsv1.RepoCreds{
		URL:                        "URL",
		Username:                   "Username",
		Password:                   "Password",
		SSHPrivateKey:              "SSHPrivateKey",
		EnableOCI:                  true,
		TLSClientCertData:          "TLSClientCertData",
		TLSClientCertKey:           "TLSClientCertKey",
		Type:                       "Type",
		GithubAppPrivateKey:        "GithubAppPrivateKey",
		GithubAppId:                123,
		GithubAppInstallationId:    456,
		GitHubAppEnterpriseBaseURL: "GitHubAppEnterpriseBaseURL",
	}
	s = testee.repoCredsToSecret(creds, s)
	assert.Equal(t, []byte(creds.URL), s.Data["url"])
	assert.Equal(t, []byte(creds.Username), s.Data["username"])
	assert.Equal(t, []byte(creds.Password), s.Data["password"])
	assert.Equal(t, []byte(creds.SSHPrivateKey), s.Data["sshPrivateKey"])
	assert.Equal(t, []byte(strconv.FormatBool(creds.EnableOCI)), s.Data["enableOCI"])
	assert.Equal(t, []byte(creds.TLSClientCertData), s.Data["tlsClientCertData"])
	assert.Equal(t, []byte(creds.TLSClientCertKey), s.Data["tlsClientCertKey"])
	assert.Equal(t, []byte(creds.Type), s.Data["type"])
	assert.Equal(t, []byte(creds.GithubAppPrivateKey), s.Data["githubAppPrivateKey"])
	assert.Equal(t, []byte(strconv.FormatInt(creds.GithubAppId, 10)), s.Data["githubAppID"])
	assert.Equal(t, []byte(strconv.FormatInt(creds.GithubAppInstallationId, 10)), s.Data["githubAppInstallationID"])
	assert.Equal(t, []byte(creds.GitHubAppEnterpriseBaseURL), s.Data["githubAppEnterpriseBaseUrl"])
	assert.Equal(t, map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD}, s.Annotations)
	assert.Equal(t, map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds}, s.Labels)
}

func TestRepoWriteCredsToSecret(t *testing.T) {
	clientset := getClientset()
	testee := &secretsRepositoryBackend{
		db: &db{
			ns:            testNamespace,
			kubeclientset: clientset,
			settingsMgr:   settings.NewSettingsManager(t.Context(), clientset, testNamespace),
		},
		writeCreds: true,
	}
	s := &corev1.Secret{}
	creds := &appsv1.RepoCreds{
		URL:                        "URL",
		Username:                   "Username",
		Password:                   "Password",
		SSHPrivateKey:              "SSHPrivateKey",
		EnableOCI:                  true,
		TLSClientCertData:          "TLSClientCertData",
		TLSClientCertKey:           "TLSClientCertKey",
		Type:                       "Type",
		GithubAppPrivateKey:        "GithubAppPrivateKey",
		GithubAppId:                123,
		GithubAppInstallationId:    456,
		GitHubAppEnterpriseBaseURL: "GitHubAppEnterpriseBaseURL",
	}
	s = testee.repoCredsToSecret(creds, s)
	assert.Equal(t, []byte(creds.URL), s.Data["url"])
	assert.Equal(t, []byte(creds.Username), s.Data["username"])
	assert.Equal(t, []byte(creds.Password), s.Data["password"])
	assert.Equal(t, []byte(creds.SSHPrivateKey), s.Data["sshPrivateKey"])
	assert.Equal(t, []byte(strconv.FormatBool(creds.EnableOCI)), s.Data["enableOCI"])
	assert.Equal(t, []byte(creds.TLSClientCertData), s.Data["tlsClientCertData"])
	assert.Equal(t, []byte(creds.TLSClientCertKey), s.Data["tlsClientCertKey"])
	assert.Equal(t, []byte(creds.Type), s.Data["type"])
	assert.Equal(t, []byte(creds.GithubAppPrivateKey), s.Data["githubAppPrivateKey"])
	assert.Equal(t, []byte(strconv.FormatInt(creds.GithubAppId, 10)), s.Data["githubAppID"])
	assert.Equal(t, []byte(strconv.FormatInt(creds.GithubAppInstallationId, 10)), s.Data["githubAppInstallationID"])
	assert.Equal(t, []byte(creds.GitHubAppEnterpriseBaseURL), s.Data["githubAppEnterpriseBaseUrl"])
	assert.Equal(t, map[string]string{common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD}, s.Annotations)
	assert.Equal(t, map[string]string{common.LabelKeySecretType: common.LabelValueSecretTypeRepoCredsWrite}, s.Labels)
}

func TestRaceConditionInRepoCredsOperations(t *testing.T) {
	// Create a single shared secret that will be accessed concurrently
	sharedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", ""),
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds,
			},
		},
		Data: map[string][]byte{
			"url":      []byte("git@github.com:argoproj/argo-cd.git"),
			"username": []byte("test-user"),
			"password": []byte("test-pass"),
		},
	}

	// Create test credentials that we'll use for conversion
	repoCreds := &appsv1.RepoCreds{
		URL:      "git@github.com:argoproj/argo-cd.git",
		Username: "test-user",
		Password: "test-pass",
	}

	backend := &secretsRepositoryBackend{}

	var wg sync.WaitGroup
	concurrentOps := 50
	errChan := make(chan error, concurrentOps*2) // Channel to collect errors

	// Launch goroutines that perform concurrent operations
	for range concurrentOps {
		wg.Add(2)

		// One goroutine converts from RepoCreds to Secret
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic in repoCredsToSecret: %v", r)
				}
			}()
			_ = backend.repoCredsToSecret(repoCreds, sharedSecret)
		}()

		// Another goroutine converts from Secret to RepoCreds
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic in secretToRepoCred: %v", r)
				}
			}()
			creds, err := backend.secretToRepoCred(sharedSecret)
			if err != nil {
				errChan <- fmt.Errorf("error in secretToRepoCred: %w", err)
				return
			}
			// Verify data integrity
			if creds.URL != repoCreds.URL || creds.Username != repoCreds.Username || creds.Password != repoCreds.Password {
				errChan <- fmt.Errorf("data mismatch in conversion: expected %v, got %v", repoCreds, creds)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for any errors that occurred during concurrent operations
	for err := range errChan {
		t.Errorf("concurrent operation error: %v", err)
	}

	// Verify final state
	finalCreds, err := backend.secretToRepoCred(sharedSecret)
	require.NoError(t, err)
	assert.Equal(t, repoCreds.URL, finalCreds.URL)
	assert.Equal(t, repoCreds.Username, finalCreds.Username)
	assert.Equal(t, repoCreds.Password, finalCreds.Password)
}

func TestRaceConditionInRepositoryOperations(t *testing.T) {
	// Create a single shared secret that will be accessed concurrently
	sharedSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      RepoURLToSecretName(repoSecretPrefix, "git@github.com:argoproj/argo-cd.git", ""),
			Namespace: testNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
			},
		},
		Data: map[string][]byte{
			"url":      []byte("git@github.com:argoproj/argo-cd.git"),
			"name":     []byte("test-repo"),
			"username": []byte("test-user"),
			"password": []byte("test-pass"),
		},
	}

	// Create test repository that we'll use for conversion
	repo := &appsv1.Repository{
		Name:     "test-repo",
		Repo:     "git@github.com:argoproj/argo-cd.git",
		Username: "test-user",
		Password: "test-pass",
	}

	backend := &secretsRepositoryBackend{}

	var wg sync.WaitGroup
	concurrentOps := 50
	errChan := make(chan error, concurrentOps*2) // Channel to collect errors

	// Launch goroutines that perform concurrent operations
	for range concurrentOps {
		wg.Add(2)

		// One goroutine converts from Repository to Secret
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic in repositoryToSecret: %v", r)
				}
			}()
			_ = backend.repositoryToSecret(repo, sharedSecret)
		}()

		// Another goroutine converts from Secret to Repository
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errChan <- fmt.Errorf("panic in secretToRepository: %v", r)
				}
			}()
			repository, err := secretToRepository(sharedSecret)
			if err != nil {
				errChan <- fmt.Errorf("error in secretToRepository: %w", err)
				return
			}
			// Verify data integrity
			if repository.Name != repo.Name || repository.Repo != repo.Repo ||
				repository.Username != repo.Username || repository.Password != repo.Password {
				errChan <- fmt.Errorf("data mismatch in conversion: expected %v, got %v", repo, repository)
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for any errors that occurred during concurrent operations
	for err := range errChan {
		t.Errorf("concurrent operation error: %v", err)
	}

	// Verify final state
	finalRepo, err := secretToRepository(sharedSecret)
	require.NoError(t, err)
	assert.Equal(t, repo.Name, finalRepo.Name)
	assert.Equal(t, repo.Repo, finalRepo.Repo)
	assert.Equal(t, repo.Username, finalRepo.Username)
	assert.Equal(t, repo.Password, finalRepo.Password)
}

func TestCreateReadAndWriteSecretForSameURL(t *testing.T) {
	clientset := getClientset()
	settingsMgr := settings.NewSettingsManager(t.Context(), clientset, testNamespace)

	repo := &appsv1.Repository{
		Name:     "TestRepo",
		Repo:     "git@github.com:argoproj/argo-cd.git",
		Username: "user",
		Password: "pass",
	}

	// Create read secret
	readBackend := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsMgr,
	}, writeCreds: false}
	_, err := readBackend.CreateRepository(t.Context(), repo)
	require.NoError(t, err)

	// Create write secret
	writeBackend := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsMgr,
	}, writeCreds: true}
	_, err = writeBackend.CreateRepository(t.Context(), repo)
	require.NoError(t, err)

	// Assert both secrets exist
	readSecretName := RepoURLToSecretName(repoSecretPrefix, repo.Repo, repo.Project)
	writeSecretName := RepoURLToSecretName(repoWriteSecretPrefix, repo.Repo, repo.Project)

	readSecret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), readSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.LabelValueSecretTypeRepository, readSecret.Labels[common.LabelKeySecretType])

	writeSecret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), writeSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.LabelValueSecretTypeRepositoryWrite, writeSecret.Labels[common.LabelKeySecretType])
}

func TestCreateReadAndWriteRepoCredsSecretForSameURL(t *testing.T) {
	clientset := getClientset()
	settingsMgr := settings.NewSettingsManager(t.Context(), clientset, testNamespace)

	creds := &appsv1.RepoCreds{
		URL:      "git@github.com:argoproj/argo-cd.git",
		Username: "user",
		Password: "pass",
	}

	// Create read creds secret
	readBackend := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsMgr,
	}, writeCreds: false}
	_, err := readBackend.CreateRepoCreds(t.Context(), creds)
	require.NoError(t, err)

	// Create write creds secret
	writeBackend := &secretsRepositoryBackend{db: &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsMgr,
	}, writeCreds: true}
	_, err = writeBackend.CreateRepoCreds(t.Context(), creds)
	require.NoError(t, err)

	// Assert both secrets exist
	readSecretName := RepoURLToSecretName(credSecretPrefix, creds.URL, "")
	writeSecretName := RepoURLToSecretName(credWriteSecretPrefix, creds.URL, "")

	readSecret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), readSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.LabelValueSecretTypeRepoCreds, readSecret.Labels[common.LabelKeySecretType])

	writeSecret, err := clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), writeSecretName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, common.LabelValueSecretTypeRepoCredsWrite, writeSecret.Labels[common.LabelKeySecretType])
}
