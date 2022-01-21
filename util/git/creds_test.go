package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const gcpServiceAccountKeyJSON = `{
  "type": "service_account",
  "project_id": "my-google-project",
  "private_key_id": "REDACTED",
  "private_key": "-----BEGIN PRIVATE KEY-----\nREDACTED\n-----END PRIVATE KEY-----\n",
  "client_email": "argocd-service-account@my-google-project.iam.gserviceaccount.com",
  "client_id": "REDACTED",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/argocd-service-account%40my-google-project.iam.gserviceaccount.com"
}`

const invalidJSON = `{
  "type": "service_account",
  "project_id": "my-google-project",
`

func TestNewGoogleCloudCreds(t *testing.T) {
	googleCloudCreds := NewGoogleCloudCreds(gcpServiceAccountKeyJSON)
	assert.NotNil(t, googleCloudCreds)
}

func TestNewGoogleCloudCreds_invalidJSON(t *testing.T) {
	googleCloudCreds := NewGoogleCloudCreds(invalidJSON)
	assert.Nil(t, googleCloudCreds.creds)

	token, err := googleCloudCreds.getAccessToken()
	assert.Equal(t, "", token)
	assert.NotNil(t, err)

	username, err := googleCloudCreds.getUsername()
	assert.Equal(t, "", username)
	assert.NotNil(t, err)

	closer, envStringSlice, err := googleCloudCreds.Environ()
	assert.Equal(t, NopCloser{}, closer)
	assert.Equal(t, []string(nil), envStringSlice)
	assert.NotNil(t, err)
}

func TestGoogleCloudCreds_Environ(t *testing.T) {
	staticToken := &oauth2.Token{AccessToken: "token"}
	googleCloudCreds := GoogleCloudCreds{&google.Credentials{
		ProjectID:   "my-google-project",
		TokenSource: oauth2.StaticTokenSource(staticToken),
		JSON:        []byte(gcpServiceAccountKeyJSON),
	}}

	closer, env, err := googleCloudCreds.Environ()
	assert.NoError(t, err)
	defer func() { _ = closer.Close() }()

	assert.Equal(t, []string{"GIT_ASKPASS=git-ask-pass.sh", "GIT_USERNAME=argocd-service-account@my-google-project.iam.gserviceaccount.com", "GIT_PASSWORD=token"}, env)
}
