package commands

import (
	"os"
	"testing"

	argocdclient "github.com/argoproj/argo-cd/v2/pkg/apiclient"
	"github.com/argoproj/argo-cd/v2/util/localconfig"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//

func Test_userDisplayName_email(t *testing.T) {
	claims := jwt.MapClaims{"iss": "qux", "sub": "foo", "email": "firstname.lastname@example.com", "groups": []string{"baz"}}
	actualName := userDisplayName(claims)
	expectedName := "firstname.lastname@example.com"
	assert.Equal(t, expectedName, actualName)
}

func Test_userDisplayName_name(t *testing.T) {
	claims := jwt.MapClaims{"iss": "qux", "sub": "foo", "name": "Firstname Lastname", "groups": []string{"baz"}}
	actualName := userDisplayName(claims)
	expectedName := "Firstname Lastname"
	assert.Equal(t, expectedName, actualName)
}

func Test_userDisplayName_sub(t *testing.T) {
	claims := jwt.MapClaims{"iss": "qux", "sub": "foo", "groups": []string{"baz"}}
	actualName := userDisplayName(claims)
	expectedName := "foo"
	assert.Equal(t, expectedName, actualName)
}

func Test_CoreLogin(t *testing.T) {
	var customKubeConfig = "/etc/rancher/k3s/k3s.yaml"
	// Write the test config file
	err := os.WriteFile(testConfigFilePath, []byte(testCoreConfig), os.ModePerm)
	assert.NoError(t, err)
	defer os.Remove(testConfigFilePath)

	err = os.Chmod(testConfigFilePath, 0600)
	require.NoError(t, err)

	command := NewLoginCommand(&argocdclient.ClientOptions{ConfigPath: testConfigFilePath, Core: true})
	_ = command.Flags().Set("kubeconfig", customKubeConfig)
	command.Run(command, []string{"kubernetes"})

	localConfig, err := localconfig.ReadLocalConfig(testConfigFilePath)
	assert.NoError(t, err)
	assert.Equal(t, localConfig.DefaultKubeconfig, customKubeConfig)
	assert.Equal(t, localConfig.CurrentContext, "kubernetes")

}
