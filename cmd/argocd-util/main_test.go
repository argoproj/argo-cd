package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var textToRedact = `
- config:
    clientID: aabbccddeeff00112233
    clientSecret: $dex.github.clientSecret
    orgs:
    - name: your-github-org
    redirectURI: https://argocd.example.com/api/dex/callback
  id: github
  name: GitHub
  type: github
grpc:
  addr: 0.0.0.0:5557
issuer: https://argocd.example.com/api/dex
oauth2:
  skipApprovalScreen: true
staticClients:
- id: argo-cd
  name: Argo CD
  redirectURIs:
  - https://argocd.example.com/auth/callback
  secret: Dis9M-GA11oTwZVQQWdDklPQw-sWXZkWJFyyEhMs
- id: argo-cd-cli
  name: Argo CD CLI
  public: true
  redirectURIs:
  - http://localhost
storage:
  type: memory
web:
  http: 0.0.0.0:5556`

var expectedRedaction = `
- config:
    clientID: aabbccddeeff00112233
    clientSecret: ********
    orgs:
    - name: your-github-org
    redirectURI: https://argocd.example.com/api/dex/callback
  id: github
  name: GitHub
  type: github
grpc:
  addr: 0.0.0.0:5557
issuer: https://argocd.example.com/api/dex
oauth2:
  skipApprovalScreen: true
staticClients:
- id: argo-cd
  name: Argo CD
  redirectURIs:
  - https://argocd.example.com/auth/callback
  secret: ********
- id: argo-cd-cli
  name: Argo CD CLI
  public: true
  redirectURIs:
  - http://localhost
storage:
  type: memory
web:
  http: 0.0.0.0:5556`

func TestSecretsRedactor(t *testing.T) {
	assert.Equal(t, expectedRedaction, redactor(textToRedact))
}
