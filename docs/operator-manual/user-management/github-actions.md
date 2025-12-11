# GitHub Actions

GitHub is an OAuth identity provider which can be used in GitHub Actions
to generate tokens that identifies the repository and where it runs.

See: <https://docs.github.com/en/actions/security-for-github-actions/security-hardening-your-deployments/about-security-hardening-with-openid-connect>

You need to use OAuth 2.0 Token Exchange. Some identity providers supports this
out of the box such as Dex.

## Using Dex

Edit the `argocd-cm` and configure the `dex.config` section:

```yaml
dex.config: |
  connectors:
    - type: oidc
      id: github-actions
      name: GitHub Actions
      config:
        issuer: https://token.actions.githubusercontent.com/
        # If using GitHub Enterprise Server, then use this issuer:
        #issuer: https://github.example.com/_services/token
        scopes: [openid]
        userNameKey: sub
        insecureSkipEmailVerified: true
```

ArgoCD automatically generates a static client named `argo-cd-cli` that you can use to get your token from a GitHub Action.

Here is an example of GitHub Action that will retrieve a valid Argo CD authentication token from Dex and use it to perform action with the CLI:

```yaml
name: argocd-test

on:
  pull_request:

permissions:
  id-token: write # This is required for requesting the JWT

jobs:
  argocd-test:
    runs-on:
      group: ephemeral_runners
    steps:
      # Actions have access to two special environment variables ACTIONS_CACHE_URL and ACTIONS_RUNTIME_TOKEN.
      # Inline step scripts in workflows do not see these variables.
      - uses: actions/github-script@v6
        id: script
        timeout-minutes: 10
        with:
          debug: true
          script: |
            const token = process.env['ACTIONS_RUNTIME_TOKEN']
            const runtimeUrl = process.env['ACTIONS_ID_TOKEN_REQUEST_URL']
            core.setOutput('TOKEN', token.trim())
            core.setOutput('IDTOKENURL', runtimeUrl.trim())

      - name: Obtain access token
        id: idtoken
        run: |
          # get an token from github
          echo "getting token from GitHub"
          GH_TOKEN_RESPONSE=$(curl -sSf \
            "${{steps.script.outputs.IDTOKENURL}}" \
            -H "Authorization: bearer  ${{steps.script.outputs.TOKEN}}" \
            -H "Accept: application/json; api-version=2.0" \
            -H "Content-Type: application/json" \
            -d "{}" \
          )
          GH_TOKEN=$(jq -r .value <<< $GH_TOKEN_RESPONSE)
          echo "::add-mask::$GH_TOKEN"

          # exchange it for a dex token
          DEX_URL="https://argocd.example.com/api/dex/token"
          echo "getting access token from Dex: $DEX_URL"
          DEX_TOKEN_RESPONSE=$(curl -sSf \
              "$DEX_URL" \
              --user argo-cd-cli: \
              --data-urlencode "connector_id=github-actions" \
              --data-urlencode "grant_type=urn:ietf:params:oauth:grant-type:token-exchange" \
              --data-urlencode "scope=openid email profile federated:id" \
              --data-urlencode "requested_token_type=urn:ietf:params:oauth:token-type:access_token" \
              --data-urlencode "subject_token=$GH_TOKEN" \
              --data-urlencode "subject_token_type=urn:ietf:params:oauth:token-type:id_token")
          DEX_TOKEN=$(jq -r .access_token <<< $DEX_TOKEN_RESPONSE)

          if [[ -z "$DEX_TOKEN" ]]; then
            echo "::error::No token found in dex response"
            exit 1
          fi

          echo "::add-mask::$(echo "$DEX_TOKEN" | base64 -w0)"
          echo "::add-mask::$DEX_TOKEN"
          echo "dex-token=$DEX_TOKEN" >> "$GITHUB_OUTPUT"
          # use $DEX_TOKEN

      - name: Setup ArgoCD CLI
        run: |
          curl -sSL -o argocd-linux-amd64 https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
          mkdir -p "$RUNNER_TEMP/argocd"
          install -m 555 argocd-linux-amd64 "$RUNNER_TEMP/argocd/argocd"
          rm argocd-linux-amd64
          echo "$RUNNER_TEMP/argocd" >> "$GITHUB_PATH"

      - name: Use CLI in some commands
        env:
          ARGOCD_AUTH_TOKEN: ${{ steps.idtoken.outputs.dex-token }}
          ARGOCD_SERVER: argocd.example.com
          ARGOCD_OPTS: --grpc-web
        run: |
          set -x
          argocd version
          argocd account get-user-info
          argocd proj list
          argocd app list
```


## Configuring RBAC

When using ArgoCD v3.0.0 or later, then you define your `policy.csv` like so:

```yaml
configs:
  rbac:
    policy.csv: |
      p, repo:my-org/my-repo:pull_request, projects, get, my-project, allow
      p, repo:my-org/my-repo:pull_request, applications, get, my-project/*, allow
      p, repo:my-org/my-repo:pull_request, applicationsets, get, my-project/*, allow
```

More info: [RBAC Configuration](../rbac.md)

> [!NOTE]
> Defining policies are not supported on ArgoCD v2.
> To define policies, please [upgrade](../upgrading/overview.md)
> to to v3.0.0 or later.
