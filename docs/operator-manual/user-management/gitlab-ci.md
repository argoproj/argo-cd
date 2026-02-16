# GitLab CI

GitLab is an OAuth identity provider which can be used in GitLab CI
to generate tokens that identifies the repository and where it runs.

See: <https://docs.gitlab.com/ci/secrets/id_token_authentication>

You need to use OAuth 2.0 Token Exchange. Some identity providers supports this
out of the box such as Dex.

## Using Dex

Edit the `argocd-cm` and configure the `dex.config` section:

```yaml
dex.config: |
  connectors:
    - type: oidc
      id: github-ci
      name: GitLab CI
      config:
        issuer: https://gitlab.com
        # If using GitLab self-hosted, then use your GitLab issuer
        scopes: [openid]
        userNameKey: sub
        insecureSkipEmailVerified: true
```

ArgoCD automatically generates a static client named `argo-cd-cli` that you can use to get your token from a GitLab CI.

Here is an example of GitLab CI that will retrieve a valid Argo CD authentication token from Dex and use it to perform operations with the CLI:

```yaml
deploy:
  id_tokens:
    GITLAB_OIDC_TOKEN:
      aud: https://argocd.example.com # Your ArgoCD URL
  
  script:
    - apt-get update && apt-get install -y jq curl
    - curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
    - install -m 555 argocd-linux-amd64 /usr/local/bin/argocd
    - rm argocd-linux-amd64
    - |       
      # Exchange GitLab token for Dex token
      DEX_URL="https://argocd.example.com/api/dex/token"
      DEX_TOKEN_RESPONSE=$(curl -sSf \
        "$DEX_URL" \
        --user argo-cd-cli: \
        --data-urlencode "connector_id=gitlab-ci" \
        --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
        --data-urlencode "scope=openid email profile federated:id" \
        --data-urlencode "requested_token_type=urn:ietf:params:oauth:token-type:access_token" \
        --data-urlencode "subject_token=$GITLAB_OIDC_TOKEN" \
        --data-urlencode "subject_token_type=urn:ietf:params:oauth:token-type:id_token")
      
      DEX_TOKEN=$(echo "$DEX_TOKEN_RESPONSE" | jq -r .access_token)
      
      # Use with ArgoCD CLI
      export ARGOCD_SERVER="argocd.example.com" 
      export ARGOCD_OPTS="--grpc-web"
      export ARGOCD_AUTH_TOKEN="$DEX_TOKEN"
      argocd version
      argocd account get-user-info
      argocd app list
```


## Configuring RBAC

When using ArgoCD global RBAC comfig map, you can define your `policy.csv` like so:

```yaml
configs:
  rbac:
    policy.csv: |
      # Specific project(infra) for specific apps
      p, project_path:my-repo/my-project:*, applications, get, infra/*, allow
      # Only main branch can sync under production project
      p, project_path:my-repo/my-project:ref_type:branch:ref:main, applications, sync, production/*, allow
```

More info: [RBAC Configuration](../rbac.md)
