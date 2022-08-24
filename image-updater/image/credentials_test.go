package image

import (
	"os"
	"path"
	"testing"

	"github.com/argoproj/argo-cd/v2/image-updater/kube"

	"github.com/argoproj-labs/argocd-image-updater/test/fake"
	"github.com/argoproj-labs/argocd-image-updater/test/fixture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_ParseCredentialAnnotation(t *testing.T) {
	t.Run("Parse valid credentials definition of type secret", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=secret:mynamespace/mysecret#anyfield", true)
		assert.NoError(t, err)
		assert.Equal(t, "gcr.io", src.Registry)
		assert.Equal(t, "mynamespace", src.SecretNamespace)
		assert.Equal(t, "mysecret", src.SecretName)
		assert.Equal(t, "anyfield", src.SecretField)
	})

	t.Run("Parse valid credentials definition of type pullsecret", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=pullsecret:mynamespace/mysecret", true)
		assert.NoError(t, err)
		assert.Equal(t, "gcr.io", src.Registry)
		assert.Equal(t, "mynamespace", src.SecretNamespace)
		assert.Equal(t, "mysecret", src.SecretName)
		assert.Equal(t, ".dockerconfigjson", src.SecretField)
	})

	t.Run("Parse invalid secret definition - missing registry", func(t *testing.T) {
		src, err := ParseCredentialSource("secret:mynamespace/mysecret#anyfield", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid secret definition - empty registry", func(t *testing.T) {
		src, err := ParseCredentialSource("=secret:mynamespace/mysecret#anyfield", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid secret definition - unknown credential type", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=secrets:mynamespace/mysecret#anyfield", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid secret definition - missing field", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=secret:mynamespace/mysecret#", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid secret definition - missing namespace", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=secret:/mysecret#anyfield", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid credential definition - missing name", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=secret:mynamespace/#anyfield", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid credential definition - missing most", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=secret:", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid pullsecret definition - missing namespace", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=pullsecret:/mysecret", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse invalid credential definition - missing name", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=pullsecret:mynamespace", true)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

	t.Run("Parse valid credentials definition from environment", func(t *testing.T) {
		src, err := ParseCredentialSource("env:DUMMY_SECRET", false)
		require.NoError(t, err)
		require.NotNil(t, src)
		assert.Equal(t, "DUMMY_SECRET", src.EnvName)
	})

	t.Run("Parse valid credentials definition from environment", func(t *testing.T) {
		src, err := ParseCredentialSource("env:DUMMY_SECRET", false)
		require.NoError(t, err)
		require.NotNil(t, src)
		assert.Equal(t, "DUMMY_SECRET", src.EnvName)
	})

}

func Test_ParseCredentialReference(t *testing.T) {
	t.Run("Parse valid credentials definition of type secret", func(t *testing.T) {
		src, err := ParseCredentialSource("secret:mynamespace/mysecret#anyfield", false)
		assert.NoError(t, err)
		assert.Equal(t, "", src.Registry)
		assert.Equal(t, "mynamespace", src.SecretNamespace)
		assert.Equal(t, "mysecret", src.SecretName)
		assert.Equal(t, "anyfield", src.SecretField)
	})

	t.Run("Parse valid credentials definition of type pullsecret", func(t *testing.T) {
		src, err := ParseCredentialSource("gcr.io=pullsecret:mynamespace/mysecret", false)
		assert.NoError(t, err)
		assert.Equal(t, "gcr.io", src.Registry)
		assert.Equal(t, "mynamespace", src.SecretNamespace)
		assert.Equal(t, "mysecret", src.SecretName)
		assert.Equal(t, ".dockerconfigjson", src.SecretField)
	})

	t.Run("Parse invalid secret definition - empty registry", func(t *testing.T) {
		src, err := ParseCredentialSource("=secret:mynamespace/mysecret#anyfield", false)
		assert.Error(t, err)
		assert.Nil(t, src)
	})

}

func Test_FetchCredentialsFromPullSecret(t *testing.T) {
	t.Run("Fetch credentials from pull secret", func(t *testing.T) {
		dockerJson := fixture.MustReadFile("../../test/testdata/docker/valid-config.json")
		secretData := make(map[string][]byte)
		secretData[pullSecretField] = []byte(dockerJson)
		pullSecret := fixture.NewSecret("test", "test", secretData)
		clientset := fake.NewFakeClientsetWithResources(pullSecret)
		credSrc := &CredentialSource{
			Type:            CredentialSourcePullSecret,
			Registry:        "https://registry-1.docker.io/v2",
			SecretNamespace: "test",
			SecretName:      "test",
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", &kube.KubernetesClient{Clientset: clientset})
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.Equal(t, "foo", creds.Username)
		assert.Equal(t, "bar", creds.Password)
	})

	t.Run("Fetch credentials from pull secret with protocol stripped", func(t *testing.T) {
		dockerJson := fixture.MustReadFile("../../test/testdata/docker/valid-config-noproto.json")
		secretData := make(map[string][]byte)
		secretData[pullSecretField] = []byte(dockerJson)
		pullSecret := fixture.NewSecret("test", "test", secretData)
		clientset := fake.NewFakeClientsetWithResources(pullSecret)
		credSrc := &CredentialSource{
			Type:            CredentialSourcePullSecret,
			Registry:        "https://registry-1.docker.io/v2",
			SecretNamespace: "test",
			SecretName:      "test",
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", &kube.KubernetesClient{Clientset: clientset})
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.Equal(t, "foo", creds.Username)
		assert.Equal(t, "bar", creds.Password)
	})
}

func Test_FetchCredentialsFromEnv(t *testing.T) {
	t.Run("Fetch credentials from environment", func(t *testing.T) {
		err := os.Setenv("MY_SECRET_ENV", "foo:bar")
		require.NoError(t, err)
		credSrc := &CredentialSource{
			Type:     CredentialSourceEnv,
			Registry: "https://registry-1.docker.io/v2",
			EnvName:  "MY_SECRET_ENV",
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", nil)
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.Equal(t, "foo", creds.Username)
		assert.Equal(t, "bar", creds.Password)
	})

	t.Run("Fetch credentials from environment with missing env var", func(t *testing.T) {
		err := os.Setenv("MY_SECRET_ENV", "")
		require.NoError(t, err)
		credSrc := &CredentialSource{
			Type:     CredentialSourceEnv,
			Registry: "https://registry-1.docker.io/v2",
			EnvName:  "MY_SECRET_ENV",
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", nil)
		require.Error(t, err)
		require.Nil(t, creds)
	})

	t.Run("Fetch credentials from environment with invalid value in env var", func(t *testing.T) {
		for _, value := range []string{"babayaga", "foo:", "bar:", ":"} {
			err := os.Setenv("MY_SECRET_ENV", value)
			require.NoError(t, err)
			credSrc := &CredentialSource{
				Type:     CredentialSourceEnv,
				Registry: "https://registry-1.docker.io/v2",
				EnvName:  "MY_SECRET_ENV",
			}
			creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", nil)
			require.Error(t, err)
			require.Nil(t, creds)
		}
	})
}

func Test_FetchCredentialsFromExt(t *testing.T) {
	t.Run("Fetch credentials from external script - valid output", func(t *testing.T) {
		pwd, err := os.Getwd()
		require.NoError(t, err)
		credSrc := &CredentialSource{
			Type:       CredentialSourceExt,
			Registry:   "https://registry-1.docker.io/v2",
			ScriptPath: path.Join(pwd, "..", "..", "test", "testdata", "scripts", "get-credentials-valid.sh"),
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", nil)
		require.NoError(t, err)
		require.NotNil(t, creds)
		assert.Equal(t, "username", creds.Username)
		assert.Equal(t, "password", creds.Password)
	})
	t.Run("Fetch credentials from external script - invalid script output", func(t *testing.T) {
		pwd, err := os.Getwd()
		require.NoError(t, err)
		credSrc := &CredentialSource{
			Type:       CredentialSourceExt,
			Registry:   "https://registry-1.docker.io/v2",
			ScriptPath: path.Join(pwd, "..", "..", "test", "testdata", "scripts", "get-credentials-invalid.sh"),
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", nil)
		require.Errorf(t, err, "invalid script output")
		require.Nil(t, creds)
	})
	t.Run("Fetch credentials from external script - script does not exist", func(t *testing.T) {
		pwd, err := os.Getwd()
		require.NoError(t, err)
		credSrc := &CredentialSource{
			Type:       CredentialSourceExt,
			Registry:   "https://registry-1.docker.io/v2",
			ScriptPath: path.Join(pwd, "..", "..", "test", "testdata", "scripts", "get-credentials-notexist.sh"),
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", nil)
		require.Errorf(t, err, "no such file or directory")
		require.Nil(t, creds)
	})
	t.Run("Fetch credentials from external script - relative path", func(t *testing.T) {
		credSrc := &CredentialSource{
			Type:       CredentialSourceExt,
			Registry:   "https://registry-1.docker.io/v2",
			ScriptPath: "get-credentials-notexist.sh",
		}
		creds, err := credSrc.FetchCredentials("https://registry-1.docker.io", nil)
		require.Errorf(t, err, "path to script must be absolute")
		require.Nil(t, creds)
	})
}

func Test_ParseDockerConfig(t *testing.T) {
	t.Run("Parse valid Docker configuration with matching registry", func(t *testing.T) {
		config := fixture.MustReadFile("../../test/testdata/docker/valid-config.json")
		username, password, err := parseDockerConfigJson("https://registry-1.docker.io", config)
		require.NoError(t, err)
		assert.Equal(t, "foo", username)
		assert.Equal(t, "bar", password)
	})

	t.Run("Parse valid Docker configuration with matching registry as prefix", func(t *testing.T) {
		config := fixture.MustReadFile("../../test/testdata/docker/valid-config-noproto.json")
		username, password, err := parseDockerConfigJson("https://registry-1.docker.io", config)
		require.NoError(t, err)
		assert.Equal(t, "foo", username)
		assert.Equal(t, "bar", password)
	})

	t.Run("Parse valid Docker configuration with matching registry as prefix with / in the end", func(t *testing.T) {
		config := fixture.MustReadFile("../../test/testdata/docker/valid-config-noproto.json")
		username, password, err := parseDockerConfigJson("https://registry-1.docker.io/", config)
		require.NoError(t, err)
		assert.Equal(t, "foo", username)
		assert.Equal(t, "bar", password)
	})

	t.Run("Parse valid Docker configuration without matching registry", func(t *testing.T) {
		config := fixture.MustReadFile("../../test/testdata/docker/valid-config.json")
		username, password, err := parseDockerConfigJson("https://gcr.io", config)
		assert.Error(t, err)
		assert.Empty(t, username)
		assert.Empty(t, password)
	})
}
