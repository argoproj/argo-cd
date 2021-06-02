package db

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

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

	assert.Equal(t, input.Name, string(secret.Data["name"]))
	assert.Equal(t, input.Repo, string(secret.Data["repo"]))
	assert.Equal(t, input.Username, string(secret.Data["username"]))
	assert.Equal(t, input.Password, string(secret.Data["password"]))
	assert.Equal(t, strconv.FormatBool(input.InsecureIgnoreHostKey), string(secret.Data["insecureIgnoreHostKey"]))
	assert.Equal(t, strconv.FormatBool(input.EnableLFS), string(secret.Data["enableLfs"]))
}
