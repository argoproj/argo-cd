package db

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	repoArgoProj = `
- name: OtherRepo
  url: git@github.com:argoproj/argoproj.git
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: managed-secret
    key: password
  type: git`
)

var repoArgoCD = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: testNamespace,
		Name:      "some-repo-secret",
		Annotations: map[string]string{
			common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
		},
		Labels: map[string]string{
			common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
		},
	},
	Data: map[string][]byte{
		"name":     []byte("SomeRepo"),
		"url":      []byte("git@github.com:argoproj/argo-cd.git"),
		"username": []byte("someUsername"),
		"password": []byte("somePassword"),
		"type":     []byte("git"),
	},
}

func TestDb_CreateRepository(t *testing.T) {
	clientset := getClientset(map[string]string{})
	settingsManager := settings.NewSettingsManager(context.TODO(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	input := &appsv1.Repository{
		Name:     "TestRepo",
		Repo:     "git@github.com:argoproj/argo-cd.git",
		Username: "someUsername",
		Password: "somePassword",
	}

	// The repository was indeed created successfully
	output, err := testee.CreateRepository(context.TODO(), input)
	require.NoError(t, err)
	assert.Same(t, input, output)

	// New repositories should not be stored in the settings anymore
	settingRepositories, err := settingsManager.GetRepositories()
	require.NoError(t, err)
	assert.Empty(t, settingRepositories)

	// New repositories should be now stored as secrets
	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(
		context.TODO(),
		RepoURLToSecretName(repoSecretPrefix, input.Repo, ""),
		metav1.GetOptions{},
	)
	assert.NotNil(t, secret)
	require.NoError(t, err)
}

func TestDb_GetRepository(t *testing.T) {
	clientset := getClientset(map[string]string{"repositories": repoArgoProj}, newManagedSecret(), repoArgoCD)
	settingsManager := settings.NewSettingsManager(context.TODO(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	repository, err := testee.GetRepository(context.TODO(), "git@github.com:argoproj/argoproj.git", "")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "OtherRepo", repository.Name)

	repository, err = testee.GetRepository(context.TODO(), "git@github.com:argoproj/argo-cd.git", "")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "SomeRepo", repository.Name)

	repository, err = testee.GetRepository(context.TODO(), "git@github.com:argoproj/not-existing.git", "")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "git@github.com:argoproj/not-existing.git", repository.Repo)
}

func TestDb_ListRepositories(t *testing.T) {
	clientset := getClientset(map[string]string{"repositories": repoArgoProj}, newManagedSecret(), repoArgoCD)
	settingsManager := settings.NewSettingsManager(context.TODO(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	repositories, err := testee.ListRepositories(context.TODO())
	require.NoError(t, err)
	assert.Len(t, repositories, 2)
}

func TestDb_UpdateRepository(t *testing.T) {
	secretRepository := &appsv1.Repository{
		Name:     "SomeRepo",
		Repo:     "git@github.com:argoproj/argo-cd.git",
		Username: "someUsername",
		Password: "somePassword",
		Type:     "git",
	}
	settingRepository := &appsv1.Repository{
		Name:     "OtherRepo",
		Repo:     "git@github.com:argoproj/argoproj.git",
		Username: "otherUsername",
		Password: "otherPassword",
		Type:     "git",
	}

	clientset := getClientset(map[string]string{"repositories": repoArgoProj}, newManagedSecret(), repoArgoCD)
	settingsManager := settings.NewSettingsManager(context.TODO(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	// Verify that legacy repository can still be updated
	settingRepository.Username = "OtherUpdatedUsername"
	repository, err := testee.UpdateRepository(context.TODO(), settingRepository)
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Same(t, settingRepository, repository)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(
		context.TODO(),
		"managed-secret",
		metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "OtherUpdatedUsername", string(secret.Data["username"]))

	// Verify that secret-based repository can be updated
	secretRepository.Username = "UpdatedUsername"
	repository, err = testee.UpdateRepository(context.TODO(), secretRepository)
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Same(t, secretRepository, repository)

	secret, err = clientset.CoreV1().Secrets(testNamespace).Get(
		context.TODO(),
		"some-repo-secret",
		metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "UpdatedUsername", string(secret.Data["username"]))
}

func TestDb_DeleteRepository(t *testing.T) {
	clientset := getClientset(map[string]string{"repositories": repoArgoProj}, newManagedSecret(), repoArgoCD)
	settingsManager := settings.NewSettingsManager(context.TODO(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	err := testee.DeleteRepository(context.TODO(), "git@github.com:argoproj/argoproj.git", "")
	require.NoError(t, err)

	repositories, err := settingsManager.GetRepositories()
	require.NoError(t, err)
	assert.Empty(t, repositories)

	err = testee.DeleteRepository(context.TODO(), "git@github.com:argoproj/argo-cd.git", "")
	require.NoError(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(context.TODO(), "some-repo-secret", metav1.GetOptions{})
	require.Error(t, err)
}

func TestDb_GetRepositoryCredentials(t *testing.T) {
	repositoryCredentialsSettings := `
- type: git
  url: git@github.com:argoproj
  usernameSecret:
    name: managed-secret
    key: username
  passwordSecret:
    name: managed-secret
    key: password
`
	repoCredsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "some-repocreds-secret",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds,
			},
		},
		Data: map[string][]byte{
			"type":     []byte("git"),
			"url":      []byte("git@gitlab.com"),
			"username": []byte("someUsername"),
			"password": []byte("somePassword"),
		},
	}

	clientset := getClientset(map[string]string{"repository.credentials": repositoryCredentialsSettings}, newManagedSecret(), repoCredsSecret)
	testee := NewDB(testNamespace, settings.NewSettingsManager(context.TODO(), clientset, testNamespace), clientset)

	repoCreds, err := testee.GetRepositoryCredentials(context.TODO(), "git@github.com:argoproj/argoproj.git")
	require.NoError(t, err)
	assert.NotNil(t, repoCreds)
	assert.Equal(t, "git@github.com:argoproj", repoCreds.URL)

	repoCreds, err = testee.GetRepositoryCredentials(context.TODO(), "git@gitlab.com:someorg/foobar.git")
	require.NoError(t, err)
	assert.NotNil(t, repoCreds)
	assert.Equal(t, "git@gitlab.com", repoCreds.URL)

	repoCreds, err = testee.GetRepositoryCredentials(context.TODO(), "git@github.com:example/not-existing.git")
	require.NoError(t, err)
	assert.Nil(t, repoCreds)
}

func TestRepoURLToSecretName(t *testing.T) {
	tables := []struct {
		repoUrl    string
		secretName string
		project    string
	}{{
		repoUrl:    "git://git@github.com:argoproj/ARGO-cd.git",
		secretName: "repo-83273445",
		project:    "",
	}, {
		repoUrl:    "git://git@github.com:argoproj/ARGO-cd.git",
		secretName: "repo-2733415816",
		project:    "foobar",
	}, {
		repoUrl:    "https://github.com/argoproj/ARGO-cd",
		secretName: "repo-1890113693",
		project:    "",
	}, {
		repoUrl:    "https://github.com/argoproj/ARGO-cd",
		secretName: "repo-4161185408",
		project:    "foobar",
	}, {
		repoUrl:    "https://github.com/argoproj/argo-cd",
		secretName: "repo-42374749",
		project:    "",
	}, {
		repoUrl:    "https://github.com/argoproj/argo-cd",
		secretName: "repo-1894545728",
		project:    "foobar",
	}, {
		repoUrl:    "https://github.com/argoproj/argo-cd.git",
		secretName: "repo-821842295",
		project:    "",
	}, {
		repoUrl:    "https://github.com/argoproj/argo-cd.git",
		secretName: "repo-1474166686",
		project:    "foobar",
	}, {
		repoUrl:    "https://github.com/argoproj/argo_cd.git",
		secretName: "repo-1049844989",
		project:    "",
	}, {
		repoUrl:    "https://github.com/argoproj/argo_cd.git",
		secretName: "repo-3916272608",
		project:    "foobar",
	}, {
		repoUrl:    "ssh://git@github.com/argoproj/argo-cd.git",
		secretName: "repo-3569564120",
		project:    "",
	}, {
		repoUrl:    "ssh://git@github.com/argoproj/argo-cd.git",
		secretName: "repo-754834421",
		project:    "foobar",
	}}

	for _, v := range tables {
		if sn := RepoURLToSecretName(repoSecretPrefix, v.repoUrl, v.project); sn != v.secretName {
			t.Errorf("Expected secret name %q for repo %q; instead, got %q", v.secretName, v.repoUrl, sn)
		}
	}
}

func Test_CredsURLToSecretName(t *testing.T) {
	tables := map[string]string{
		"git://git@github.com:argoproj":  "creds-2483499391",
		"git://git@github.com:argoproj/": "creds-1465032944",
		"git@github.com:argoproj":        "creds-2666065091",
		"git@github.com:argoproj/":       "creds-346879876",
	}

	for k, v := range tables {
		if sn := RepoURLToSecretName(credSecretPrefix, k, ""); sn != v {
			t.Errorf("Expected secret name %q for repo %q; instead, got %q", v, k, sn)
		}
	}
}

func Test_GetProjectRepositories(t *testing.T) {
	repoSecretWithProject := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "some-repo-secret",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
			},
		},
		Data: map[string][]byte{
			"type":    []byte("git"),
			"url":     []byte("git@github.com:argoproj/argo-cd"),
			"project": []byte("some-project"),
		},
	}

	repoSecretWithoutProject := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "some-other-repo-secret",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
			},
		},
		Data: map[string][]byte{
			"type": []byte("git"),
			"url":  []byte("git@github.com:argoproj/argo-cd"),
		},
	}

	clientset := getClientset(map[string]string{}, repoSecretWithProject, repoSecretWithoutProject)
	argoDB := NewDB(testNamespace, settings.NewSettingsManager(context.TODO(), clientset, testNamespace), clientset)

	repos, err := argoDB.GetProjectRepositories(context.TODO(), "some-project")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "git@github.com:argoproj/argo-cd", repos[0].Repo)
}
