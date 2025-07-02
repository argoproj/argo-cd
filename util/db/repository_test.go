package db

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/settings"
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

var repoArgoProj = &corev1.Secret{
	ObjectMeta: metav1.ObjectMeta{
		Namespace: testNamespace,
		Name:      "some-other-repo-secret",
		Annotations: map[string]string{
			common.AnnotationKeyManagedBy: common.AnnotationValueManagedByArgoCD,
		},
		Labels: map[string]string{
			common.LabelKeySecretType: common.LabelValueSecretTypeRepository,
		},
	},
	Data: map[string][]byte{
		"name":     []byte("OtherRepo"),
		"url":      []byte("git@github.com:argoproj/argoproj.git"),
		"username": []byte("someUsername"),
		"password": []byte("somePassword"),
		"type":     []byte("git"),
	},
}

func TestDb_CreateRepository(t *testing.T) {
	clientset := getClientset()
	settingsManager := settings.NewSettingsManager(t.Context(), clientset, testNamespace)
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
	output, err := testee.CreateRepository(t.Context(), input)
	require.NoError(t, err)
	assert.Same(t, input, output)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(
		t.Context(),
		RepoURLToSecretName(repoSecretPrefix, input.Repo, ""),
		metav1.GetOptions{},
	)
	assert.NotNil(t, secret)
	require.NoError(t, err)
}

