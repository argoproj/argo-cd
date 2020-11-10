package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var textToRedact = `
connectors:
- config:
    clientID: aabbccddeeff00112233
    clientSecret: |
      theSecret
    orgs:
    - name: your-github-org
    redirectURI: https://argocd.example.com/api/dex/callback
  id: github
  name: GitHub
  type: github
- config:
    bindDN: uid=serviceaccount,cn=users,dc=example,dc=com
    bindPW: theSecret
    host: ldap.example.com:636
  id: ldap
  name: LDAP
  type: ldap
grpc:
  addr: 0.0.0.0:5557
telemetry:
  http: 0.0.0.0:5558
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

var expectedRedaction = `connectors:
- config:
    clientID: aabbccddeeff00112233
    clientSecret: '********'
    orgs:
    - name: your-github-org
    redirectURI: https://argocd.example.com/api/dex/callback
  id: github
  name: GitHub
  type: github
- config:
    bindDN: uid=serviceaccount,cn=users,dc=example,dc=com
    bindPW: '********'
    host: ldap.example.com:636
  id: ldap
  name: LDAP
  type: ldap
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
  secret: '********'
- id: argo-cd-cli
  name: Argo CD CLI
  public: true
  redirectURIs:
  - http://localhost
storage:
  type: memory
telemetry:
  http: 0.0.0.0:5558
web:
  http: 0.0.0.0:5556
`

func TestSecretsRedactor(t *testing.T) {
	assert.Equal(t, expectedRedaction, redactor(textToRedact))
}
