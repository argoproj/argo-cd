package repos

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateConfig(t *testing.T) {
	assert.EqualError(t, Config{}.Validate(), "invalid config, must specify Url")

	const helm = "helm"
	assert.EqualError(t, Config{Url: "://", Type: helm}.Validate(), "invalid config, must specify Name")
	assert.NoError(t, Config{Url: "://", Type: helm, Name: "foo"}.Validate())
	assert.NoError(t, Config{Url: "://", Type: helm, Name: "foo", CAData: []byte{}}.Validate())
	assert.NoError(t, Config{Url: "://", Type: helm, Name: "foo", CertData: []byte{}}.Validate())
	assert.NoError(t, Config{Url: "://", Type: helm, Name: "foo", KeyData: []byte{}}.Validate())
	assert.EqualError(t, Config{Url: "://", Type: helm, Name: "foo", SSHPrivateKey: "foo"}.Validate(), "invalid config, must not specify SSHPrivateKey")
	assert.EqualError(t, Config{Url: "://", Type: helm, Name: "foo", InsecureIgnoreHostKey: true}.Validate(), "invalid config, must not specify InsecureIgnoreHostKey")

	const git = "git"
	assert.EqualError(t, Config{Url: "://", Type: git, Name: "foo"}.Validate(), "invalid config, must not specify Name, CertData, CAData, or KeyData")
	assert.EqualError(t, Config{Url: "://", Type: git, CAData: []byte{}}.Validate(), "invalid config, must not specify Name, CertData, CAData, or KeyData")
	assert.EqualError(t, Config{Url: "://", Type: git, CertData: []byte{}}.Validate(), "invalid config, must not specify Name, CertData, CAData, or KeyData")
	assert.EqualError(t, Config{Url: "://", Type: git, KeyData: []byte{}}.Validate(), "invalid config, must not specify Name, CertData, CAData, or KeyData")
	assert.NoError(t, Config{Url: "://", Type: git, SSHPrivateKey: "foo"}.Validate())
}