func TestDb_GetRepository(t *testing.T) {
	clientset := getClientset(repoArgoCD, repoArgoProj)
	settingsManager := settings.NewSettingsManager(t.Context(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	repository, err := testee.GetRepository(t.Context(), "git@github.com:argoproj/argoproj.git", "")
	require.NoError(t, err)
	require.NotNil(t, repository)
	assert.Equal(t, "OtherRepo", repository.Name)

	repository, err = testee.GetRepository(t.Context(), "git@github.com:argoproj/argo-cd.git", "")
	require.NoError(t, err)
	require.NotNil(t, repository)
	assert.Equal(t, "SomeRepo", repository.Name)

	repository, err = testee.GetRepository(t.Context(), "git@github.com:argoproj/not-existing.git", "")
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Equal(t, "git@github.com:argoproj/not-existing.git", repository.Repo)
}

func TestDb_ListRepositories(t *testing.T) {
	clientset := getClientset(repoArgoCD, repoArgoProj)
	settingsManager := settings.NewSettingsManager(t.Context(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	repositories, err := testee.ListRepositories(t.Context())
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

	clientset := getClientset(repoArgoCD)
	settingsManager := settings.NewSettingsManager(t.Context(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	secretRepository.Username = "UpdatedUsername"
	repository, err := testee.UpdateRepository(t.Context(), secretRepository)
	require.NoError(t, err)
	assert.NotNil(t, repository)
	assert.Same(t, secretRepository, repository)

	secret, err := clientset.CoreV1().Secrets(testNamespace).Get(
		t.Context(),
		"some-repo-secret",
		metav1.GetOptions{},
	)
	require.NoError(t, err)
	assert.NotNil(t, secret)
	assert.Equal(t, "UpdatedUsername", string(secret.Data["username"]))
}

func TestDb_DeleteRepository(t *testing.T) {
	clientset := getClientset(repoArgoCD, repoArgoProj)
	settingsManager := settings.NewSettingsManager(t.Context(), clientset, testNamespace)
	testee := &db{
		ns:            testNamespace,
		kubeclientset: clientset,
		settingsMgr:   settingsManager,
	}

	err := testee.DeleteRepository(t.Context(), "git@github.com:argoproj/argoproj.git", "")
	require.NoError(t, err)

	err = testee.DeleteRepository(t.Context(), "git@github.com:argoproj/argo-cd.git", "")
	require.NoError(t, err)

	_, err = clientset.CoreV1().Secrets(testNamespace).Get(t.Context(), "some-repo-secret", metav1.GetOptions{})
	require.Error(t, err)
}

func TestDb_GetRepositoryCredentials(t *testing.T) {
	gitHubRepoCredsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "some-repocreds-secret",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeRepoCreds,
			},
		},
		Data: map[string][]byte{
			"type":     []byte("git"),
			"url":      []byte("git@github.com:argoproj"),
			"username": []byte("someUsername"),
			"password": []byte("somePassword"),
		},
	}
	gitLabRepoCredsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: testNamespace,
			Name:      "some-other-repocreds-secret",
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

	clientset := getClientset(gitHubRepoCredsSecret, gitLabRepoCredsSecret)
	testee := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	repoCreds, err := testee.GetRepositoryCredentials(t.Context(), "git@github.com:argoproj/argoproj.git")
	require.NoError(t, err)
	require.NotNil(t, repoCreds)
	assert.Equal(t, "git@github.com:argoproj", repoCreds.URL)

	repoCreds, err = testee.GetRepositoryCredentials(t.Context(), "git@gitlab.com:someorg/foobar.git")
	require.NoError(t, err)
	require.NotNil(t, repoCreds)
	assert.Equal(t, "git@gitlab.com", repoCreds.URL)

	repoCreds, err = testee.GetRepositoryCredentials(t.Context(), "git@github.com:example/not-existing.git")
	require.NoError(t, err)
	assert.Nil(t, repoCreds)
}

func TestRepoURLToSecretName(t *testing.T) {
	tables := []struct {
		repoURL    string
		secretName string
		project    string
	}{{
		repoURL:    "git://git@github.com:argoproj/ARGO-cd.git",
		secretName: "repo-83273445",
		project:    "",
	}, {
		repoURL:    "git://git@github.com:argoproj/ARGO-cd.git",
		secretName: "repo-2733415816",
		project:    "foobar",
	}, {
		repoURL:    "https://github.com/argoproj/ARGO-cd",
		secretName: "repo-1890113693",
		project:    "",
	}, {
		repoURL:    "https://github.com/argoproj/ARGO-cd",
		secretName: "repo-4161185408",
		project:    "foobar",
	}, {
		repoURL:    "https://github.com/argoproj/argo-cd",
		secretName: "repo-42374749",
		project:    "",
	}, {
		repoURL:    "https://github.com/argoproj/argo-cd",
		secretName: "repo-1894545728",
		project:    "foobar",
	}, {
		repoURL:    "https://github.com/argoproj/argo-cd.git",
		secretName: "repo-821842295",
		project:    "",
	}, {
		repoURL:    "https://github.com/argoproj/argo-cd.git",
		secretName: "repo-1474166686",
		project:    "foobar",
	}, {
		repoURL:    "https://github.com/argoproj/argo_cd.git",
		secretName: "repo-1049844989",
		project:    "",
	}, {
		repoURL:    "https://github.com/argoproj/argo_cd.git",
		secretName: "repo-3916272608",
		project:    "foobar",
	}, {
		repoURL:    "ssh://git@github.com/argoproj/argo-cd.git",
		secretName: "repo-3569564120",
		project:    "",
	}, {
		repoURL:    "ssh://git@github.com/argoproj/argo-cd.git",
		secretName: "repo-754834421",
		project:    "foobar",
	}}

	for _, v := range tables {
		sn := RepoURLToSecretName(repoSecretPrefix, v.repoURL, v.project)
		assert.Equal(t, sn, v.secretName, "Expected secret name %q for repo %q; instead, got %q", v.secretName, v.repoURL, sn)
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
		sn := RepoURLToSecretName(credSecretPrefix, k, "")
		assert.Equal(t, sn, v, "Expected secret name %q for repo %q; instead, got %q", v, k, sn)
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

	clientset := getClientset(repoSecretWithProject, repoSecretWithoutProject)
	argoDB := NewDB(testNamespace, settings.NewSettingsManager(t.Context(), clientset, testNamespace), clientset)

	repos, err := argoDB.GetProjectRepositories("some-project")
	require.NoError(t, err)
	assert.Len(t, repos, 1)
	assert.Equal(t, "git@github.com:argoproj/argo-cd", repos[0].Repo)
}
